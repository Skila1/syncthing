// Copyright (C) 2026 The Syncthing Authors.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at https://mozilla.org/MPL/2.0/.

package comments

// Comment represents a user's note attached to a file or folder.
type Comment struct {
	ID        int64  `json:"id" db:"id"`
	UserID    int64  `json:"userId" db:"user_id"`
	Username  string `json:"username" db:"-"`
	FolderID  string `json:"folderId" db:"folder_id"`
	FilePath  string `json:"filePath" db:"file_path"`
	Content   string `json:"content" db:"content"`
	CreatedAt int64  `json:"createdAt" db:"created_at"`
}

// Store defines persistence operations for comments.
type Store interface {
	ListComments(folderID, filePath string, limit, offset int) ([]Comment, error)
	CountComments(folderID, filePath string) (int64, error)
	CreateComment(c *Comment) error
	DeleteComment(id int64) error
	GetComment(id int64) (*Comment, error)
}
