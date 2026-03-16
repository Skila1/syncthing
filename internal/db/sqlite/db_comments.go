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

	"github.com/syncthing/syncthing/lib/comments"
)

// CommentStore implements comments.Store backed by SQLite.
type CommentStore struct {
	db *baseDB
}

func NewCommentStore(db *DB) *CommentStore {
	return &CommentStore{db: db.baseDB}
}

func (s *CommentStore) ListComments(folderID, filePath string, limit, offset int) ([]comments.Comment, error) {
	if limit <= 0 {
		limit = 50
	}
	var result []comments.Comment
	err := s.db.stmt(`
		SELECT c.id, c.user_id, u.username, c.folder_id, c.file_path, c.content, c.created_at
		FROM comments c LEFT JOIN users u ON c.user_id = u.id
		WHERE c.folder_id = ? AND c.file_path = ?
		ORDER BY c.created_at DESC LIMIT ? OFFSET ?
	`).Select(&result, folderID, filePath, limit, offset)
	if err != nil {
		return nil, wrap(err)
	}
	return result, nil
}

func (s *CommentStore) CountComments(folderID, filePath string) (int64, error) {
	var count int64
	err := s.db.stmt(`
		SELECT COUNT(*) FROM comments WHERE folder_id = ? AND file_path = ?
	`).Get(&count, folderID, filePath)
	return count, wrap(err)
}

func (s *CommentStore) CreateComment(c *comments.Comment) error {
	s.db.updateLock.Lock()
	defer s.db.updateLock.Unlock()

	c.CreatedAt = time.Now().UnixMilli()
	result, err := s.db.stmt(`
		INSERT INTO comments (user_id, folder_id, file_path, content, created_at)
		VALUES (?, ?, ?, ?, ?)
	`).Exec(c.UserID, c.FolderID, c.FilePath, c.Content, c.CreatedAt)
	if err != nil {
		return wrap(err)
	}
	id, _ := result.LastInsertId()
	c.ID = id
	return nil
}

func (s *CommentStore) DeleteComment(id int64) error {
	s.db.updateLock.Lock()
	defer s.db.updateLock.Unlock()

	_, err := s.db.stmt(`DELETE FROM comments WHERE id = ?`).Exec(id)
	return wrap(err)
}

func (s *CommentStore) GetComment(id int64) (*comments.Comment, error) {
	var c comments.Comment
	err := s.db.stmt(`
		SELECT c.id, c.user_id, u.username, c.folder_id, c.file_path, c.content, c.created_at
		FROM comments c LEFT JOIN users u ON c.user_id = u.id
		WHERE c.id = ?
	`).Get(&c, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, wrap(err)
	}
	return &c, nil
}
