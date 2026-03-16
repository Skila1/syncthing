-- Copyright (C) 2026 The Syncthing Authors.
--
-- This Source Code Form is subject to the terms of the Mozilla Public
-- License, v. 2.0. If a copy of the MPL was not distributed with this file,
-- You can obtain one at https://mozilla.org/MPL/2.0/.

CREATE TABLE IF NOT EXISTS storage_pools (
    id INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE COLLATE BINARY,
    path TEXT NOT NULL UNIQUE COLLATE BINARY,
    total_bytes INTEGER NOT NULL DEFAULT 0,
    used_bytes INTEGER NOT NULL DEFAULT 0,
    status TEXT NOT NULL DEFAULT 'online' COLLATE BINARY,
    strategy TEXT NOT NULL DEFAULT 'manual' COLLATE BINARY,
    created_at INTEGER NOT NULL
) STRICT;

CREATE TABLE IF NOT EXISTS user_pool_assignments (
    user_id INTEGER NOT NULL PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    pool_id INTEGER NOT NULL REFERENCES storage_pools(id) ON DELETE CASCADE
) STRICT;

CREATE INDEX IF NOT EXISTS user_pool_assignments_pool ON user_pool_assignments (pool_id);
