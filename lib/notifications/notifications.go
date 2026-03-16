// Copyright (C) 2026 The Syncthing Authors.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at https://mozilla.org/MPL/2.0/.

package notifications

import (
	"log/slog"
	"time"

	"github.com/syncthing/syncthing/lib/email"
)

const (
	ActionFileCreate  = "file_create"
	ActionFileModify  = "file_modify"
	ActionFileDelete  = "file_delete"
	ActionFileShare   = "file_share"
	ActionSyncComplete = "sync_complete"
	ActionUserLogin   = "user_login"

	TypeInfo     = "info"
	TypeWarning  = "warning"
	TypeError    = "error"
	TypeActivity = "activity"

	MaxActivityAge = 90 * 24 * time.Hour
)

type ActivityEntry struct {
	ID        int64  `json:"id" db:"id"`
	UserID    int64  `json:"userId" db:"user_id"`
	FolderID  string `json:"folderId" db:"folder_id"`
	Action    string `json:"action" db:"action"`
	Path      string `json:"path" db:"path"`
	Detail    string `json:"detail" db:"detail"`
	CreatedAt int64  `json:"createdAt" db:"created_at"`
}

type Notification struct {
	ID        int64  `json:"id" db:"id"`
	UserID    int64  `json:"userId" db:"user_id"`
	Type      string `json:"type" db:"type"`
	Title     string `json:"title" db:"title"`
	Message   string `json:"message" db:"message"`
	Read      bool   `json:"read" db:"read"`
	CreatedAt int64  `json:"createdAt" db:"created_at"`
}

type NotificationPref struct {
	ID           int64  `json:"id" db:"id"`
	UserID       int64  `json:"userId" db:"user_id"`
	FolderID     string `json:"folderId" db:"folder_id"`
	NotifyCreate bool   `json:"notifyCreate" db:"notify_create"`
	NotifyModify bool   `json:"notifyModify" db:"notify_modify"`
	NotifyDelete bool   `json:"notifyDelete" db:"notify_delete"`
	NotifyShare  bool   `json:"notifyShare" db:"notify_share"`
	NotifyEmail  bool   `json:"notifyEmail" db:"notify_email"`
}

// Store defines persistence for activity and notifications.
type Store interface {
	RecordActivity(entry *ActivityEntry) error
	ListActivity(userID int64, folderID string, action string, limit, offset int) ([]ActivityEntry, error)
	DeleteOldActivity(before int64) error

	CreateNotification(n *Notification) error
	ListNotifications(userID int64, unreadOnly bool, limit, offset int) ([]Notification, error)
	CountUnread(userID int64) (int, error)
	MarkRead(id int64) error
	MarkAllRead(userID int64) error
	DeleteNotification(id int64) error

	GetNotificationPrefs(userID int64) ([]NotificationPref, error)
	SetNotificationPref(pref *NotificationPref) error
	DeleteNotificationPref(id int64) error
}

// Manager provides the notification and activity feed logic.
type Manager struct {
	store Store
	smtp  *email.SMTPConfig
}

func NewManager(store Store, smtp *email.SMTPConfig) *Manager {
	return &Manager{store: store, smtp: smtp}
}

// RecordActivity logs a user action and optionally creates a notification.
func (m *Manager) RecordActivity(userID int64, folderID, action, path, detail string) {
	entry := &ActivityEntry{
		UserID:    userID,
		FolderID:  folderID,
		Action:    action,
		Path:      path,
		Detail:    detail,
		CreatedAt: time.Now().UnixMilli(),
	}
	if err := m.store.RecordActivity(entry); err != nil {
		slog.Error("Failed to record activity", "error", err)
	}
}

// Notify creates an in-app notification for a user.
func (m *Manager) Notify(userID int64, nType, title, message string) {
	n := &Notification{
		UserID:    userID,
		Type:      nType,
		Title:     title,
		Message:   message,
		CreatedAt: time.Now().UnixMilli(),
	}
	if err := m.store.CreateNotification(n); err != nil {
		slog.Error("Failed to create notification", "error", err)
	}
}

// NotifyFileEvent checks preferences and creates a notification if the
// user has opted in for this action type in this folder.
func (m *Manager) NotifyFileEvent(userID int64, folderID, action, path, userEmail string) {
	prefs, err := m.store.GetNotificationPrefs(userID)
	if err != nil {
		return
	}

	var pref *NotificationPref
	for i := range prefs {
		if prefs[i].FolderID == folderID {
			pref = &prefs[i]
			break
		}
	}
	if pref == nil {
		for i := range prefs {
			if prefs[i].FolderID == "" {
				pref = &prefs[i]
				break
			}
		}
	}

	shouldNotify := true
	shouldEmail := false
	if pref != nil {
		switch action {
		case ActionFileCreate:
			shouldNotify = pref.NotifyCreate
		case ActionFileModify:
			shouldNotify = pref.NotifyModify
		case ActionFileDelete:
			shouldNotify = pref.NotifyDelete
		case ActionFileShare:
			shouldNotify = pref.NotifyShare
		}
		shouldEmail = pref.NotifyEmail && shouldNotify
	}

	if !shouldNotify {
		return
	}

	title := actionTitle(action)
	message := path
	if folderID != "" {
		message = folderID + ": " + path
	}

	m.Notify(userID, TypeActivity, title, message)

	if shouldEmail && m.smtp.IsConfigured() && userEmail != "" {
		if err := m.smtp.Send(userEmail, "Syncthing: "+title, message); err != nil {
			slog.Error("Failed to send notification email", "error", err)
		}
	}
}

func actionTitle(action string) string {
	switch action {
	case ActionFileCreate:
		return "File Created"
	case ActionFileModify:
		return "File Modified"
	case ActionFileDelete:
		return "File Deleted"
	case ActionFileShare:
		return "File Shared"
	case ActionSyncComplete:
		return "Sync Complete"
	case ActionUserLogin:
		return "Login"
	default:
		return "Activity"
	}
}

// ListActivity returns activity entries with optional filtering.
func (m *Manager) ListActivity(userID int64, folderID, action string, limit, offset int) ([]ActivityEntry, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 500 {
		limit = 500
	}
	return m.store.ListActivity(userID, folderID, action, limit, offset)
}

// ListNotifications returns notifications for a user.
func (m *Manager) ListNotifications(userID int64, unreadOnly bool, limit, offset int) ([]Notification, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	return m.store.ListNotifications(userID, unreadOnly, limit, offset)
}

// CountUnread returns the number of unread notifications.
func (m *Manager) CountUnread(userID int64) (int, error) {
	return m.store.CountUnread(userID)
}

// MarkRead marks a notification as read.
func (m *Manager) MarkRead(id int64) error {
	return m.store.MarkRead(id)
}

// MarkAllRead marks all notifications as read for a user.
func (m *Manager) MarkAllRead(userID int64) error {
	return m.store.MarkAllRead(userID)
}

// GetPrefs returns notification preferences for a user.
func (m *Manager) GetPrefs(userID int64) ([]NotificationPref, error) {
	return m.store.GetNotificationPrefs(userID)
}

// SetPref creates or updates a notification preference.
func (m *Manager) SetPref(pref *NotificationPref) error {
	return m.store.SetNotificationPref(pref)
}

// DeletePref removes a notification preference.
func (m *Manager) DeletePref(id int64) error {
	return m.store.DeleteNotificationPref(id)
}

// CleanOldActivity removes activity entries older than MaxActivityAge.
func (m *Manager) CleanOldActivity() error {
	cutoff := time.Now().Add(-MaxActivityAge).UnixMilli()
	return m.store.DeleteOldActivity(cutoff)
}
