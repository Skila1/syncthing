// Copyright (C) 2026 The Syncthing Authors.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at https://mozilla.org/MPL/2.0/.

package sqlite

import (
	"github.com/syncthing/syncthing/lib/notifications"
)

// NotificationStore implements notifications.Store backed by SQLite.
type NotificationStore struct {
	db *baseDB
}

func NewNotificationStore(db *DB) *NotificationStore {
	return &NotificationStore{db: db.baseDB}
}

// --- Activity ---

func (s *NotificationStore) RecordActivity(entry *notifications.ActivityEntry) error {
	s.db.updateLock.Lock()
	defer s.db.updateLock.Unlock()

	result, err := s.db.stmt(`
		INSERT INTO activity_log (user_id, folder_id, action, path, detail, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`).Exec(entry.UserID, entry.FolderID, entry.Action, entry.Path, entry.Detail, entry.CreatedAt)
	if err != nil {
		return wrap(err)
	}
	id, _ := result.LastInsertId()
	entry.ID = id
	return nil
}

func (s *NotificationStore) ListActivity(userID int64, folderID string, action string, limit, offset int) ([]notifications.ActivityEntry, error) {
	query := `SELECT id, user_id, folder_id, action, path, detail, created_at FROM activity_log WHERE user_id = ?`
	args := []interface{}{userID}

	if folderID != "" {
		query += ` AND folder_id = ?`
		args = append(args, folderID)
	}
	if action != "" {
		query += ` AND action = ?`
		args = append(args, action)
	}

	query += ` ORDER BY created_at DESC LIMIT ? OFFSET ?`
	args = append(args, limit, offset)

	var result []notifications.ActivityEntry
	err := s.db.sql.Select(&result, query, args...)
	if err != nil {
		return nil, wrap(err)
	}
	return result, nil
}

func (s *NotificationStore) DeleteOldActivity(before int64) error {
	s.db.updateLock.Lock()
	defer s.db.updateLock.Unlock()

	_, err := s.db.stmt(`DELETE FROM activity_log WHERE created_at < ?`).Exec(before)
	return wrap(err)
}

// --- Notifications ---

func (s *NotificationStore) CreateNotification(n *notifications.Notification) error {
	s.db.updateLock.Lock()
	defer s.db.updateLock.Unlock()

	result, err := s.db.stmt(`
		INSERT INTO notifications (user_id, type, title, message, read, created_at)
		VALUES (?, ?, ?, ?, 0, ?)
	`).Exec(n.UserID, n.Type, n.Title, n.Message, n.CreatedAt)
	if err != nil {
		return wrap(err)
	}
	id, _ := result.LastInsertId()
	n.ID = id
	return nil
}

func (s *NotificationStore) ListNotifications(userID int64, unreadOnly bool, limit, offset int) ([]notifications.Notification, error) {
	query := `SELECT id, user_id, type, title, message, read, created_at FROM notifications WHERE user_id = ?`
	args := []interface{}{userID}

	if unreadOnly {
		query += ` AND read = 0`
	}

	query += ` ORDER BY created_at DESC LIMIT ? OFFSET ?`
	args = append(args, limit, offset)

	var result []notifications.Notification
	err := s.db.sql.Select(&result, query, args...)
	if err != nil {
		return nil, wrap(err)
	}
	return result, nil
}

func (s *NotificationStore) CountUnread(userID int64) (int, error) {
	var count int
	err := s.db.stmt(`SELECT COUNT(*) FROM notifications WHERE user_id = ? AND read = 0`).Get(&count, userID)
	if err != nil {
		return 0, wrap(err)
	}
	return count, nil
}

func (s *NotificationStore) MarkRead(id int64) error {
	s.db.updateLock.Lock()
	defer s.db.updateLock.Unlock()

	_, err := s.db.stmt(`UPDATE notifications SET read = 1 WHERE id = ?`).Exec(id)
	return wrap(err)
}

func (s *NotificationStore) MarkAllRead(userID int64) error {
	s.db.updateLock.Lock()
	defer s.db.updateLock.Unlock()

	_, err := s.db.stmt(`UPDATE notifications SET read = 1 WHERE user_id = ? AND read = 0`).Exec(userID)
	return wrap(err)
}

func (s *NotificationStore) DeleteNotification(id int64) error {
	s.db.updateLock.Lock()
	defer s.db.updateLock.Unlock()

	_, err := s.db.stmt(`DELETE FROM notifications WHERE id = ?`).Exec(id)
	return wrap(err)
}

// --- Notification Preferences ---

func (s *NotificationStore) GetNotificationPrefs(userID int64) ([]notifications.NotificationPref, error) {
	var result []notifications.NotificationPref
	err := s.db.stmt(`
		SELECT id, user_id, folder_id, notify_create, notify_modify, notify_delete, notify_share, notify_email
		FROM notification_prefs WHERE user_id = ? ORDER BY folder_id
	`).Select(&result, userID)
	if err != nil {
		return nil, wrap(err)
	}
	return result, nil
}

func (s *NotificationStore) SetNotificationPref(pref *notifications.NotificationPref) error {
	s.db.updateLock.Lock()
	defer s.db.updateLock.Unlock()

	result, err := s.db.stmt(`
		INSERT INTO notification_prefs (user_id, folder_id, notify_create, notify_modify, notify_delete, notify_share, notify_email)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(user_id, folder_id) DO UPDATE SET
			notify_create = excluded.notify_create,
			notify_modify = excluded.notify_modify,
			notify_delete = excluded.notify_delete,
			notify_share = excluded.notify_share,
			notify_email = excluded.notify_email
	`).Exec(pref.UserID, pref.FolderID, pref.NotifyCreate, pref.NotifyModify, pref.NotifyDelete, pref.NotifyShare, pref.NotifyEmail)
	if err != nil {
		return wrap(err)
	}
	id, _ := result.LastInsertId()
	if id > 0 {
		pref.ID = id
	}
	return nil
}

func (s *NotificationStore) DeleteNotificationPref(id int64) error {
	s.db.updateLock.Lock()
	defer s.db.updateLock.Unlock()

	_, err := s.db.stmt(`DELETE FROM notification_prefs WHERE id = ?`).Exec(id)
	return wrap(err)
}
