// Copyright (C) 2026 The Syncthing Authors.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at https://mozilla.org/MPL/2.0/.

package users

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/syncthing/syncthing/lib/rand"
)

const (
	RoleAdmin = "admin"
	RoleUser  = "user"

	StatusActive    = "active"
	StatusSuspended = "suspended"

	MaxActiveSessions   = 25
	SessionLifetime     = 7 * 24 * time.Hour
	SessionTokenLength  = 64

	QuotaWarningPercent  = 80
	QuotaCriticalPercent = 95

	PasswordResetLifetime  = 1 * time.Hour
	PasswordResetTokenLen  = 64
	MFARecoveryCodeCount   = 10
	MFARecoveryCodeLength  = 8
	RememberDeviceLifetime = 30 * 24 * time.Hour
)

var (
	ErrUserNotFound       = errors.New("user not found")
	ErrUserExists         = errors.New("username already exists")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrAccountSuspended   = errors.New("account is suspended")
	ErrQuotaExceeded      = errors.New("storage quota exceeded")
	ErrMFARequired        = errors.New("MFA verification required")
	ErrInvalidMFACode     = errors.New("invalid MFA code")
	ErrResetTokenInvalid  = errors.New("reset token is invalid or expired")
	ErrMFANotEnabled      = errors.New("MFA is not enabled for this user")
)

type User struct {
	ID           int64  `json:"id" db:"id"`
	Username     string `json:"username" db:"username"`
	Email        string `json:"email" db:"email"`
	PasswordHash string `json:"-" db:"password_hash"`
	Role         string `json:"role" db:"role"`
	RootPath     string `json:"rootPath" db:"root_path"`
	QuotaBytes   int64  `json:"quotaBytes" db:"quota_bytes"`
	UsedBytes    int64  `json:"usedBytes" db:"used_bytes"`
	MFAEnabled   bool   `json:"mfaEnabled" db:"mfa_enabled"`
	MFASecret    string `json:"-" db:"mfa_secret"`
	CreatedAt    int64  `json:"createdAt" db:"created_at"`
	UpdatedAt    int64  `json:"updatedAt" db:"updated_at"`
	Status       string `json:"status" db:"status"`
}

func (u *User) IsAdmin() bool {
	return u.Role == RoleAdmin
}

type Session struct {
	Token     string `json:"token" db:"token"`
	UserID    int64  `json:"userId" db:"user_id"`
	CreatedAt int64  `json:"createdAt" db:"created_at"`
	ExpiresAt int64  `json:"expiresAt" db:"expires_at"`
	IPAddress string `json:"ipAddress" db:"ip_address"`
}

// PasswordReset represents a password reset token.
type PasswordReset struct {
	ID        int64  `db:"id"`
	UserID    int64  `db:"user_id"`
	Token     string `db:"token"`
	ExpiresAt int64  `db:"expires_at"`
	Used      bool   `db:"used"`
}

// MFARecoveryCode represents a one-time MFA recovery code.
type MFARecoveryCode struct {
	ID       int64  `db:"id"`
	UserID   int64  `db:"user_id"`
	CodeHash string `db:"code_hash"`
	Used     bool   `db:"used"`
}

// Store defines the interface for user data persistence.
type Store interface {
	CreateUser(username, email, passwordHash, role, rootPath string) (*User, error)
	GetUser(id int64) (*User, error)
	GetUserByUsername(username string) (*User, error)
	ListUsers() ([]User, error)
	UpdateUser(user *User) error
	UpdateUsedBytes(id int64, usedBytes int64) error
	DeleteUser(id int64) error

	CreateSession(token string, userID int64, expiresAt int64, ipAddress string) error
	GetSession(token string) (*Session, bool, error)
	DeleteSession(token string) error
	DeleteExpiredSessions() error
	DeleteUserSessions(userID int64) error
	CountUserSessions(userID int64) (int, error)
	DeleteOldestUserSession(userID int64) error

	CreatePasswordReset(userID int64, token string, expiresAt int64) error
	GetPasswordReset(token string) (*PasswordReset, error)
	MarkPasswordResetUsed(token string) error
	DeleteExpiredPasswordResets() error

	SaveMFARecoveryCodes(userID int64, codeHashes []string) error
	GetMFARecoveryCodes(userID int64) ([]MFARecoveryCode, error)
	MarkMFARecoveryCodeUsed(id int64) error
	DeleteMFARecoveryCodes(userID int64) error
}

// Manager coordinates user and session operations with an in-memory session
// cache for performance.
type Manager struct {
	store        Store
	userDataDir  string

	mu           sync.RWMutex
	sessionCache map[string]*cachedSession
}

type cachedSession struct {
	session *Session
	user    *User
}

func NewManager(store Store, userDataDir string) *Manager {
	m := &Manager{
		store:        store,
		userDataDir:  userDataDir,
		sessionCache: make(map[string]*cachedSession),
	}
	return m
}

func (m *Manager) CreateUser(username, email, password, role string) (*User, error) {
	if role != RoleAdmin && role != RoleUser {
		return nil, fmt.Errorf("invalid role: %s", role)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	rootPath := filepath.Join(m.userDataDir, username)

	user, err := m.store.CreateUser(username, email, string(hash), role, rootPath)
	if err != nil {
		return nil, err
	}

	if err := os.MkdirAll(rootPath, 0o700); err != nil {
		return nil, fmt.Errorf("create user root directory: %w", err)
	}

	return user, nil
}

func (m *Manager) Authenticate(username, password string) (*User, error) {
	user, err := m.store.GetUserByUsername(username)
	if err != nil {
		return nil, ErrInvalidCredentials
	}

	if user.Status != StatusActive {
		return nil, ErrAccountSuspended
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, ErrInvalidCredentials
	}

	return user, nil
}

func (m *Manager) CreateSession(userID int64, ipAddress string) (string, error) {
	count, err := m.store.CountUserSessions(userID)
	if err != nil {
		return "", err
	}
	if count >= MaxActiveSessions {
		if err := m.store.DeleteOldestUserSession(userID); err != nil {
			return "", err
		}
	}

	token := rand.String(SessionTokenLength)
	expiresAt := time.Now().Add(SessionLifetime).UnixNano()

	if err := m.store.CreateSession(token, userID, expiresAt, ipAddress); err != nil {
		return "", err
	}

	user, err := m.store.GetUser(userID)
	if err != nil {
		return "", err
	}

	m.mu.Lock()
	m.sessionCache[token] = &cachedSession{
		session: &Session{
			Token:     token,
			UserID:    userID,
			CreatedAt: time.Now().UnixNano(),
			ExpiresAt: expiresAt,
			IPAddress: ipAddress,
		},
		user: user,
	}
	m.mu.Unlock()

	return token, nil
}

// ValidateSession checks a session token and returns the associated user, or
// nil if the session is invalid or expired.
func (m *Manager) ValidateSession(token string) *User {
	m.mu.RLock()
	cached, ok := m.sessionCache[token]
	m.mu.RUnlock()

	if ok {
		if cached.session.ExpiresAt > time.Now().UnixNano() {
			return cached.user
		}
		m.InvalidateSession(token)
		return nil
	}

	sess, ok, err := m.store.GetSession(token)
	if err != nil || !ok {
		return nil
	}

	if sess.ExpiresAt < time.Now().UnixNano() {
		_ = m.store.DeleteSession(token)
		return nil
	}

	user, err := m.store.GetUser(sess.UserID)
	if err != nil {
		return nil
	}

	m.mu.Lock()
	m.sessionCache[token] = &cachedSession{session: sess, user: user}
	m.mu.Unlock()

	return user
}

func (m *Manager) InvalidateSession(token string) {
	m.mu.Lock()
	delete(m.sessionCache, token)
	m.mu.Unlock()
	_ = m.store.DeleteSession(token)
}

func (m *Manager) InvalidateUserSessions(userID int64) {
	m.mu.Lock()
	for token, cached := range m.sessionCache {
		if cached.session.UserID == userID {
			delete(m.sessionCache, token)
		}
	}
	m.mu.Unlock()
	_ = m.store.DeleteUserSessions(userID)
}

func (m *Manager) GetUser(id int64) (*User, error) {
	return m.store.GetUser(id)
}

func (m *Manager) GetUserByUsername(username string) (*User, error) {
	return m.store.GetUserByUsername(username)
}

func (m *Manager) ListUsers() ([]User, error) {
	return m.store.ListUsers()
}

func (m *Manager) UpdateUser(user *User) error {
	user.UpdatedAt = time.Now().UnixNano()
	return m.store.UpdateUser(user)
}

func (m *Manager) SetPassword(userID int64, newPassword string) error {
	user, err := m.store.GetUser(userID)
	if err != nil {
		return err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	user.PasswordHash = string(hash)
	user.UpdatedAt = time.Now().UnixNano()
	if err := m.store.UpdateUser(user); err != nil {
		return err
	}

	m.InvalidateUserSessions(userID)
	return nil
}

func (m *Manager) DeleteUser(id int64) error {
	m.InvalidateUserSessions(id)
	return m.store.DeleteUser(id)
}

// StorageStatus describes a user's current storage state.
type StorageStatus struct {
	UsedBytes    int64   `json:"usedBytes"`
	QuotaBytes   int64   `json:"quotaBytes"`
	UsagePercent float64 `json:"usagePercent"`
	Level        string  `json:"level"` // "ok", "warning", "critical", "exceeded"
}

func (m *Manager) GetStorageStatus(userID int64) (*StorageStatus, error) {
	user, err := m.store.GetUser(userID)
	if err != nil {
		return nil, err
	}

	s := &StorageStatus{
		UsedBytes:  user.UsedBytes,
		QuotaBytes: user.QuotaBytes,
	}

	if user.QuotaBytes > 0 {
		s.UsagePercent = float64(user.UsedBytes) / float64(user.QuotaBytes) * 100
	}

	switch {
	case user.QuotaBytes <= 0:
		s.Level = "ok"
	case s.UsagePercent >= 100:
		s.Level = "exceeded"
	case s.UsagePercent >= float64(QuotaCriticalPercent):
		s.Level = "critical"
	case s.UsagePercent >= float64(QuotaWarningPercent):
		s.Level = "warning"
	default:
		s.Level = "ok"
	}

	return s, nil
}

// CheckQuota returns ErrQuotaExceeded if accepting additionalBytes would
// push the user over quota. A quota of 0 means unlimited.
func (m *Manager) CheckQuota(userID int64, additionalBytes int64) error {
	user, err := m.store.GetUser(userID)
	if err != nil {
		return err
	}
	if user.QuotaBytes <= 0 {
		return nil
	}
	if user.UsedBytes+additionalBytes > user.QuotaBytes {
		return ErrQuotaExceeded
	}
	return nil
}

// RecalculateUsage walks the user's root directory and updates used_bytes.
func (m *Manager) RecalculateUsage(userID int64) (int64, error) {
	user, err := m.store.GetUser(userID)
	if err != nil {
		return 0, err
	}

	var total int64
	err = filepath.Walk(user.RootPath, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip unreadable entries
		}
		if !info.IsDir() {
			total += info.Size()
		}
		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("walk user directory: %w", err)
	}

	if err := m.store.UpdateUsedBytes(userID, total); err != nil {
		return 0, err
	}
	return total, nil
}

// RecalculateAllUsage recalculates disk usage for every active user.
func (m *Manager) RecalculateAllUsage() error {
	userList, err := m.store.ListUsers()
	if err != nil {
		return err
	}
	for _, u := range userList {
		if u.Status != StatusActive {
			continue
		}
		if _, err := m.RecalculateUsage(u.ID); err != nil {
			return fmt.Errorf("recalculate user %q: %w", u.Username, err)
		}
	}
	return nil
}

func (m *Manager) SetQuota(userID int64, quotaBytes int64) error {
	user, err := m.store.GetUser(userID)
	if err != nil {
		return err
	}
	user.QuotaBytes = quotaBytes
	user.UpdatedAt = time.Now().UnixNano()
	return m.store.UpdateUser(user)
}

// --- MFA ---

// EnableMFA stores the TOTP secret and enables MFA for the user.
func (m *Manager) EnableMFA(userID int64, secret string) error {
	user, err := m.store.GetUser(userID)
	if err != nil {
		return err
	}
	user.MFASecret = secret
	user.MFAEnabled = true
	user.UpdatedAt = time.Now().UnixNano()
	return m.store.UpdateUser(user)
}

// DisableMFA removes MFA configuration for a user.
func (m *Manager) DisableMFA(userID int64) error {
	user, err := m.store.GetUser(userID)
	if err != nil {
		return err
	}
	user.MFASecret = ""
	user.MFAEnabled = false
	user.UpdatedAt = time.Now().UnixNano()
	if err := m.store.UpdateUser(user); err != nil {
		return err
	}
	return m.store.DeleteMFARecoveryCodes(userID)
}

// GetMFASecret returns the user's MFA secret (for enrollment verification).
func (m *Manager) GetMFASecret(userID int64) (string, error) {
	user, err := m.store.GetUser(userID)
	if err != nil {
		return "", err
	}
	return user.MFASecret, nil
}

// StoreRecoveryCodes hashes and saves recovery codes. Returns the plaintext
// codes (caller should display them to the user exactly once).
func (m *Manager) StoreRecoveryCodes(userID int64, codes []string) error {
	hashes := make([]string, len(codes))
	for i, code := range codes {
		h, err := bcrypt.GenerateFromPassword([]byte(code), bcrypt.DefaultCost)
		if err != nil {
			return fmt.Errorf("hash recovery code: %w", err)
		}
		hashes[i] = string(h)
	}
	return m.store.SaveMFARecoveryCodes(userID, hashes)
}

// ValidateRecoveryCode checks a recovery code against stored hashes. On
// success the code is marked as used.
func (m *Manager) ValidateRecoveryCode(userID int64, code string) (bool, error) {
	codes, err := m.store.GetMFARecoveryCodes(userID)
	if err != nil {
		return false, err
	}
	for _, rc := range codes {
		if rc.Used {
			continue
		}
		if bcrypt.CompareHashAndPassword([]byte(rc.CodeHash), []byte(code)) == nil {
			return true, m.store.MarkMFARecoveryCodeUsed(rc.ID)
		}
	}
	return false, nil
}

// --- Password Reset ---

// CreatePasswordResetToken generates a reset token for the given user.
func (m *Manager) CreatePasswordResetToken(userID int64) (string, error) {
	_, err := m.store.GetUser(userID)
	if err != nil {
		return "", err
	}
	token := rand.String(PasswordResetTokenLen)
	expiresAt := time.Now().Add(PasswordResetLifetime).UnixNano()
	if err := m.store.CreatePasswordReset(userID, token, expiresAt); err != nil {
		return "", err
	}
	return token, nil
}

// ResetPassword validates a reset token and sets the new password.
func (m *Manager) ResetPassword(token, newPassword string) error {
	reset, err := m.store.GetPasswordReset(token)
	if err != nil {
		return ErrResetTokenInvalid
	}
	if reset.Used || reset.ExpiresAt < time.Now().UnixNano() {
		return ErrResetTokenInvalid
	}
	if err := m.store.MarkPasswordResetUsed(token); err != nil {
		return err
	}
	return m.SetPassword(reset.UserID, newPassword)
}

func (m *Manager) CleanExpiredSessions() error {
	m.mu.Lock()
	now := time.Now().UnixNano()
	for token, cached := range m.sessionCache {
		if cached.session.ExpiresAt < now {
			delete(m.sessionCache, token)
		}
	}
	m.mu.Unlock()

	return m.store.DeleteExpiredSessions()
}

func (m *Manager) HasUsers() (bool, error) {
	users, err := m.store.ListUsers()
	if err != nil {
		return false, err
	}
	return len(users) > 0, nil
}

// EnsureAdmin creates the initial admin account if no users exist. Returns
// true if an admin was created.
func (m *Manager) EnsureAdmin(username, password string) (bool, error) {
	has, err := m.HasUsers()
	if err != nil {
		return false, err
	}
	if has {
		return false, nil
	}

	_, err = m.CreateUser(username, "", password, RoleAdmin)
	if err != nil {
		return false, err
	}
	return true, nil
}
