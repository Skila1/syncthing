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

	"github.com/syncthing/syncthing/lib/provisioning"
)

// ProvisioningStore implements provisioning.Store backed by SQLite.
type ProvisioningStore struct {
	db *baseDB
}

func NewProvisioningStore(db *DB) *ProvisioningStore {
	return &ProvisioningStore{db: db.baseDB}
}

func (s *ProvisioningStore) ListInvites() ([]provisioning.InviteLink, error) {
	var result []provisioning.InviteLink
	err := s.db.stmt(`
		SELECT id, token, role, quota_bytes, max_uses, used, expires_at, created_by, created_at
		FROM invite_links ORDER BY created_at DESC
	`).Select(&result)
	if err != nil {
		return nil, wrap(err)
	}
	return result, nil
}

func (s *ProvisioningStore) CreateInvite(inv *provisioning.InviteLink) error {
	s.db.updateLock.Lock()
	defer s.db.updateLock.Unlock()

	inv.CreatedAt = time.Now().UnixMilli()
	result, err := s.db.stmt(`
		INSERT INTO invite_links (token, role, quota_bytes, max_uses, used, expires_at, created_by, created_at)
		VALUES (?, ?, ?, ?, 0, ?, ?, ?)
	`).Exec(inv.Token, inv.Role, inv.QuotaBytes, inv.MaxUses, inv.ExpiresAt, inv.CreatedBy, inv.CreatedAt)
	if err != nil {
		return wrap(err)
	}
	id, _ := result.LastInsertId()
	inv.ID = id
	return nil
}

func (s *ProvisioningStore) DeleteInvite(id int64) error {
	s.db.updateLock.Lock()
	defer s.db.updateLock.Unlock()

	_, err := s.db.stmt(`DELETE FROM invite_links WHERE id = ?`).Exec(id)
	return wrap(err)
}

func (s *ProvisioningStore) GetInviteByToken(token string) (*provisioning.InviteLink, error) {
	var inv provisioning.InviteLink
	err := s.db.stmt(`
		SELECT id, token, role, quota_bytes, max_uses, used, expires_at, created_by, created_at
		FROM invite_links WHERE token = ?
	`).Get(&inv, token)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, wrap(err)
	}
	return &inv, nil
}

func (s *ProvisioningStore) IncrementInviteUsed(id int64) error {
	s.db.updateLock.Lock()
	defer s.db.updateLock.Unlock()

	_, err := s.db.stmt(`UPDATE invite_links SET used = used + 1 WHERE id = ?`).Exec(id)
	return wrap(err)
}

func (s *ProvisioningStore) ListAlertRules() ([]provisioning.AlertRule, error) {
	var result []provisioning.AlertRule
	err := s.db.stmt(`
		SELECT id, metric, operator, threshold, enabled, created_at
		FROM alert_rules ORDER BY id
	`).Select(&result)
	if err != nil {
		return nil, wrap(err)
	}
	return result, nil
}

func (s *ProvisioningStore) CreateAlertRule(ar *provisioning.AlertRule) error {
	s.db.updateLock.Lock()
	defer s.db.updateLock.Unlock()

	ar.CreatedAt = time.Now().UnixMilli()
	result, err := s.db.stmt(`
		INSERT INTO alert_rules (metric, operator, threshold, enabled, created_at)
		VALUES (?, ?, ?, ?, ?)
	`).Exec(ar.Metric, ar.Operator, ar.Threshold, ar.Enabled, ar.CreatedAt)
	if err != nil {
		return wrap(err)
	}
	id, _ := result.LastInsertId()
	ar.ID = id
	return nil
}

func (s *ProvisioningStore) DeleteAlertRule(id int64) error {
	s.db.updateLock.Lock()
	defer s.db.updateLock.Unlock()

	_, err := s.db.stmt(`DELETE FROM alert_rules WHERE id = ?`).Exec(id)
	return wrap(err)
}
