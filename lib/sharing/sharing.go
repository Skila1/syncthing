// Copyright (C) 2026 The Syncthing Authors.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at https://mozilla.org/MPL/2.0/.

package sharing

import (
	"errors"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/syncthing/syncthing/lib/rand"
)

const (
	TokenLength   = 32
	DefaultExpiry = 7 * 24 * time.Hour
	MaxExpiryDays = 90
)

var (
	ErrLinkNotFound    = errors.New("share link not found")
	ErrLinkExpired     = errors.New("share link has expired")
	ErrDownloadLimit   = errors.New("download limit reached")
	ErrInvalidPassword = errors.New("invalid share link password")
)

// ShareLink represents a temporary download link.
type ShareLink struct {
	ID            int64  `json:"id" db:"id"`
	Token         string `json:"token" db:"token"`
	UserID        int64  `json:"userId" db:"user_id"`
	FolderID      string `json:"folderId" db:"folder_id"`
	FilePath      string `json:"filePath" db:"file_path"`
	ExpiresAt     int64  `json:"expiresAt" db:"expires_at"`
	MaxDownloads  int    `json:"maxDownloads" db:"max_downloads"`
	DownloadCount int    `json:"downloadCount" db:"download_count"`
	PasswordHash  string `json:"-" db:"password_hash"`
	HasPassword   bool   `json:"hasPassword" db:"-"`
	CreatedAt     int64  `json:"createdAt" db:"created_at"`
	Username      string `json:"username,omitempty" db:"username"`
}

// Store defines persistence for share links.
type Store interface {
	CreateShareLink(link *ShareLink) error
	GetShareLink(token string) (*ShareLink, error)
	GetShareLinkByID(id int64) (*ShareLink, error)
	ListShareLinks(userID int64) ([]ShareLink, error)
	ListAllShareLinks() ([]ShareLink, error)
	IncrementDownloadCount(token string) error
	DeleteShareLink(id int64) error
	DeleteExpiredShareLinks() error
}

// Manager provides share link logic.
type Manager struct {
	store Store
}

func NewManager(store Store) *Manager {
	return &Manager{store: store}
}

// CreateLink generates a new share link.
func (m *Manager) CreateLink(userID int64, folderID, filePath, password string, expiresIn time.Duration, maxDownloads int) (*ShareLink, error) {
	token := rand.String(TokenLength)

	var expiresAt int64
	if expiresIn > 0 {
		expiresAt = time.Now().Add(expiresIn).UnixNano()
	}

	var pwHash string
	if password != "" {
		h, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			return nil, err
		}
		pwHash = string(h)
	}

	link := &ShareLink{
		Token:        token,
		UserID:       userID,
		FolderID:     folderID,
		FilePath:     filePath,
		ExpiresAt:    expiresAt,
		MaxDownloads: maxDownloads,
		PasswordHash: pwHash,
		CreatedAt:    time.Now().UnixNano(),
	}

	if err := m.store.CreateShareLink(link); err != nil {
		return nil, err
	}

	link.HasPassword = pwHash != ""
	return link, nil
}

// ValidateLink checks if a share link is valid and optionally verifies the
// password. Returns the link if valid.
func (m *Manager) ValidateLink(token, password string) (*ShareLink, error) {
	link, err := m.store.GetShareLink(token)
	if err != nil {
		return nil, ErrLinkNotFound
	}

	if link.ExpiresAt > 0 && link.ExpiresAt < time.Now().UnixNano() {
		return nil, ErrLinkExpired
	}

	if link.MaxDownloads > 0 && link.DownloadCount >= link.MaxDownloads {
		return nil, ErrDownloadLimit
	}

	if link.PasswordHash != "" {
		if password == "" {
			return link, ErrInvalidPassword
		}
		if err := bcrypt.CompareHashAndPassword([]byte(link.PasswordHash), []byte(password)); err != nil {
			return nil, ErrInvalidPassword
		}
	}

	return link, nil
}

// RecordDownload increments the download counter.
func (m *Manager) RecordDownload(token string) error {
	return m.store.IncrementDownloadCount(token)
}

// ListLinks returns share links for a user.
func (m *Manager) ListLinks(userID int64) ([]ShareLink, error) {
	links, err := m.store.ListShareLinks(userID)
	if err != nil {
		return nil, err
	}
	for i := range links {
		links[i].HasPassword = links[i].PasswordHash != ""
	}
	return links, nil
}

// ListAllLinks returns all share links (admin).
func (m *Manager) ListAllLinks() ([]ShareLink, error) {
	links, err := m.store.ListAllShareLinks()
	if err != nil {
		return nil, err
	}
	for i := range links {
		links[i].HasPassword = links[i].PasswordHash != ""
	}
	return links, nil
}

// RevokeLink deletes a share link.
func (m *Manager) RevokeLink(id int64) error {
	return m.store.DeleteShareLink(id)
}

// CleanExpired removes expired links.
func (m *Manager) CleanExpired() error {
	return m.store.DeleteExpiredShareLinks()
}
