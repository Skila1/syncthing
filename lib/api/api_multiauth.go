// Copyright (C) 2026 The Syncthing Authors.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at https://mozilla.org/MPL/2.0/.

package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/syncthing/syncthing/lib/config"
	"github.com/syncthing/syncthing/lib/events"
	"github.com/syncthing/syncthing/lib/mfa"
	"github.com/syncthing/syncthing/lib/rand"
	"github.com/syncthing/syncthing/lib/users"
)

const (
	mfaPendingTokenLifetime = 5 * time.Minute
	rememberDeviceCookieLen = 64
)

// mfaPendingEntry tracks a user who authenticated with password but still
// needs to provide their TOTP code.
type mfaPendingEntry struct {
	userID    int64
	expiresAt time.Time
}

type multiUserAuthMiddleware struct {
	shortID            string
	cookieName         string
	rememberCookieName string
	guiCfg             config.GUIConfiguration
	userManager        *users.Manager
	evLogger           events.Logger
	next               http.Handler

	mfaPendingMu sync.Mutex
	mfaPending   map[string]*mfaPendingEntry // token -> pending entry
}

func newMultiUserAuthMiddleware(shortID string, guiCfg config.GUIConfiguration, userManager *users.Manager, evLogger events.Logger, next http.Handler) *multiUserAuthMiddleware {
	return &multiUserAuthMiddleware{
		shortID:            shortID,
		cookieName:         "sessionid-" + shortID,
		rememberCookieName: "mfa-remember-" + shortID,
		guiCfg:             guiCfg,
		userManager:        userManager,
		evLogger:           evLogger,
		next:               next,
		mfaPending:         make(map[string]*mfaPendingEntry),
	}
}

func (m *multiUserAuthMiddleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// API key bypasses user auth (for automation)
	if hasValidAPIKeyHeader(r, m.guiCfg) {
		// API key users are treated as admin
		adminUser := &users.User{
			ID:       0,
			Username: "api",
			Role:     users.RoleAdmin,
			Status:   users.StatusActive,
		}
		r = withUser(r, adminUser)
		m.next.ServeHTTP(w, r)
		return
	}

	// Check session cookie
	if user := m.checkSessionCookie(r); user != nil {
		r = withUser(r, user)
		m.next.ServeHTTP(w, r)
		return
	}

	// Fall back to Basic auth
	if user := m.attemptBasicAuth(r); user != nil {
		r = withUser(r, user)
		m.next.ServeHTTP(w, r)
		return
	}

	// No-auth paths (static assets, login page, health check)
	if isNoAuthPath(r.URL.Path, m.guiCfg.MetricsWithoutAuth) {
		m.next.ServeHTTP(w, r)
		return
	}

	if m.guiCfg.SendBasicAuthPrompt {
		unauthorized(w, m.shortID)
		return
	}

	forbidden(w)
}

func (m *multiUserAuthMiddleware) checkSessionCookie(r *http.Request) *users.User {
	for _, cookie := range r.Cookies() {
		if cookie.Name == m.cookieName {
			if user := m.userManager.ValidateSession(cookie.Value); user != nil {
				return user
			}
		}
	}
	return nil
}

func (m *multiUserAuthMiddleware) attemptBasicAuth(r *http.Request) *users.User {
	username, password, ok := r.BasicAuth()
	if !ok {
		return nil
	}

	slog.Debug("Sessionless HTTP request with authentication; this is expensive.")

	user, err := m.userManager.Authenticate(username, password)
	if err != nil {
		emitLoginAttempt(false, username, r, m.evLogger)
		antiBruteForceSleep()
		return nil
	}

	return user
}

func (m *multiUserAuthMiddleware) passwordAuthHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username     string `json:"username"`
		Password     string `json:"password"`
		StayLoggedIn bool   `json:"stayLoggedIn"`
	}
	if err := unmarshalTo(http.MaxBytesReader(w, r.Body, maxLoginRequestSize), &req); err != nil {
		l.Debugln("Failed to parse username and password:", err)
		http.Error(w, "Failed to parse username and password.", http.StatusBadRequest)
		return
	}

	user, err := m.userManager.Authenticate(req.Username, req.Password)
	if err != nil {
		emitLoginAttempt(false, req.Username, r, m.evLogger)
		antiBruteForceSleep()
		forbidden(w)
		return
	}

	// If MFA is enabled and the device is not remembered, require TOTP
	if user.MFAEnabled && !m.isDeviceRemembered(r, user.ID) {
		pendingToken := rand.String(64)
		m.mfaPendingMu.Lock()
		m.mfaPending[pendingToken] = &mfaPendingEntry{
			userID:    user.ID,
			expiresAt: time.Now().Add(mfaPendingTokenLifetime),
		}
		m.mfaPendingMu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"mfaRequired":  true,
			"mfaPending":   pendingToken,
			"stayLoggedIn": req.StayLoggedIn,
		})
		return
	}

	m.completeLogin(w, r, user, req.StayLoggedIn, false)
}

func (m *multiUserAuthMiddleware) mfaVerifyHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		MFAPending     string `json:"mfaPending"`
		Code           string `json:"code"`
		StayLoggedIn   bool   `json:"stayLoggedIn"`
		RememberDevice bool   `json:"rememberDevice"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxLoginRequestSize)).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	m.mfaPendingMu.Lock()
	pending, ok := m.mfaPending[req.MFAPending]
	if ok {
		delete(m.mfaPending, req.MFAPending)
	}
	m.mfaPendingMu.Unlock()

	if !ok || pending.expiresAt.Before(time.Now()) {
		antiBruteForceSleep()
		http.Error(w, "MFA session expired, please login again", http.StatusUnauthorized)
		return
	}

	user, err := m.userManager.GetUser(pending.userID)
	if err != nil {
		http.Error(w, "User not found", http.StatusInternalServerError)
		return
	}

	// Try TOTP code first, then recovery code
	valid, err := mfa.ValidateCode(user.MFASecret, req.Code)
	if err != nil {
		slog.Error("MFA validation error", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if !valid {
		// Try as recovery code
		valid, err = m.userManager.ValidateRecoveryCode(user.ID, req.Code)
		if err != nil {
			slog.Error("Recovery code validation error", "error", err)
		}
	}

	if !valid {
		antiBruteForceSleep()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "Invalid MFA code",
		})
		return
	}

	m.completeLogin(w, r, user, req.StayLoggedIn, req.RememberDevice)
}

func (m *multiUserAuthMiddleware) completeLogin(w http.ResponseWriter, r *http.Request, user *users.User, stayLoggedIn, rememberDevice bool) {
	remoteAddr, _ := remoteAddress(r)
	token, err := m.userManager.CreateSession(user.ID, remoteAddr)
	if err != nil {
		slog.Error("Failed to create session", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	connectionIsHTTPS := r.TLS != nil || isForwardedHTTPS(r)
	useSecureCookie := connectionIsHTTPS || m.guiCfg.UseTLS()

	maxAge := 0
	if stayLoggedIn {
		maxAge = int(users.SessionLifetime.Seconds())
	}
	http.SetCookie(w, &http.Cookie{
		Name:   m.cookieName,
		Value:  token,
		MaxAge: maxAge,
		Secure: useSecureCookie,
		Path:   "/",
	})

	if rememberDevice && user.MFAEnabled {
		deviceToken := m.generateRememberToken(user.ID)
		http.SetCookie(w, &http.Cookie{
			Name:   m.rememberCookieName,
			Value:  deviceToken,
			MaxAge: int(users.RememberDeviceLifetime.Seconds()),
			Secure: useSecureCookie,
			Path:   "/",
		})
	}

	emitLoginAttempt(true, user.Username, r, m.evLogger)
	w.WriteHeader(http.StatusNoContent)
}

// isDeviceRemembered checks the "remember device" cookie.
func (m *multiUserAuthMiddleware) isDeviceRemembered(r *http.Request, userID int64) bool {
	for _, cookie := range r.Cookies() {
		if cookie.Name == m.rememberCookieName {
			return m.validateRememberToken(cookie.Value, userID)
		}
	}
	return false
}

// generateRememberToken creates a signed token binding user ID to the device.
func (m *multiUserAuthMiddleware) generateRememberToken(userID int64) string {
	nonce := rand.String(rememberDeviceCookieLen)
	mac := hmac.New(sha256.New, []byte(m.shortID))
	mac.Write([]byte(fmt.Sprintf("%d:%s", userID, nonce)))
	sig := hex.EncodeToString(mac.Sum(nil))
	return fmt.Sprintf("%d:%s:%s", userID, nonce, sig)
}

// validateRememberToken verifies a remember-device token.
func (m *multiUserAuthMiddleware) validateRememberToken(token string, userID int64) bool {
	parts := strings.SplitN(token, ":", 3)
	if len(parts) != 3 {
		return false
	}
	if parts[0] != fmt.Sprintf("%d", userID) {
		return false
	}
	// The nonce differs, so recompute the signature for the given nonce
	mac := hmac.New(sha256.New, []byte(m.shortID))
	mac.Write([]byte(fmt.Sprintf("%d:%s", userID, parts[1])))
	expectedSig := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(parts[2]), []byte(expectedSig))
}

func (m *multiUserAuthMiddleware) resetPasswordHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Token       string `json:"token"`
		NewPassword string `json:"newPassword"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxLoginRequestSize)).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if req.Token == "" || req.NewPassword == "" {
		http.Error(w, "Token and new password are required", http.StatusBadRequest)
		return
	}

	if err := m.userManager.ResetPassword(req.Token, req.NewPassword); err != nil {
		if err == users.ErrResetTokenInvalid {
			http.Error(w, "Invalid or expired reset token", http.StatusBadRequest)
			return
		}
		slog.Error("Password reset failed", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (m *multiUserAuthMiddleware) handleLogout(w http.ResponseWriter, r *http.Request) {
	for _, cookie := range r.Cookies() {
		if cookie.Name == m.cookieName {
			m.userManager.InvalidateSession(cookie.Value)
			http.SetCookie(w, &http.Cookie{
				Name:   m.cookieName,
				Value:  "",
				MaxAge: -1,
				Secure: cookie.Secure,
				Path:   cookie.Path,
			})
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

func (m *multiUserAuthMiddleware) cleanupPendingMFA() {
	m.mfaPendingMu.Lock()
	defer m.mfaPendingMu.Unlock()
	now := time.Now()
	for token, entry := range m.mfaPending {
		if entry.expiresAt.Before(now) {
			delete(m.mfaPending, token)
		}
	}
}

func isForwardedHTTPS(r *http.Request) bool {
	return strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") ||
		strings.Contains(strings.ToLower(r.Header.Get("Forwarded")), "proto=https")
}
