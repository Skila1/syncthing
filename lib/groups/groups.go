// Copyright (C) 2026 The Syncthing Authors.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at https://mozilla.org/MPL/2.0/.

package groups

// Group represents a named collection of users.
type Group struct {
	ID          int64  `json:"id" db:"id"`
	Name        string `json:"name" db:"name"`
	Description string `json:"description" db:"description"`
	CreatedAt   int64  `json:"createdAt" db:"created_at"`
}

// GroupMember links a user to a group.
type GroupMember struct {
	GroupID int64 `json:"groupId" db:"group_id"`
	UserID  int64 `json:"userId" db:"user_id"`
}

// GroupFolder shares a folder with a group at a permission level.
type GroupFolder struct {
	GroupID    int64  `json:"groupId" db:"group_id"`
	FolderID   string `json:"folderId" db:"folder_id"`
	Permission string `json:"permission" db:"permission"`
}

// Store defines persistence operations for group management.
type Store interface {
	ListGroups() ([]Group, error)
	GetGroup(id int64) (*Group, error)
	CreateGroup(g *Group) error
	UpdateGroup(g *Group) error
	DeleteGroup(id int64) error

	ListGroupMembers(groupID int64) ([]GroupMember, error)
	ListUserGroups(userID int64) ([]Group, error)
	AddMember(groupID, userID int64) error
	RemoveMember(groupID, userID int64) error

	ListGroupFolders(groupID int64) ([]GroupFolder, error)
	SetGroupFolder(gf *GroupFolder) error
	RemoveGroupFolder(groupID int64, folderID string) error
	ListFolderGroups(folderID string) ([]GroupFolder, error)
}

// Manager provides group management operations.
type Manager struct {
	store Store
}

func NewManager(store Store) *Manager {
	return &Manager{store: store}
}

func (m *Manager) ListGroups() ([]Group, error) {
	return m.store.ListGroups()
}

func (m *Manager) GetGroup(id int64) (*Group, error) {
	return m.store.GetGroup(id)
}

func (m *Manager) CreateGroup(g *Group) error {
	return m.store.CreateGroup(g)
}

func (m *Manager) UpdateGroup(g *Group) error {
	return m.store.UpdateGroup(g)
}

func (m *Manager) DeleteGroup(id int64) error {
	return m.store.DeleteGroup(id)
}

func (m *Manager) ListMembers(groupID int64) ([]GroupMember, error) {
	return m.store.ListGroupMembers(groupID)
}

func (m *Manager) ListUserGroups(userID int64) ([]Group, error) {
	return m.store.ListUserGroups(userID)
}

func (m *Manager) AddMember(groupID, userID int64) error {
	return m.store.AddMember(groupID, userID)
}

func (m *Manager) RemoveMember(groupID, userID int64) error {
	return m.store.RemoveMember(groupID, userID)
}

func (m *Manager) ListGroupFolders(groupID int64) ([]GroupFolder, error) {
	return m.store.ListGroupFolders(groupID)
}

func (m *Manager) SetGroupFolder(gf *GroupFolder) error {
	return m.store.SetGroupFolder(gf)
}

func (m *Manager) RemoveGroupFolder(groupID int64, folderID string) error {
	return m.store.RemoveGroupFolder(groupID, folderID)
}

// UserHasFolderAccess checks if a user has access to a folder via any group.
func (m *Manager) UserHasFolderAccess(userID int64, folderID string) (bool, string) {
	userGroups, err := m.store.ListUserGroups(userID)
	if err != nil {
		return false, ""
	}
	bestPerm := ""
	for _, g := range userGroups {
		folders, err := m.store.ListGroupFolders(g.ID)
		if err != nil {
			continue
		}
		for _, gf := range folders {
			if gf.FolderID == folderID {
				if gf.Permission == "readwrite" {
					return true, "readwrite"
				}
				if bestPerm == "" {
					bestPerm = gf.Permission
				}
			}
		}
	}
	return bestPerm != "", bestPerm
}
