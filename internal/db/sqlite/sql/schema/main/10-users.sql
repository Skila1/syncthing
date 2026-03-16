-- Copyright (C) 2026 The Syncthing Authors.
--
-- This Source Code Form is subject to the terms of the Mozilla Public
-- License, v. 2.0. If a copy of the MPL was not distributed with this file,
-- You can obtain one at https://mozilla.org/MPL/2.0/.

-- Multi-user support: users and sessions tables

CREATE TABLE IF NOT EXISTS users (
    id INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
    username TEXT NOT NULL UNIQUE COLLATE NOCASE,
    email TEXT NOT NULL DEFAULT '' COLLATE NOCASE,
    password_hash TEXT NOT NULL COLLATE BINARY,
    role TEXT NOT NULL DEFAULT 'user' COLLATE BINARY, -- 'admin' or 'user'
    root_path TEXT NOT NULL COLLATE BINARY,
    quota_bytes INTEGER NOT NULL DEFAULT 0, -- 0 = unlimited
    used_bytes INTEGER NOT NULL DEFAULT 0,
    mfa_enabled INTEGER NOT NULL DEFAULT 0, -- 0=disabled, 1=enabled
    mfa_secret TEXT NOT NULL DEFAULT '' COLLATE BINARY, -- base32-encoded TOTP secret
    created_at INTEGER NOT NULL, -- unix nanos
    updated_at INTEGER NOT NULL, -- unix nanos
    status TEXT NOT NULL DEFAULT 'active' COLLATE BINARY -- 'active', 'suspended', 'deleted'
) STRICT
;

CREATE TABLE IF NOT EXISTS user_sessions (
    token TEXT NOT NULL PRIMARY KEY COLLATE BINARY,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at INTEGER NOT NULL, -- unix nanos
    expires_at INTEGER NOT NULL, -- unix nanos
    ip_address TEXT NOT NULL DEFAULT '' COLLATE BINARY
) STRICT
;

CREATE INDEX IF NOT EXISTS user_sessions_user_id ON user_sessions (user_id)
;
CREATE INDEX IF NOT EXISTS user_sessions_expires_at ON user_sessions (expires_at)
;

CREATE TABLE IF NOT EXISTS password_resets (
    id INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token TEXT NOT NULL UNIQUE COLLATE BINARY,
    expires_at INTEGER NOT NULL, -- unix nanos
    used INTEGER NOT NULL DEFAULT 0 -- 0=unused, 1=used
) STRICT
;
CREATE INDEX IF NOT EXISTS password_resets_token ON password_resets (token)
;
CREATE INDEX IF NOT EXISTS password_resets_user_id ON password_resets (user_id)
;

CREATE TABLE IF NOT EXISTS mfa_recovery (
    id INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    code_hash TEXT NOT NULL COLLATE BINARY,
    used INTEGER NOT NULL DEFAULT 0 -- 0=unused, 1=used
) STRICT
;
CREATE INDEX IF NOT EXISTS mfa_recovery_user_id ON mfa_recovery (user_id)
;

CREATE TABLE IF NOT EXISTS folder_permissions (
    id INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
    folder_id TEXT NOT NULL COLLATE BINARY,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    permission TEXT NOT NULL DEFAULT 'read' COLLATE BINARY, -- 'owner', 'readwrite', 'read'
    created_at INTEGER NOT NULL, -- unix nanos
    UNIQUE(folder_id, user_id)
) STRICT
;
CREATE INDEX IF NOT EXISTS folder_permissions_folder ON folder_permissions (folder_id)
;
CREATE INDEX IF NOT EXISTS folder_permissions_user ON folder_permissions (user_id)
;

CREATE TABLE IF NOT EXISTS device_ownership (
    id INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
    device_id TEXT NOT NULL COLLATE BINARY, -- protocol.DeviceID.String()
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    approved INTEGER NOT NULL DEFAULT 0, -- 0=pending, 1=approved
    created_at INTEGER NOT NULL, -- unix nanos
    UNIQUE(device_id)
) STRICT
;
CREATE INDEX IF NOT EXISTS device_ownership_user ON device_ownership (user_id)
;
CREATE INDEX IF NOT EXISTS device_ownership_device ON device_ownership (device_id)
;

CREATE TABLE IF NOT EXISTS share_links (
    id INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
    token TEXT NOT NULL UNIQUE COLLATE BINARY,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    folder_id TEXT NOT NULL COLLATE BINARY,
    file_path TEXT NOT NULL COLLATE BINARY, -- relative path within folder, empty = whole folder
    expires_at INTEGER NOT NULL, -- unix nanos, 0 = never
    max_downloads INTEGER NOT NULL DEFAULT 0, -- 0 = unlimited
    download_count INTEGER NOT NULL DEFAULT 0,
    password_hash TEXT NOT NULL DEFAULT '' COLLATE BINARY, -- empty = no password
    created_at INTEGER NOT NULL -- unix nanos
) STRICT
;
CREATE INDEX IF NOT EXISTS share_links_token ON share_links (token)
;
CREATE INDEX IF NOT EXISTS share_links_user ON share_links (user_id)
;

CREATE TABLE IF NOT EXISTS cleanup_policies (
    id INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    max_age_days INTEGER NOT NULL DEFAULT 30,
    max_size_bytes INTEGER NOT NULL DEFAULT 0, -- 0 = unlimited
    pattern TEXT NOT NULL DEFAULT '' COLLATE BINARY, -- glob pattern, empty = all
    enabled INTEGER NOT NULL DEFAULT 1,
    UNIQUE(user_id, pattern)
) STRICT
;
CREATE INDEX IF NOT EXISTS cleanup_policies_user ON cleanup_policies (user_id)
;

CREATE TABLE IF NOT EXISTS activity_log (
    id INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    folder_id TEXT NOT NULL DEFAULT '' COLLATE BINARY,
    action TEXT NOT NULL COLLATE BINARY, -- file_create, file_modify, file_delete, file_share, sync_complete, user_login
    path TEXT NOT NULL DEFAULT '' COLLATE BINARY,
    detail TEXT NOT NULL DEFAULT '' COLLATE BINARY,
    created_at INTEGER NOT NULL -- unix millis
) STRICT
;
CREATE INDEX IF NOT EXISTS activity_log_user ON activity_log (user_id, created_at)
;
CREATE INDEX IF NOT EXISTS activity_log_folder ON activity_log (folder_id, created_at)
;

CREATE TABLE IF NOT EXISTS notifications (
    id INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    type TEXT NOT NULL COLLATE BINARY, -- info, warning, error, activity
    title TEXT NOT NULL DEFAULT '' COLLATE BINARY,
    message TEXT NOT NULL COLLATE BINARY,
    read INTEGER NOT NULL DEFAULT 0,
    created_at INTEGER NOT NULL -- unix millis
) STRICT
;
CREATE INDEX IF NOT EXISTS notifications_user ON notifications (user_id, read, created_at)
;

CREATE TABLE IF NOT EXISTS notification_prefs (
    id INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    folder_id TEXT NOT NULL DEFAULT '' COLLATE BINARY, -- empty = global default
    notify_create INTEGER NOT NULL DEFAULT 1,
    notify_modify INTEGER NOT NULL DEFAULT 0,
    notify_delete INTEGER NOT NULL DEFAULT 1,
    notify_share INTEGER NOT NULL DEFAULT 1,
    notify_email INTEGER NOT NULL DEFAULT 0,
    UNIQUE(user_id, folder_id)
) STRICT
;
CREATE INDEX IF NOT EXISTS notification_prefs_user ON notification_prefs (user_id)
;

CREATE TABLE IF NOT EXISTS user_bandwidth (
    user_id INTEGER NOT NULL PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    max_send_kbps INTEGER NOT NULL DEFAULT 0, -- 0 = unlimited
    max_recv_kbps INTEGER NOT NULL DEFAULT 0
) STRICT
;

CREATE TABLE IF NOT EXISTS sync_schedules (
    id INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    folder_id TEXT NOT NULL DEFAULT '' COLLATE BINARY, -- empty = all folders
    start_time TEXT NOT NULL COLLATE BINARY, -- "HH:MM" 24h format
    end_time TEXT NOT NULL COLLATE BINARY,
    days_of_week TEXT NOT NULL DEFAULT '0123456' COLLATE BINARY, -- 0=Sun 6=Sat
    enabled INTEGER NOT NULL DEFAULT 1
) STRICT
;
CREATE INDEX IF NOT EXISTS sync_schedules_user ON sync_schedules (user_id)
;

CREATE TABLE IF NOT EXISTS audit_log (
    id INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL DEFAULT 0,
    action TEXT NOT NULL COLLATE BINARY,
    target_type TEXT NOT NULL DEFAULT '' COLLATE BINARY,
    target_id TEXT NOT NULL DEFAULT '' COLLATE BINARY,
    ip_address TEXT NOT NULL DEFAULT '' COLLATE BINARY,
    details_json TEXT NOT NULL DEFAULT '{}' COLLATE BINARY,
    created_at INTEGER NOT NULL
) STRICT
;
CREATE INDEX IF NOT EXISTS audit_log_user ON audit_log (user_id, created_at)
;
CREATE INDEX IF NOT EXISTS audit_log_action ON audit_log (action, created_at)
;

CREATE TABLE IF NOT EXISTS ip_restrictions (
    id INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL DEFAULT 0,
    cidr TEXT NOT NULL COLLATE BINARY,
    action TEXT NOT NULL DEFAULT 'deny' COLLATE BINARY,
    description TEXT NOT NULL DEFAULT '' COLLATE BINARY,
    created_at INTEGER NOT NULL
) STRICT
;
CREATE INDEX IF NOT EXISTS ip_restrictions_user ON ip_restrictions (user_id)
;

CREATE TABLE IF NOT EXISTS encryption_keys (
    user_id INTEGER NOT NULL PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    encrypted_key BLOB NOT NULL,
    salt BLOB NOT NULL,
    escrow_key BLOB,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
) STRICT
;
