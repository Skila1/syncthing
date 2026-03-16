-- Copyright (C) 2026 The Syncthing Authors.
--
-- This Source Code Form is subject to the terms of the Mozilla Public
-- License, v. 2.0. If a copy of the MPL was not distributed with this file,
-- You can obtain one at https://mozilla.org/MPL/2.0/.

CREATE TABLE IF NOT EXISTS backups (
    id INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    folder_id TEXT NOT NULL DEFAULT '' COLLATE BINARY,
    type TEXT NOT NULL DEFAULT 'full' COLLATE BINARY,
    status TEXT NOT NULL DEFAULT 'pending' COLLATE BINARY,
    started_at INTEGER NOT NULL DEFAULT 0,
    completed_at INTEGER NOT NULL DEFAULT 0,
    size_bytes INTEGER NOT NULL DEFAULT 0,
    backup_path TEXT NOT NULL DEFAULT '' COLLATE BINARY,
    created_at INTEGER NOT NULL
) STRICT;

CREATE INDEX IF NOT EXISTS backups_user ON backups (user_id, created_at);

CREATE TABLE IF NOT EXISTS snapshots (
    id INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    folder_id TEXT NOT NULL COLLATE BINARY,
    name TEXT NOT NULL DEFAULT '' COLLATE BINARY,
    snapshot_path TEXT NOT NULL DEFAULT '' COLLATE BINARY,
    size_bytes INTEGER NOT NULL DEFAULT 0,
    metadata_json TEXT NOT NULL DEFAULT '{}' COLLATE BINARY,
    created_at INTEGER NOT NULL
) STRICT;

CREATE INDEX IF NOT EXISTS snapshots_user ON snapshots (user_id, folder_id, created_at);
