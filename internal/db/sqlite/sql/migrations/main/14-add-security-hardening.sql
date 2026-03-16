-- Copyright (C) 2026 The Syncthing Authors.
--
-- This Source Code Form is subject to the terms of the Mozilla Public
-- License, v. 2.0. If a copy of the MPL was not distributed with this file,
-- You can obtain one at https://mozilla.org/MPL/2.0/.

CREATE TABLE IF NOT EXISTS audit_log (
    id INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL DEFAULT 0,
    action TEXT NOT NULL COLLATE BINARY,
    target_type TEXT NOT NULL DEFAULT '' COLLATE BINARY,
    target_id TEXT NOT NULL DEFAULT '' COLLATE BINARY,
    ip_address TEXT NOT NULL DEFAULT '' COLLATE BINARY,
    details_json TEXT NOT NULL DEFAULT '{}' COLLATE BINARY,
    created_at INTEGER NOT NULL
) STRICT;

CREATE INDEX IF NOT EXISTS audit_log_user ON audit_log (user_id, created_at);
CREATE INDEX IF NOT EXISTS audit_log_action ON audit_log (action, created_at);

CREATE TABLE IF NOT EXISTS ip_restrictions (
    id INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL DEFAULT 0,
    cidr TEXT NOT NULL COLLATE BINARY,
    action TEXT NOT NULL DEFAULT 'deny' COLLATE BINARY,
    description TEXT NOT NULL DEFAULT '' COLLATE BINARY,
    created_at INTEGER NOT NULL
) STRICT;

CREATE INDEX IF NOT EXISTS ip_restrictions_user ON ip_restrictions (user_id);

CREATE TABLE IF NOT EXISTS encryption_keys (
    user_id INTEGER NOT NULL PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    encrypted_key BLOB NOT NULL,
    salt BLOB NOT NULL,
    escrow_key BLOB,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
) STRICT;
