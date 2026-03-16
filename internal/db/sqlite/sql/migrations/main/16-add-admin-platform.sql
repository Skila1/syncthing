-- Copyright (C) 2026 The Syncthing Authors.
--
-- This Source Code Form is subject to the terms of the Mozilla Public
-- License, v. 2.0. If a copy of the MPL was not distributed with this file,
-- You can obtain one at https://mozilla.org/MPL/2.0/.

CREATE TABLE IF NOT EXISTS invite_links (
    id INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
    token TEXT NOT NULL UNIQUE COLLATE BINARY,
    role TEXT NOT NULL DEFAULT 'user' COLLATE BINARY,
    quota_bytes INTEGER NOT NULL DEFAULT 0,
    max_uses INTEGER NOT NULL DEFAULT 1,
    used INTEGER NOT NULL DEFAULT 0,
    expires_at INTEGER NOT NULL DEFAULT 0,
    created_by INTEGER NOT NULL DEFAULT 0,
    created_at INTEGER NOT NULL
) STRICT;

CREATE TABLE IF NOT EXISTS alert_rules (
    id INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
    metric TEXT NOT NULL COLLATE BINARY,
    operator TEXT NOT NULL DEFAULT '>' COLLATE BINARY,
    threshold REAL NOT NULL,
    enabled INTEGER NOT NULL DEFAULT 1,
    created_at INTEGER NOT NULL
) STRICT;
