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

	"github.com/syncthing/syncthing/lib/backup"
)

// BackupStore implements backup.Store backed by SQLite.
type BackupStore struct {
	db *baseDB
}

func NewBackupStore(db *DB) *BackupStore {
	return &BackupStore{db: db.baseDB}
}

func (s *BackupStore) ListBackups(userID int64) ([]backup.Backup, error) {
	var result []backup.Backup
	err := s.db.stmt(`
		SELECT id, user_id, folder_id, type, status, started_at, completed_at, size_bytes, backup_path, created_at
		FROM backups WHERE user_id = ? ORDER BY created_at DESC
	`).Select(&result, userID)
	if err != nil {
		return nil, wrap(err)
	}
	return result, nil
}

func (s *BackupStore) GetBackup(id int64) (*backup.Backup, error) {
	var b backup.Backup
	err := s.db.stmt(`
		SELECT id, user_id, folder_id, type, status, started_at, completed_at, size_bytes, backup_path, created_at
		FROM backups WHERE id = ?
	`).Get(&b, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, wrap(err)
	}
	return &b, nil
}

func (s *BackupStore) CreateBackup(b *backup.Backup) error {
	s.db.updateLock.Lock()
	defer s.db.updateLock.Unlock()

	b.CreatedAt = time.Now().UnixMilli()
	result, err := s.db.stmt(`
		INSERT INTO backups (user_id, folder_id, type, status, started_at, completed_at, size_bytes, backup_path, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`).Exec(b.UserID, b.FolderID, b.Type, b.Status, b.StartedAt, b.CompletedAt, b.SizeBytes, b.BackupPath, b.CreatedAt)
	if err != nil {
		return wrap(err)
	}
	id, _ := result.LastInsertId()
	b.ID = id
	return nil
}

func (s *BackupStore) UpdateBackup(b *backup.Backup) error {
	s.db.updateLock.Lock()
	defer s.db.updateLock.Unlock()

	_, err := s.db.stmt(`
		UPDATE backups SET status = ?, started_at = ?, completed_at = ?, size_bytes = ?, backup_path = ?
		WHERE id = ?
	`).Exec(b.Status, b.StartedAt, b.CompletedAt, b.SizeBytes, b.BackupPath, b.ID)
	return wrap(err)
}

func (s *BackupStore) DeleteBackup(id int64) error {
	s.db.updateLock.Lock()
	defer s.db.updateLock.Unlock()

	_, err := s.db.stmt(`DELETE FROM backups WHERE id = ?`).Exec(id)
	return wrap(err)
}

func (s *BackupStore) ListSnapshots(userID int64, folderID string) ([]backup.Snapshot, error) {
	var result []backup.Snapshot
	if folderID != "" {
		err := s.db.stmt(`
			SELECT id, user_id, folder_id, name, snapshot_path, size_bytes, metadata_json, created_at
			FROM snapshots WHERE user_id = ? AND folder_id = ? ORDER BY created_at DESC
		`).Select(&result, userID, folderID)
		if err != nil {
			return nil, wrap(err)
		}
	} else {
		err := s.db.stmt(`
			SELECT id, user_id, folder_id, name, snapshot_path, size_bytes, metadata_json, created_at
			FROM snapshots WHERE user_id = ? ORDER BY created_at DESC
		`).Select(&result, userID)
		if err != nil {
			return nil, wrap(err)
		}
	}
	return result, nil
}

func (s *BackupStore) GetSnapshot(id int64) (*backup.Snapshot, error) {
	var snap backup.Snapshot
	err := s.db.stmt(`
		SELECT id, user_id, folder_id, name, snapshot_path, size_bytes, metadata_json, created_at
		FROM snapshots WHERE id = ?
	`).Get(&snap, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, wrap(err)
	}
	return &snap, nil
}

func (s *BackupStore) CreateSnapshot(snap *backup.Snapshot) error {
	s.db.updateLock.Lock()
	defer s.db.updateLock.Unlock()

	result, err := s.db.stmt(`
		INSERT INTO snapshots (user_id, folder_id, name, snapshot_path, size_bytes, metadata_json, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`).Exec(snap.UserID, snap.FolderID, snap.Name, snap.SnapshotPath, snap.SizeBytes, snap.MetadataJSON, snap.CreatedAt)
	if err != nil {
		return wrap(err)
	}
	id, _ := result.LastInsertId()
	snap.ID = id
	return nil
}

func (s *BackupStore) DeleteSnapshot(id int64) error {
	s.db.updateLock.Lock()
	defer s.db.updateLock.Unlock()

	_, err := s.db.stmt(`DELETE FROM snapshots WHERE id = ?`).Exec(id)
	return wrap(err)
}
