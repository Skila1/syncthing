// Copyright (C) 2026 The Syncthing Authors.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at https://mozilla.org/MPL/2.0/.

package sqlite

import (
	"time"

	"github.com/syncthing/syncthing/lib/audit"
)

// IPRestrictionStore implements audit.IPRestrictionStore backed by SQLite.
type IPRestrictionStore struct {
	db *baseDB
}

func NewIPRestrictionStore(db *DB) *IPRestrictionStore {
	return &IPRestrictionStore{db: db.baseDB}
}

func (s *IPRestrictionStore) ListRestrictions(userID int64) ([]audit.IPRestriction, error) {
	var result []audit.IPRestriction
	var err error
	if userID > 0 {
		err = s.db.stmt(`
			SELECT id, user_id, cidr, action, description, created_at
			FROM ip_restrictions WHERE user_id = ? OR user_id = 0 ORDER BY id
		`).Select(&result, userID)
	} else {
		err = s.db.stmt(`
			SELECT id, user_id, cidr, action, description, created_at
			FROM ip_restrictions ORDER BY id
		`).Select(&result)
	}
	if err != nil {
		return nil, wrap(err)
	}
	return result, nil
}

func (s *IPRestrictionStore) ListAllRestrictions() ([]audit.IPRestriction, error) {
	var result []audit.IPRestriction
	err := s.db.stmt(`
		SELECT id, user_id, cidr, action, description, created_at
		FROM ip_restrictions ORDER BY id
	`).Select(&result)
	if err != nil {
		return nil, wrap(err)
	}
	return result, nil
}

func (s *IPRestrictionStore) CreateRestriction(r *audit.IPRestriction) error {
	s.db.updateLock.Lock()
	defer s.db.updateLock.Unlock()

	r.CreatedAt = time.Now().UnixMilli()
	result, err := s.db.stmt(`
		INSERT INTO ip_restrictions (user_id, cidr, action, description, created_at)
		VALUES (?, ?, ?, ?, ?)
	`).Exec(r.UserID, r.CIDR, r.Action, r.Description, r.CreatedAt)
	if err != nil {
		return wrap(err)
	}
	id, _ := result.LastInsertId()
	r.ID = id
	return nil
}

func (s *IPRestrictionStore) DeleteRestriction(id int64) error {
	s.db.updateLock.Lock()
	defer s.db.updateLock.Unlock()

	_, err := s.db.stmt(`DELETE FROM ip_restrictions WHERE id = ?`).Exec(id)
	return wrap(err)
}
