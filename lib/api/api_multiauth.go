// Copyright (C) 2026 The Syncthing Authors.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at https://mozilla.org/MPL/2.0/.

package api

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/syncthing/syncthing/lib/config"
	"github.com/syncthing/syncthing/lib/events"
	"github.com/syncthing/syncthing/lib/users"
)

type multiUserAuthMiddleware struct {
	shortID     string
	cookieName  string
	guiCfg      config.GUIConfiguration
	userManager *users.Manager
	evLogger    events.Logger
	next        http.Handler
}

func newMultiUserAuthMiddleware(shortID string, guiCfg config.GUIConfiguration, userManager *users.Manager, evLogger events.Logger, next http.Handler) *multiUserAuthMiddleware {
	return &multiUserAuthMiddleware{
		shortID:     shortID,
		cookieName:  "sessionid-" + shortID,
		guiCfg:      guiCfg,
		userManager: userManager,
		evLogger:    evLogger,
		next:        next,
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
		Username     string
		Password     string
		StayLoggedIn bool
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

	remoteAddr, _ := remoteAddress(r)
	token, err := m.userManager.CreateSession(user.ID, remoteAddr)
	if err != nil {
		slog.Error("Failed to create session", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	connectionIsHTTPS := r.TLS != nil ||
		isForwardedHTTPS(r)
	useSecureCookie := connectionIsHTTPS || m.guiCfg.UseTLS()

	maxAge := 0
	if req.StayLoggedIn {
		maxAge = int(users.SessionLifetime.Seconds())
	}
	http.SetCookie(w, &http.Cookie{
		Name:   m.cookieName,
		Value:  token,
		MaxAge: maxAge,
		Secure: useSecureCookie,
		Path:   "/",
	})

	emitLoginAttempt(true, req.Username, r, m.evLogger)
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

func isForwardedHTTPS(r *http.Request) bool {
	return strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") ||
		strings.Contains(strings.ToLower(r.Header.Get("Forwarded")), "proto=https")
}
