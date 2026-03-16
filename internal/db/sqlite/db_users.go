// Copyright (C) 2026 The Syncthing Authors.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at https://mozilla.org/MPL/2.0/.

package sqlite

import (
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/syncthing/syncthing/lib/users"
)

// UserStore implements users.Store backed by the main SQLite database.
type UserStore struct {
	db *baseDB
}

func NewUserStore(db *DB) *UserStore {
	return &UserStore{db: db.baseDB}
}

func (s *UserStore) CreateUser(username, email, passwordHash, role, rootPath string) (*users.User, error) {
	s.db.updateLock.Lock()
	defer s.db.updateLock.Unlock()

	now := time.Now().UnixNano()
	result, err := s.db.stmt(`
		INSERT INTO users (username, email, password_hash, role, root_path, created_at, updated_at, status)
		VALUES (?, ?, ?, ?, ?, ?, ?, 'active')
	`).Exec(username, email, passwordHash, role, rootPath, now, now)
	if err != nil {
		if isUniqueConstraintError(err) {
			return nil, users.ErrUserExists
		}
		return nil, wrap(err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, wrap(err)
	}

	return &users.User{
		ID:           id,
		Username:     username,
		Email:        email,
		PasswordHash: passwordHash,
		Role:         role,
		RootPath:     rootPath,
		CreatedAt:    now,
		UpdatedAt:    now,
		Status:       users.StatusActive,
	}, nil
}

func (s *UserStore) GetUser(id int64) (*users.User, error) {
	var u users.User
	err := s.db.stmt(`
		SELECT id, username, email, password_hash, role, root_path, created_at, updated_at, status
		FROM users WHERE id = ?
	`).Get(&u, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, users.ErrUserNotFound
		}
		return nil, wrap(err)
	}
	return &u, nil
}

func (s *UserStore) GetUserByUsername(username string) (*users.User, error) {
	var u users.User
	err := s.db.stmt(`
		SELECT id, username, email, password_hash, role, root_path, created_at, updated_at, status
		FROM users WHERE username = ?
	`).Get(&u, username)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, users.ErrUserNotFound
		}
		return nil, wrap(err)
	}
	return &u, nil
}

func (s *UserStore) ListUsers() ([]users.User, error) {
	var result []users.User
	err := s.db.stmt(`
		SELECT id, username, email, password_hash, role, root_path, created_at, updated_at, status
		FROM users WHERE status != 'deleted'
		ORDER BY username
	`).Select(&result)
	if err != nil {
		return nil, wrap(err)
	}
	return result, nil
}

func (s *UserStore) UpdateUser(user *users.User) error {
	s.db.updateLock.Lock()
	defer s.db.updateLock.Unlock()

	_, err := s.db.stmt(`
		UPDATE users
		SET username = ?, email = ?, password_hash = ?, role = ?, root_path = ?, updated_at = ?, status = ?
		WHERE id = ?
	`).Exec(user.Username, user.Email, user.PasswordHash, user.Role, user.RootPath, user.UpdatedAt, user.Status, user.ID)
	return wrap(err)
}

func (s *UserStore) DeleteUser(id int64) error {
	s.db.updateLock.Lock()
	defer s.db.updateLock.Unlock()

	_, err := s.db.stmt(`
		UPDATE users SET status = 'deleted', updated_at = ? WHERE id = ?
	`).Exec(time.Now().UnixNano(), id)
	return wrap(err)
}

func (s *UserStore) CreateSession(token string, userID int64, expiresAt int64, ipAddress string) error {
	s.db.updateLock.Lock()
	defer s.db.updateLock.Unlock()

	_, err := s.db.stmt(`
		INSERT INTO user_sessions (token, user_id, created_at, expires_at, ip_address)
		VALUES (?, ?, ?, ?, ?)
	`).Exec(token, userID, time.Now().UnixNano(), expiresAt, ipAddress)
	return wrap(err)
}

func (s *UserStore) GetSession(token string) (*users.Session, bool, error) {
	var sess users.Session
	err := s.db.stmt(`
		SELECT token, user_id, created_at, expires_at, ip_address
		FROM user_sessions WHERE token = ?
	`).Get(&sess, token)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, false, nil
		}
		return nil, false, wrap(err)
	}
	return &sess, true, nil
}

func (s *UserStore) DeleteSession(token string) error {
	s.db.updateLock.Lock()
	defer s.db.updateLock.Unlock()

	_, err := s.db.stmt(`
		DELETE FROM user_sessions WHERE token = ?
	`).Exec(token)
	return wrap(err)
}

func (s *UserStore) DeleteExpiredSessions() error {
	s.db.updateLock.Lock()
	defer s.db.updateLock.Unlock()

	_, err := s.db.stmt(`
		DELETE FROM user_sessions WHERE expires_at < ?
	`).Exec(time.Now().UnixNano())
	return wrap(err)
}

func (s *UserStore) DeleteUserSessions(userID int64) error {
	s.db.updateLock.Lock()
	defer s.db.updateLock.Unlock()

	_, err := s.db.stmt(`
		DELETE FROM user_sessions WHERE user_id = ?
	`).Exec(userID)
	return wrap(err)
}

func (s *UserStore) CountUserSessions(userID int64) (int, error) {
	var count int
	err := s.db.stmt(`
		SELECT COUNT(*) FROM user_sessions WHERE user_id = ?
	`).Get(&count, userID)
	return count, wrap(err)
}

func (s *UserStore) DeleteOldestUserSession(userID int64) error {
	s.db.updateLock.Lock()
	defer s.db.updateLock.Unlock()

	_, err := s.db.stmt(`
		DELETE FROM user_sessions
		WHERE token = (
			SELECT token FROM user_sessions
			WHERE user_id = ?
			ORDER BY created_at ASC
			LIMIT 1
		)
	`).Exec(userID)
	return wrap(err)
}

func isUniqueConstraintError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "UNIQUE constraint failed") || strings.Contains(msg, "unique constraint")
}
