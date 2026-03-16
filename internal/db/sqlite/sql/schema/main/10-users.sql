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
