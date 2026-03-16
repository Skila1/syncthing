// Copyright (C) 2026 The Syncthing Authors.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at https://mozilla.org/MPL/2.0/.

package permissions

import (
	"errors"
)

const (
	PermOwner     = "owner"
	PermReadWrite = "readwrite"
	PermRead      = "read"
)

var (
	ErrAccessDenied     = errors.New("access denied")
	ErrPermissionExists = errors.New("permission already exists")
)

// FolderPermission represents a user's access level to a folder.
type FolderPermission struct {
	ID         int64  `json:"id" db:"id"`
	FolderID   string `json:"folderId" db:"folder_id"`
	UserID     int64  `json:"userId" db:"user_id"`
	Permission string `json:"permission" db:"permission"`
	CreatedAt  int64  `json:"createdAt" db:"created_at"`
	Username   string `json:"username,omitempty" db:"username"` // joined field
}

// DeviceOwnership links a Syncthing device to a user.
type DeviceOwnership struct {
	ID        int64  `json:"id" db:"id"`
	DeviceID  string `json:"deviceId" db:"device_id"`
	UserID    int64  `json:"userId" db:"user_id"`
	Approved  bool   `json:"approved" db:"approved"`
	CreatedAt int64  `json:"createdAt" db:"created_at"`
	Username  string `json:"username,omitempty" db:"username"` // joined field
}

// Store defines persistence for permissions and device ownership.
type Store interface {
	SetFolderPermission(folderID string, userID int64, permission string) error
	GetFolderPermission(folderID string, userID int64) (*FolderPermission, error)
	GetFolderPermissions(folderID string) ([]FolderPermission, error)
	GetUserFolders(userID int64) ([]FolderPermission, error)
	DeleteFolderPermission(folderID string, userID int64) error
	DeleteAllFolderPermissions(folderID string) error

	SetDeviceOwner(deviceID string, userID int64) error
	GetDeviceOwner(deviceID string) (*DeviceOwnership, error)
	GetUserDevices(userID int64) ([]DeviceOwnership, error)
	ApproveDevice(deviceID string) error
	DeleteDeviceOwnership(deviceID string) error
}

// Manager provides permission-checking logic on top of the store.
type Manager struct {
	store Store
}

func NewManager(store Store) *Manager {
	return &Manager{store: store}
}

// CanReadFolder returns true if the user has any level of access to the folder.
func (m *Manager) CanReadFolder(folderID string, userID int64, isAdmin bool) bool {
	if isAdmin {
		return true
	}
	perm, err := m.store.GetFolderPermission(folderID, userID)
	if err != nil {
		return false
	}
	return perm.Permission == PermOwner || perm.Permission == PermReadWrite || perm.Permission == PermRead
}

// CanWriteFolder returns true if the user has write access to the folder.
func (m *Manager) CanWriteFolder(folderID string, userID int64, isAdmin bool) bool {
	if isAdmin {
		return true
	}
	perm, err := m.store.GetFolderPermission(folderID, userID)
	if err != nil {
		return false
	}
	return perm.Permission == PermOwner || perm.Permission == PermReadWrite
}

// IsOwner returns true if the user owns the folder.
func (m *Manager) IsOwner(folderID string, userID int64, isAdmin bool) bool {
	if isAdmin {
		return true
	}
	perm, err := m.store.GetFolderPermission(folderID, userID)
	if err != nil {
		return false
	}
	return perm.Permission == PermOwner
}

// SetPermission creates or updates a folder permission.
func (m *Manager) SetPermission(folderID string, userID int64, permission string) error {
	if permission != PermOwner && permission != PermReadWrite && permission != PermRead {
		return errors.New("invalid permission level")
	}
	return m.store.SetFolderPermission(folderID, userID, permission)
}

// GetFolderPermissions returns all permissions for a folder.
func (m *Manager) GetFolderPermissions(folderID string) ([]FolderPermission, error) {
	return m.store.GetFolderPermissions(folderID)
}

// GetUserFolders returns all folders the user has access to.
func (m *Manager) GetUserFolders(userID int64) ([]FolderPermission, error) {
	return m.store.GetUserFolders(userID)
}

// RemovePermission removes a user's access to a folder.
func (m *Manager) RemovePermission(folderID string, userID int64) error {
	return m.store.DeleteFolderPermission(folderID, userID)
}

// ClaimDevice assigns a device to a user.
func (m *Manager) ClaimDevice(deviceID string, userID int64) error {
	return m.store.SetDeviceOwner(deviceID, userID)
}

// GetDeviceOwner returns the owner of a device.
func (m *Manager) GetDeviceOwner(deviceID string) (*DeviceOwnership, error) {
	return m.store.GetDeviceOwner(deviceID)
}

// GetUserDevices returns all devices owned by a user.
func (m *Manager) GetUserDevices(userID int64) ([]DeviceOwnership, error) {
	return m.store.GetUserDevices(userID)
}

// ApproveDevice marks a device as approved.
func (m *Manager) ApproveDevice(deviceID string) error {
	return m.store.ApproveDevice(deviceID)
}

// ReleaseDevice removes device ownership.
func (m *Manager) ReleaseDevice(deviceID string) error {
	return m.store.DeleteDeviceOwnership(deviceID)
}

// IsDeviceOwner checks if a user owns a device.
func (m *Manager) IsDeviceOwner(deviceID string, userID int64, isAdmin bool) bool {
	if isAdmin {
		return true
	}
	owner, err := m.store.GetDeviceOwner(deviceID)
	if err != nil {
		return false
	}
	return owner.UserID == userID
}
