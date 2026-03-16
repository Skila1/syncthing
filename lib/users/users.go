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
)

var (
	ErrUserNotFound      = errors.New("user not found")
	ErrUserExists        = errors.New("username already exists")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrAccountSuspended  = errors.New("account is suspended")
)

type User struct {
	ID           int64  `json:"id" db:"id"`
	Username     string `json:"username" db:"username"`
	Email        string `json:"email" db:"email"`
	PasswordHash string `json:"-" db:"password_hash"`
	Role         string `json:"role" db:"role"`
	RootPath     string `json:"rootPath" db:"root_path"`
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

// Store defines the interface for user data persistence.
type Store interface {
	CreateUser(username, email, passwordHash, role, rootPath string) (*User, error)
	GetUser(id int64) (*User, error)
	GetUserByUsername(username string) (*User, error)
	ListUsers() ([]User, error)
	UpdateUser(user *User) error
	DeleteUser(id int64) error

	CreateSession(token string, userID int64, expiresAt int64, ipAddress string) error
	GetSession(token string) (*Session, bool, error)
	DeleteSession(token string) error
	DeleteExpiredSessions() error
	DeleteUserSessions(userID int64) error
	CountUserSessions(userID int64) (int, error)
	DeleteOldestUserSession(userID int64) error
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
