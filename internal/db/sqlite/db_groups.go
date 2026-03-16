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

	"github.com/syncthing/syncthing/lib/groups"
)

// GroupStore implements groups.Store backed by SQLite.
type GroupStore struct {
	db *baseDB
}

func NewGroupStore(db *DB) *GroupStore {
	return &GroupStore{db: db.baseDB}
}

func (s *GroupStore) ListGroups() ([]groups.Group, error) {
	var result []groups.Group
	err := s.db.stmt(`SELECT id, name, description, created_at FROM groups ORDER BY name`).Select(&result)
	if err != nil {
		return nil, wrap(err)
	}
	return result, nil
}

func (s *GroupStore) GetGroup(id int64) (*groups.Group, error) {
	var g groups.Group
	err := s.db.stmt(`SELECT id, name, description, created_at FROM groups WHERE id = ?`).Get(&g, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, wrap(err)
	}
	return &g, nil
}

func (s *GroupStore) CreateGroup(g *groups.Group) error {
	s.db.updateLock.Lock()
	defer s.db.updateLock.Unlock()

	g.CreatedAt = time.Now().UnixMilli()
	result, err := s.db.stmt(`
		INSERT INTO groups (name, description, created_at) VALUES (?, ?, ?)
	`).Exec(g.Name, g.Description, g.CreatedAt)
	if err != nil {
		return wrap(err)
	}
	id, _ := result.LastInsertId()
	g.ID = id
	return nil
}

func (s *GroupStore) UpdateGroup(g *groups.Group) error {
	s.db.updateLock.Lock()
	defer s.db.updateLock.Unlock()

	_, err := s.db.stmt(`UPDATE groups SET name = ?, description = ? WHERE id = ?`).Exec(g.Name, g.Description, g.ID)
	return wrap(err)
}

func (s *GroupStore) DeleteGroup(id int64) error {
	s.db.updateLock.Lock()
	defer s.db.updateLock.Unlock()

	_, err := s.db.stmt(`DELETE FROM groups WHERE id = ?`).Exec(id)
	return wrap(err)
}

func (s *GroupStore) ListGroupMembers(groupID int64) ([]groups.GroupMember, error) {
	var result []groups.GroupMember
	err := s.db.stmt(`SELECT group_id, user_id FROM group_members WHERE group_id = ?`).Select(&result, groupID)
	if err != nil {
		return nil, wrap(err)
	}
	return result, nil
}

func (s *GroupStore) ListUserGroups(userID int64) ([]groups.Group, error) {
	var result []groups.Group
	err := s.db.stmt(`
		SELECT g.id, g.name, g.description, g.created_at
		FROM groups g INNER JOIN group_members gm ON g.id = gm.group_id
		WHERE gm.user_id = ? ORDER BY g.name
	`).Select(&result, userID)
	if err != nil {
		return nil, wrap(err)
	}
	return result, nil
}

func (s *GroupStore) AddMember(groupID, userID int64) error {
	s.db.updateLock.Lock()
	defer s.db.updateLock.Unlock()

	_, err := s.db.stmt(`INSERT OR IGNORE INTO group_members (group_id, user_id) VALUES (?, ?)`).Exec(groupID, userID)
	return wrap(err)
}

func (s *GroupStore) RemoveMember(groupID, userID int64) error {
	s.db.updateLock.Lock()
	defer s.db.updateLock.Unlock()

	_, err := s.db.stmt(`DELETE FROM group_members WHERE group_id = ? AND user_id = ?`).Exec(groupID, userID)
	return wrap(err)
}

func (s *GroupStore) ListGroupFolders(groupID int64) ([]groups.GroupFolder, error) {
	var result []groups.GroupFolder
	err := s.db.stmt(`SELECT group_id, folder_id, permission FROM group_folders WHERE group_id = ?`).Select(&result, groupID)
	if err != nil {
		return nil, wrap(err)
	}
	return result, nil
}

func (s *GroupStore) SetGroupFolder(gf *groups.GroupFolder) error {
	s.db.updateLock.Lock()
	defer s.db.updateLock.Unlock()

	_, err := s.db.stmt(`
		INSERT INTO group_folders (group_id, folder_id, permission) VALUES (?, ?, ?)
		ON CONFLICT(group_id, folder_id) DO UPDATE SET permission = excluded.permission
	`).Exec(gf.GroupID, gf.FolderID, gf.Permission)
	return wrap(err)
}

func (s *GroupStore) RemoveGroupFolder(groupID int64, folderID string) error {
	s.db.updateLock.Lock()
	defer s.db.updateLock.Unlock()

	_, err := s.db.stmt(`DELETE FROM group_folders WHERE group_id = ? AND folder_id = ?`).Exec(groupID, folderID)
	return wrap(err)
}

func (s *GroupStore) ListFolderGroups(folderID string) ([]groups.GroupFolder, error) {
	var result []groups.GroupFolder
	err := s.db.stmt(`SELECT group_id, folder_id, permission FROM group_folders WHERE folder_id = ?`).Select(&result, folderID)
	if err != nil {
		return nil, wrap(err)
	}
	return result, nil
}
