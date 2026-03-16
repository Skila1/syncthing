// Copyright (C) 2026 The Syncthing Authors.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at https://mozilla.org/MPL/2.0/.

package sqlite

import (
	"fmt"
	"strings"

	"github.com/syncthing/syncthing/lib/audit"
)

// AuditStore implements audit.Store backed by SQLite.
type AuditStore struct {
	db *baseDB
}

func NewAuditStore(db *DB) *AuditStore {
	return &AuditStore{db: db.baseDB}
}

func (s *AuditStore) RecordAudit(e *audit.Entry) error {
	s.db.updateLock.Lock()
	defer s.db.updateLock.Unlock()

	_, err := s.db.stmt(`
		INSERT INTO audit_log (user_id, action, target_type, target_id, ip_address, details_json, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`).Exec(e.UserID, e.Action, e.TargetType, e.TargetID, e.IPAddress, e.DetailsJSON, e.CreatedAt)
	return wrap(err)
}

func (s *AuditStore) ListAudit(f audit.Filter) ([]audit.Entry, error) {
	var conditions []string
	var args []interface{}

	if f.UserID > 0 {
		conditions = append(conditions, "user_id = ?")
		args = append(args, f.UserID)
	}
	if f.Action != "" {
		conditions = append(conditions, "action = ?")
		args = append(args, f.Action)
	}
	if f.TargetType != "" {
		conditions = append(conditions, "target_type = ?")
		args = append(args, f.TargetType)
	}
	if f.Since > 0 {
		conditions = append(conditions, "created_at >= ?")
		args = append(args, f.Since)
	}
	if f.Before > 0 {
		conditions = append(conditions, "created_at < ?")
		args = append(args, f.Before)
	}

	where := ""
	if len(conditions) > 0 {
		where = "WHERE " + strings.Join(conditions, " AND ")
	}

	limit := f.Limit
	if limit <= 0 {
		limit = 100
	}

	query := fmt.Sprintf(`
		SELECT id, user_id, action, target_type, target_id, ip_address, details_json, created_at
		FROM audit_log %s ORDER BY created_at DESC LIMIT ? OFFSET ?
	`, where)
	args = append(args, limit, f.Offset)

	var result []audit.Entry
	err := s.db.sql.Select(&result, query, args...)
	if err != nil {
		return nil, wrap(err)
	}
	return result, nil
}

func (s *AuditStore) CountAudit(f audit.Filter) (int64, error) {
	var conditions []string
	var args []interface{}

	if f.UserID > 0 {
		conditions = append(conditions, "user_id = ?")
		args = append(args, f.UserID)
	}
	if f.Action != "" {
		conditions = append(conditions, "action = ?")
		args = append(args, f.Action)
	}
	if f.TargetType != "" {
		conditions = append(conditions, "target_type = ?")
		args = append(args, f.TargetType)
	}
	if f.Since > 0 {
		conditions = append(conditions, "created_at >= ?")
		args = append(args, f.Since)
	}
	if f.Before > 0 {
		conditions = append(conditions, "created_at < ?")
		args = append(args, f.Before)
	}

	where := ""
	if len(conditions) > 0 {
		where = "WHERE " + strings.Join(conditions, " AND ")
	}

	query := fmt.Sprintf("SELECT COUNT(*) FROM audit_log %s", where)

	var count int64
	err := s.db.sql.Get(&count, query, args...)
	return count, wrap(err)
}
