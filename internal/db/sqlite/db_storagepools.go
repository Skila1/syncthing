// Copyright (C) 2026 The Syncthing Authors.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at https://mozilla.org/MPL/2.0/.

package sqlite

import (
	"database/sql"
	"errors"
	"time"

	"github.com/syncthing/syncthing/lib/storage"
)

// StoragePoolStore implements storage.Store backed by SQLite.
type StoragePoolStore struct {
	db *baseDB
}

func NewStoragePoolStore(db *DB) *StoragePoolStore {
	return &StoragePoolStore{db: db.baseDB}
}

func (s *StoragePoolStore) ListPools() ([]storage.Pool, error) {
	var result []storage.Pool
	err := s.db.stmt(`
		SELECT id, name, path, total_bytes, used_bytes, status, strategy, created_at
		FROM storage_pools ORDER BY name
	`).Select(&result)
	if err != nil {
		return nil, wrap(err)
	}
	return result, nil
}

func (s *StoragePoolStore) GetPool(id int64) (*storage.Pool, error) {
	var p storage.Pool
	err := s.db.stmt(`
		SELECT id, name, path, total_bytes, used_bytes, status, strategy, created_at
		FROM storage_pools WHERE id = ?
	`).Get(&p, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, wrap(err)
	}
	return &p, nil
}

func (s *StoragePoolStore) CreatePool(p *storage.Pool) error {
	s.db.updateLock.Lock()
	defer s.db.updateLock.Unlock()

	p.CreatedAt = time.Now().UnixMilli()
	if p.Status == "" {
		p.Status = storage.StatusOnline
	}
	if p.Strategy == "" {
		p.Strategy = storage.StrategyManual
	}
	result, err := s.db.stmt(`
		INSERT INTO storage_pools (name, path, total_bytes, used_bytes, status, strategy, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`).Exec(p.Name, p.Path, p.TotalBytes, p.UsedBytes, p.Status, p.Strategy, p.CreatedAt)
	if err != nil {
		return wrap(err)
	}
	id, _ := result.LastInsertId()
	p.ID = id
	return nil
}

func (s *StoragePoolStore) UpdatePool(p *storage.Pool) error {
	s.db.updateLock.Lock()
	defer s.db.updateLock.Unlock()

	_, err := s.db.stmt(`
		UPDATE storage_pools SET name = ?, path = ?, total_bytes = ?, status = ?, strategy = ?
		WHERE id = ?
	`).Exec(p.Name, p.Path, p.TotalBytes, p.Status, p.Strategy, p.ID)
	return wrap(err)
}

func (s *StoragePoolStore) DeletePool(id int64) error {
	s.db.updateLock.Lock()
	defer s.db.updateLock.Unlock()

	_, err := s.db.stmt(`DELETE FROM storage_pools WHERE id = ?`).Exec(id)
	return wrap(err)
}

func (s *StoragePoolStore) UpdatePoolUsage(id int64, usedBytes int64) error {
	s.db.updateLock.Lock()
	defer s.db.updateLock.Unlock()

	_, err := s.db.stmt(`UPDATE storage_pools SET used_bytes = ? WHERE id = ?`).Exec(usedBytes, id)
	return wrap(err)
}

func (s *StoragePoolStore) GetUserPool(userID int64) (*storage.UserPoolAssignment, error) {
	var a storage.UserPoolAssignment
	err := s.db.stmt(`SELECT user_id, pool_id FROM user_pool_assignments WHERE user_id = ?`).Get(&a, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, wrap(err)
	}
	return &a, nil
}

func (s *StoragePoolStore) SetUserPool(userID, poolID int64) error {
	s.db.updateLock.Lock()
	defer s.db.updateLock.Unlock()

	_, err := s.db.stmt(`
		INSERT INTO user_pool_assignments (user_id, pool_id) VALUES (?, ?)
		ON CONFLICT(user_id) DO UPDATE SET pool_id = excluded.pool_id
	`).Exec(userID, poolID)
	return wrap(err)
}

func (s *StoragePoolStore) RemoveUserPool(userID int64) error {
	s.db.updateLock.Lock()
	defer s.db.updateLock.Unlock()

	_, err := s.db.stmt(`DELETE FROM user_pool_assignments WHERE user_id = ?`).Exec(userID)
	return wrap(err)
}

func (s *StoragePoolStore) ListPoolUsers(poolID int64) ([]storage.UserPoolAssignment, error) {
	var result []storage.UserPoolAssignment
	err := s.db.stmt(`SELECT user_id, pool_id FROM user_pool_assignments WHERE pool_id = ?`).Select(&result, poolID)
	if err != nil {
		return nil, wrap(err)
	}
	return result, nil
}
