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

	"github.com/syncthing/syncthing/lib/permissions"
)

// PermissionStore implements permissions.Store backed by SQLite.
type PermissionStore struct {
	db *baseDB
}

func NewPermissionStore(db *DB) *PermissionStore {
	return &PermissionStore{db: db.baseDB}
}

// --- Folder Permissions ---

func (s *PermissionStore) SetFolderPermission(folderID string, userID int64, permission string) error {
	s.db.updateLock.Lock()
	defer s.db.updateLock.Unlock()

	now := time.Now().UnixNano()
	_, err := s.db.stmt(`
		INSERT INTO folder_permissions (folder_id, user_id, permission, created_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(folder_id, user_id) DO UPDATE SET permission = excluded.permission
	`).Exec(folderID, userID, permission, now)
	return wrap(err)
}

func (s *PermissionStore) GetFolderPermission(folderID string, userID int64) (*permissions.FolderPermission, error) {
	var fp permissions.FolderPermission
	err := s.db.stmt(`
		SELECT fp.id, fp.folder_id, fp.user_id, fp.permission, fp.created_at, u.username
		FROM folder_permissions fp
		JOIN users u ON u.id = fp.user_id
		WHERE fp.folder_id = ? AND fp.user_id = ?
	`).Get(&fp, folderID, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, permissions.ErrAccessDenied
		}
		return nil, wrap(err)
	}
	return &fp, nil
}

func (s *PermissionStore) GetFolderPermissions(folderID string) ([]permissions.FolderPermission, error) {
	var result []permissions.FolderPermission
	err := s.db.stmt(`
		SELECT fp.id, fp.folder_id, fp.user_id, fp.permission, fp.created_at, u.username
		FROM folder_permissions fp
		JOIN users u ON u.id = fp.user_id
		WHERE fp.folder_id = ?
		ORDER BY fp.permission, u.username
	`).Select(&result, folderID)
	if err != nil {
		return nil, wrap(err)
	}
	return result, nil
}

func (s *PermissionStore) GetUserFolders(userID int64) ([]permissions.FolderPermission, error) {
	var result []permissions.FolderPermission
	err := s.db.stmt(`
		SELECT id, folder_id, user_id, permission, created_at, '' as username
		FROM folder_permissions
		WHERE user_id = ?
		ORDER BY folder_id
	`).Select(&result, userID)
	if err != nil {
		return nil, wrap(err)
	}
	return result, nil
}

func (s *PermissionStore) DeleteFolderPermission(folderID string, userID int64) error {
	s.db.updateLock.Lock()
	defer s.db.updateLock.Unlock()

	_, err := s.db.stmt(`
		DELETE FROM folder_permissions WHERE folder_id = ? AND user_id = ?
	`).Exec(folderID, userID)
	return wrap(err)
}

func (s *PermissionStore) DeleteAllFolderPermissions(folderID string) error {
	s.db.updateLock.Lock()
	defer s.db.updateLock.Unlock()

	_, err := s.db.stmt(`
		DELETE FROM folder_permissions WHERE folder_id = ?
	`).Exec(folderID)
	return wrap(err)
}

// --- Device Ownership ---

func (s *PermissionStore) SetDeviceOwner(deviceID string, userID int64) error {
	s.db.updateLock.Lock()
	defer s.db.updateLock.Unlock()

	now := time.Now().UnixNano()
	_, err := s.db.stmt(`
		INSERT INTO device_ownership (device_id, user_id, approved, created_at)
		VALUES (?, ?, 1, ?)
		ON CONFLICT(device_id) DO UPDATE SET user_id = excluded.user_id
	`).Exec(deviceID, userID, now)
	return wrap(err)
}

func (s *PermissionStore) GetDeviceOwner(deviceID string) (*permissions.DeviceOwnership, error) {
	var do permissions.DeviceOwnership
	err := s.db.stmt(`
		SELECT do.id, do.device_id, do.user_id, do.approved, do.created_at, u.username
		FROM device_ownership do
		JOIN users u ON u.id = do.user_id
		WHERE do.device_id = ?
	`).Get(&do, deviceID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, permissions.ErrAccessDenied
		}
		return nil, wrap(err)
	}
	return &do, nil
}

func (s *PermissionStore) GetUserDevices(userID int64) ([]permissions.DeviceOwnership, error) {
	var result []permissions.DeviceOwnership
	err := s.db.stmt(`
		SELECT id, device_id, user_id, approved, created_at, '' as username
		FROM device_ownership
		WHERE user_id = ?
		ORDER BY device_id
	`).Select(&result, userID)
	if err != nil {
		return nil, wrap(err)
	}
	return result, nil
}

func (s *PermissionStore) ApproveDevice(deviceID string) error {
	s.db.updateLock.Lock()
	defer s.db.updateLock.Unlock()

	_, err := s.db.stmt(`
		UPDATE device_ownership SET approved = 1 WHERE device_id = ?
	`).Exec(deviceID)
	return wrap(err)
}

func (s *PermissionStore) DeleteDeviceOwnership(deviceID string) error {
	s.db.updateLock.Lock()
	defer s.db.updateLock.Unlock()

	_, err := s.db.stmt(`
		DELETE FROM device_ownership WHERE device_id = ?
	`).Exec(deviceID)
	return wrap(err)
}
