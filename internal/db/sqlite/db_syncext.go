// Copyright (C) 2026 The Syncthing Authors.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at https://mozilla.org/MPL/2.0/.

package sqlite

import (
	"database/sql"
	"errors"

	"github.com/syncthing/syncthing/lib/syncext"
)

// SyncExtStore implements syncext.Store backed by SQLite.
type SyncExtStore struct {
	db *baseDB
}

func NewSyncExtStore(db *DB) *SyncExtStore {
	return &SyncExtStore{db: db.baseDB}
}

func (s *SyncExtStore) GetUserBandwidth(userID int64) (*syncext.UserBandwidth, error) {
	var bw syncext.UserBandwidth
	err := s.db.stmt(`
		SELECT user_id, max_send_kbps, max_recv_kbps
		FROM user_bandwidth WHERE user_id = ?
	`).Get(&bw, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return &syncext.UserBandwidth{UserID: userID}, nil
		}
		return nil, wrap(err)
	}
	return &bw, nil
}

func (s *SyncExtStore) SetUserBandwidth(bw *syncext.UserBandwidth) error {
	s.db.updateLock.Lock()
	defer s.db.updateLock.Unlock()

	_, err := s.db.stmt(`
		INSERT INTO user_bandwidth (user_id, max_send_kbps, max_recv_kbps)
		VALUES (?, ?, ?)
		ON CONFLICT(user_id) DO UPDATE SET
			max_send_kbps = excluded.max_send_kbps,
			max_recv_kbps = excluded.max_recv_kbps
	`).Exec(bw.UserID, bw.MaxSendKbps, bw.MaxRecvKbps)
	return wrap(err)
}

func (s *SyncExtStore) ListSchedules(userID int64) ([]syncext.SyncSchedule, error) {
	var result []syncext.SyncSchedule
	err := s.db.stmt(`
		SELECT id, user_id, folder_id, start_time, end_time, days_of_week, enabled
		FROM sync_schedules WHERE user_id = ? ORDER BY id
	`).Select(&result, userID)
	if err != nil {
		return nil, wrap(err)
	}
	return result, nil
}

func (s *SyncExtStore) SetSchedule(sched *syncext.SyncSchedule) error {
	s.db.updateLock.Lock()
	defer s.db.updateLock.Unlock()

	if sched.ID > 0 {
		_, err := s.db.stmt(`
			UPDATE sync_schedules SET folder_id = ?, start_time = ?, end_time = ?, days_of_week = ?, enabled = ?
			WHERE id = ?
		`).Exec(sched.FolderID, sched.StartTime, sched.EndTime, sched.DaysOfWeek, sched.Enabled, sched.ID)
		return wrap(err)
	}

	result, err := s.db.stmt(`
		INSERT INTO sync_schedules (user_id, folder_id, start_time, end_time, days_of_week, enabled)
		VALUES (?, ?, ?, ?, ?, ?)
	`).Exec(sched.UserID, sched.FolderID, sched.StartTime, sched.EndTime, sched.DaysOfWeek, sched.Enabled)
	if err != nil {
		return wrap(err)
	}

	id, _ := result.LastInsertId()
	sched.ID = id
	return nil
}

func (s *SyncExtStore) DeleteSchedule(id int64) error {
	s.db.updateLock.Lock()
	defer s.db.updateLock.Unlock()

	_, err := s.db.stmt(`DELETE FROM sync_schedules WHERE id = ?`).Exec(id)
	return wrap(err)
}
