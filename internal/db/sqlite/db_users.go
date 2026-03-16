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

const userColumns = `id, username, email, password_hash, role, root_path, quota_bytes, used_bytes, mfa_enabled, mfa_secret, created_at, updated_at, status`

func (s *UserStore) CreateUser(username, email, passwordHash, role, rootPath string) (*users.User, error) {
	s.db.updateLock.Lock()
	defer s.db.updateLock.Unlock()

	now := time.Now().UnixNano()
	result, err := s.db.stmt(`
		INSERT INTO users (username, email, password_hash, role, root_path, quota_bytes, used_bytes, mfa_enabled, mfa_secret, created_at, updated_at, status)
		VALUES (?, ?, ?, ?, ?, 0, 0, 0, '', ?, ?, 'active')
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
		SELECT ` + userColumns + `
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
		SELECT ` + userColumns + `
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
		SELECT ` + userColumns + `
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
		SET username = ?, email = ?, password_hash = ?, role = ?, root_path = ?,
		    quota_bytes = ?, used_bytes = ?, mfa_enabled = ?, mfa_secret = ?,
		    updated_at = ?, status = ?
		WHERE id = ?
	`).Exec(user.Username, user.Email, user.PasswordHash, user.Role, user.RootPath,
		user.QuotaBytes, user.UsedBytes, user.MFAEnabled, user.MFASecret,
		user.UpdatedAt, user.Status, user.ID)
	return wrap(err)
}

func (s *UserStore) UpdateUsedBytes(id int64, usedBytes int64) error {
	s.db.updateLock.Lock()
	defer s.db.updateLock.Unlock()

	_, err := s.db.stmt(`
		UPDATE users SET used_bytes = ?, updated_at = ? WHERE id = ?
	`).Exec(usedBytes, time.Now().UnixNano(), id)
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

// --- Password Reset ---

func (s *UserStore) CreatePasswordReset(userID int64, token string, expiresAt int64) error {
	s.db.updateLock.Lock()
	defer s.db.updateLock.Unlock()

	_, err := s.db.stmt(`
		INSERT INTO password_resets (user_id, token, expires_at, used)
		VALUES (?, ?, ?, 0)
	`).Exec(userID, token, expiresAt)
	return wrap(err)
}

func (s *UserStore) GetPasswordReset(token string) (*users.PasswordReset, error) {
	var pr users.PasswordReset
	err := s.db.stmt(`
		SELECT id, user_id, token, expires_at, used
		FROM password_resets WHERE token = ?
	`).Get(&pr, token)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, users.ErrResetTokenInvalid
		}
		return nil, wrap(err)
	}
	return &pr, nil
}

func (s *UserStore) MarkPasswordResetUsed(token string) error {
	s.db.updateLock.Lock()
	defer s.db.updateLock.Unlock()

	_, err := s.db.stmt(`
		UPDATE password_resets SET used = 1 WHERE token = ?
	`).Exec(token)
	return wrap(err)
}

func (s *UserStore) DeleteExpiredPasswordResets() error {
	s.db.updateLock.Lock()
	defer s.db.updateLock.Unlock()

	_, err := s.db.stmt(`
		DELETE FROM password_resets WHERE expires_at < ? OR used = 1
	`).Exec(time.Now().UnixNano())
	return wrap(err)
}

// --- MFA Recovery Codes ---

func (s *UserStore) SaveMFARecoveryCodes(userID int64, codeHashes []string) error {
	s.db.updateLock.Lock()
	defer s.db.updateLock.Unlock()

	// Remove existing codes first
	if _, err := s.db.stmt(`DELETE FROM mfa_recovery WHERE user_id = ?`).Exec(userID); err != nil {
		return wrap(err)
	}
	for _, hash := range codeHashes {
		if _, err := s.db.stmt(`
			INSERT INTO mfa_recovery (user_id, code_hash, used) VALUES (?, ?, 0)
		`).Exec(userID, hash); err != nil {
			return wrap(err)
		}
	}
	return nil
}

func (s *UserStore) GetMFARecoveryCodes(userID int64) ([]users.MFARecoveryCode, error) {
	var codes []users.MFARecoveryCode
	err := s.db.stmt(`
		SELECT id, user_id, code_hash, used
		FROM mfa_recovery WHERE user_id = ?
	`).Select(&codes, userID)
	if err != nil {
		return nil, wrap(err)
	}
	return codes, nil
}

func (s *UserStore) MarkMFARecoveryCodeUsed(id int64) error {
	s.db.updateLock.Lock()
	defer s.db.updateLock.Unlock()

	_, err := s.db.stmt(`UPDATE mfa_recovery SET used = 1 WHERE id = ?`).Exec(id)
	return wrap(err)
}

func (s *UserStore) DeleteMFARecoveryCodes(userID int64) error {
	s.db.updateLock.Lock()
	defer s.db.updateLock.Unlock()

	_, err := s.db.stmt(`DELETE FROM mfa_recovery WHERE user_id = ?`).Exec(userID)
	return wrap(err)
}

func isUniqueConstraintError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "UNIQUE constraint failed") || strings.Contains(msg, "unique constraint")
}
