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

	"github.com/syncthing/syncthing/lib/sharing"
)

// ShareStore implements sharing.Store backed by SQLite.
type ShareStore struct {
	db *baseDB
}

func NewShareStore(db *DB) *ShareStore {
	return &ShareStore{db: db.baseDB}
}

func (s *ShareStore) CreateShareLink(link *sharing.ShareLink) error {
	s.db.updateLock.Lock()
	defer s.db.updateLock.Unlock()

	result, err := s.db.stmt(`
		INSERT INTO share_links (token, user_id, folder_id, file_path, expires_at, max_downloads, download_count, password_hash, created_at)
		VALUES (?, ?, ?, ?, ?, ?, 0, ?, ?)
	`).Exec(link.Token, link.UserID, link.FolderID, link.FilePath, link.ExpiresAt, link.MaxDownloads, link.PasswordHash, link.CreatedAt)
	if err != nil {
		return wrap(err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return wrap(err)
	}
	link.ID = id
	return nil
}

func (s *ShareStore) GetShareLink(token string) (*sharing.ShareLink, error) {
	var link sharing.ShareLink
	err := s.db.stmt(`
		SELECT sl.id, sl.token, sl.user_id, sl.folder_id, sl.file_path, sl.expires_at,
		       sl.max_downloads, sl.download_count, sl.password_hash, sl.created_at,
		       u.username
		FROM share_links sl
		JOIN users u ON u.id = sl.user_id
		WHERE sl.token = ?
	`).Get(&link, token)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, sharing.ErrLinkNotFound
		}
		return nil, wrap(err)
	}
	return &link, nil
}

func (s *ShareStore) GetShareLinkByID(id int64) (*sharing.ShareLink, error) {
	var link sharing.ShareLink
	err := s.db.stmt(`
		SELECT sl.id, sl.token, sl.user_id, sl.folder_id, sl.file_path, sl.expires_at,
		       sl.max_downloads, sl.download_count, sl.password_hash, sl.created_at,
		       u.username
		FROM share_links sl
		JOIN users u ON u.id = sl.user_id
		WHERE sl.id = ?
	`).Get(&link, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, sharing.ErrLinkNotFound
		}
		return nil, wrap(err)
	}
	return &link, nil
}

func (s *ShareStore) ListShareLinks(userID int64) ([]sharing.ShareLink, error) {
	var result []sharing.ShareLink
	err := s.db.stmt(`
		SELECT sl.id, sl.token, sl.user_id, sl.folder_id, sl.file_path, sl.expires_at,
		       sl.max_downloads, sl.download_count, sl.password_hash, sl.created_at,
		       '' as username
		FROM share_links sl
		WHERE sl.user_id = ?
		ORDER BY sl.created_at DESC
	`).Select(&result, userID)
	if err != nil {
		return nil, wrap(err)
	}
	return result, nil
}

func (s *ShareStore) ListAllShareLinks() ([]sharing.ShareLink, error) {
	var result []sharing.ShareLink
	err := s.db.stmt(`
		SELECT sl.id, sl.token, sl.user_id, sl.folder_id, sl.file_path, sl.expires_at,
		       sl.max_downloads, sl.download_count, sl.password_hash, sl.created_at,
		       u.username
		FROM share_links sl
		JOIN users u ON u.id = sl.user_id
		ORDER BY sl.created_at DESC
	`).Select(&result)
	if err != nil {
		return nil, wrap(err)
	}
	return result, nil
}

func (s *ShareStore) IncrementDownloadCount(token string) error {
	s.db.updateLock.Lock()
	defer s.db.updateLock.Unlock()

	_, err := s.db.stmt(`
		UPDATE share_links SET download_count = download_count + 1 WHERE token = ?
	`).Exec(token)
	return wrap(err)
}

func (s *ShareStore) DeleteShareLink(id int64) error {
	s.db.updateLock.Lock()
	defer s.db.updateLock.Unlock()

	_, err := s.db.stmt(`DELETE FROM share_links WHERE id = ?`).Exec(id)
	return wrap(err)
}

func (s *ShareStore) DeleteExpiredShareLinks() error {
	s.db.updateLock.Lock()
	defer s.db.updateLock.Unlock()

	_, err := s.db.stmt(`
		DELETE FROM share_links WHERE expires_at > 0 AND expires_at < ?
	`).Exec(time.Now().UnixNano())
	return wrap(err)
}
