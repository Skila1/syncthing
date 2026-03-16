// Copyright (C) 2026 The Syncthing Authors.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at https://mozilla.org/MPL/2.0/.

package sqlite

import (
	"github.com/syncthing/syncthing/lib/trash"
)

// CleanupStore implements trash.CleanupStore backed by SQLite.
type CleanupStore struct {
	db *baseDB
}

func NewCleanupStore(db *DB) *CleanupStore {
	return &CleanupStore{db: db.baseDB}
}

func (s *CleanupStore) GetCleanupPolicies(userID int64) ([]trash.CleanupPolicy, error) {
	var result []trash.CleanupPolicy
	err := s.db.stmt(`
		SELECT id, user_id, max_age_days, max_size_bytes, pattern, enabled
		FROM cleanup_policies
		WHERE user_id = ?
		ORDER BY id
	`).Select(&result, userID)
	if err != nil {
		return nil, wrap(err)
	}
	return result, nil
}

func (s *CleanupStore) GetAllCleanupPolicies() ([]trash.CleanupPolicy, error) {
	var result []trash.CleanupPolicy
	err := s.db.stmt(`
		SELECT id, user_id, max_age_days, max_size_bytes, pattern, enabled
		FROM cleanup_policies
		ORDER BY user_id, id
	`).Select(&result)
	if err != nil {
		return nil, wrap(err)
	}
	return result, nil
}

func (s *CleanupStore) SetCleanupPolicy(policy *trash.CleanupPolicy) error {
	s.db.updateLock.Lock()
	defer s.db.updateLock.Unlock()

	if policy.ID > 0 {
		_, err := s.db.stmt(`
			UPDATE cleanup_policies SET max_age_days = ?, max_size_bytes = ?, pattern = ?, enabled = ?
			WHERE id = ?
		`).Exec(policy.MaxAgeDays, policy.MaxSizeBytes, policy.Pattern, policy.Enabled, policy.ID)
		return wrap(err)
	}

	result, err := s.db.stmt(`
		INSERT INTO cleanup_policies (user_id, max_age_days, max_size_bytes, pattern, enabled)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(user_id, pattern) DO UPDATE SET
			max_age_days = excluded.max_age_days,
			max_size_bytes = excluded.max_size_bytes,
			enabled = excluded.enabled
	`).Exec(policy.UserID, policy.MaxAgeDays, policy.MaxSizeBytes, policy.Pattern, policy.Enabled)
	if err != nil {
		return wrap(err)
	}

	id, err := result.LastInsertId()
	if err == nil {
		policy.ID = id
	}
	return nil
}

func (s *CleanupStore) DeleteCleanupPolicy(id int64) error {
	s.db.updateLock.Lock()
	defer s.db.updateLock.Unlock()

	_, err := s.db.stmt(`DELETE FROM cleanup_policies WHERE id = ?`).Exec(id)
	return wrap(err)
}
