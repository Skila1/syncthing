// Copyright (C) 2026 The Syncthing Authors.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at https://mozilla.org/MPL/2.0/.

package sqlite

import (
	"database/sql"
	"errors"

	"github.com/syncthing/syncthing/lib/encryption"
)

// EncryptionStore implements encryption.Store backed by SQLite.
type EncryptionStore struct {
	db *baseDB
}

func NewEncryptionStore(db *DB) *EncryptionStore {
	return &EncryptionStore{db: db.baseDB}
}

func (s *EncryptionStore) GetEncryptionKey(userID int64) (*encryption.KeyRecord, error) {
	var rec encryption.KeyRecord
	err := s.db.stmt(`
		SELECT user_id, encrypted_key, salt, escrow_key, created_at, updated_at
		FROM encryption_keys WHERE user_id = ?
	`).Get(&rec, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, wrap(err)
	}
	return &rec, nil
}

func (s *EncryptionStore) SetEncryptionKey(rec *encryption.KeyRecord) error {
	s.db.updateLock.Lock()
	defer s.db.updateLock.Unlock()

	_, err := s.db.stmt(`
		INSERT INTO encryption_keys (user_id, encrypted_key, salt, escrow_key, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(user_id) DO UPDATE SET
			encrypted_key = excluded.encrypted_key,
			salt = excluded.salt,
			escrow_key = excluded.escrow_key,
			updated_at = excluded.updated_at
	`).Exec(rec.UserID, rec.EncryptedKey, rec.Salt, rec.EscrowKey, rec.CreatedAt, rec.UpdatedAt)
	return wrap(err)
}

func (s *EncryptionStore) DeleteEncryptionKey(userID int64) error {
	s.db.updateLock.Lock()
	defer s.db.updateLock.Unlock()

	_, err := s.db.stmt(`DELETE FROM encryption_keys WHERE user_id = ?`).Exec(userID)
	return wrap(err)
}
