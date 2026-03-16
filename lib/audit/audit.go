// Copyright (C) 2026 The Syncthing Authors.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at https://mozilla.org/MPL/2.0/.

package audit

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

const (
	ActionLogin          = "login"
	ActionLoginFailed    = "login_failed"
	ActionLogout         = "logout"
	ActionPasswordChange = "password_change"
	ActionPasswordReset  = "password_reset"
	ActionMFAEnable      = "mfa_enable"
	ActionMFADisable     = "mfa_disable"
	ActionUserCreate     = "user_create"
	ActionUserDelete     = "user_delete"
	ActionUserUpdate     = "user_update"
	ActionPermChange     = "permission_change"
	ActionShareCreate    = "share_create"
	ActionShareRevoke    = "share_revoke"
	ActionShareAccess    = "share_access"
	ActionFileUpload     = "file_upload"
	ActionFileDelete     = "file_delete"
	ActionFileRestore    = "file_restore"
	ActionIPRuleCreate   = "ip_rule_create"
	ActionIPRuleDelete   = "ip_rule_delete"
	ActionAdminAction    = "admin_action"
	ActionConfigChange   = "config_change"

	TargetUser   = "user"
	TargetFolder = "folder"
	TargetDevice = "device"
	TargetShare  = "share"
	TargetFile   = "file"
	TargetSystem = "system"
	TargetIPRule = "ip_rule"
)

type Entry struct {
	ID          int64  `json:"id" db:"id"`
	UserID      int64  `json:"userId" db:"user_id"`
	Action      string `json:"action" db:"action"`
	TargetType  string `json:"targetType" db:"target_type"`
	TargetID    string `json:"targetId" db:"target_id"`
	IPAddress   string `json:"ipAddress" db:"ip_address"`
	DetailsJSON string `json:"details" db:"details_json"`
	CreatedAt   int64  `json:"createdAt" db:"created_at"`
}

type Store interface {
	RecordAudit(e *Entry) error
	ListAudit(filter Filter) ([]Entry, error)
	CountAudit(filter Filter) (int64, error)
}

type Filter struct {
	UserID     int64
	Action     string
	TargetType string
	Since      int64
	Before     int64
	Limit      int
	Offset     int
}

type Logger struct {
	store Store
}

func NewLogger(store Store) *Logger {
	return &Logger{store: store}
}

// Log records an audit event. It is safe to call concurrently.
func (l *Logger) Log(userID int64, action, targetType, targetID, ip string, details map[string]interface{}) {
	if l == nil || l.store == nil {
		return
	}

	detailsBytes, _ := json.Marshal(details)
	if detailsBytes == nil {
		detailsBytes = []byte("{}")
	}

	entry := &Entry{
		UserID:      userID,
		Action:      action,
		TargetType:  targetType,
		TargetID:    targetID,
		IPAddress:   ip,
		DetailsJSON: string(detailsBytes),
		CreatedAt:   time.Now().UnixMilli(),
	}

	if err := l.store.RecordAudit(entry); err != nil {
		slog.Error("Failed to record audit entry", "error", err, "action", action)
	}
}

// LogRequest is a convenience wrapper that extracts the IP from an HTTP request.
func (l *Logger) LogRequest(userID int64, action, targetType, targetID string, r *http.Request, details map[string]interface{}) {
	l.Log(userID, action, targetType, targetID, ExtractIP(r), details)
}

// ExtractIP returns the client IP from the request, checking X-Forwarded-For
// and X-Real-Ip headers before falling back to RemoteAddr.
func ExtractIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.SplitN(xff, ",", 2)
		return strings.TrimSpace(parts[0])
	}
	if xri := r.Header.Get("X-Real-Ip"); xri != "" {
		return strings.TrimSpace(xri)
	}
	addr := r.RemoteAddr
	if idx := strings.LastIndex(addr, ":"); idx != -1 {
		return addr[:idx]
	}
	return addr
}
