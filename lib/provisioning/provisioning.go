// Copyright (C) 2026 The Syncthing Authors.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at https://mozilla.org/MPL/2.0/.

package provisioning

import (
	"encoding/csv"
	"io"
	"strings"
	"time"
)

// InviteLink is a signup token with optional constraints.
type InviteLink struct {
	ID         int64  `json:"id" db:"id"`
	Token      string `json:"token" db:"token"`
	Role       string `json:"role" db:"role"`
	QuotaBytes int64  `json:"quotaBytes" db:"quota_bytes"`
	MaxUses    int    `json:"maxUses" db:"max_uses"`
	Used       int    `json:"used" db:"used"`
	ExpiresAt  int64  `json:"expiresAt" db:"expires_at"`
	CreatedBy  int64  `json:"createdBy" db:"created_by"`
	CreatedAt  int64  `json:"createdAt" db:"created_at"`
}

// AlertRule defines a threshold that triggers an admin notification.
type AlertRule struct {
	ID        int64   `json:"id" db:"id"`
	Metric    string  `json:"metric" db:"metric"`
	Operator  string  `json:"operator" db:"operator"`
	Threshold float64 `json:"threshold" db:"threshold"`
	Enabled   bool    `json:"enabled" db:"enabled"`
	CreatedAt int64   `json:"createdAt" db:"created_at"`
}

// Store defines persistence for admin platform features.
type Store interface {
	ListInvites() ([]InviteLink, error)
	CreateInvite(inv *InviteLink) error
	DeleteInvite(id int64) error
	GetInviteByToken(token string) (*InviteLink, error)
	IncrementInviteUsed(id int64) error

	ListAlertRules() ([]AlertRule, error)
	CreateAlertRule(ar *AlertRule) error
	DeleteAlertRule(id int64) error
}

// CSVUser represents a single row from a bulk import CSV.
type CSVUser struct {
	Username string
	Password string
	Role     string
	Quota    string
}

// ParseCSV reads a CSV with columns: username, password, role, quota.
func ParseCSV(r io.Reader) ([]CSVUser, error) {
	reader := csv.NewReader(r)
	reader.TrimLeadingSpace = true

	var users []CSVUser
	header := true
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if header {
			header = false
			continue
		}
		if len(record) < 2 {
			continue
		}
		u := CSVUser{
			Username: strings.TrimSpace(record[0]),
			Password: strings.TrimSpace(record[1]),
		}
		if len(record) > 2 {
			u.Role = strings.TrimSpace(record[2])
		}
		if len(record) > 3 {
			u.Quota = strings.TrimSpace(record[3])
		}
		if u.Role == "" {
			u.Role = "user"
		}
		users = append(users, u)
	}
	return users, nil
}

// IsInviteValid checks if an invite link is still usable.
func IsInviteValid(inv *InviteLink) bool {
	if inv == nil {
		return false
	}
	if inv.MaxUses > 0 && inv.Used >= inv.MaxUses {
		return false
	}
	if inv.ExpiresAt > 0 && time.Now().UnixMilli() > inv.ExpiresAt {
		return false
	}
	return true
}
