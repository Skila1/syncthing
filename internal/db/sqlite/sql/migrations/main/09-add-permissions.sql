-- Copyright (C) 2026 The Syncthing Authors.
--
-- This Source Code Form is subject to the terms of the Mozilla Public
-- License, v. 2.0. If a copy of the MPL was not distributed with this file,
-- You can obtain one at https://mozilla.org/MPL/2.0/.

CREATE TABLE IF NOT EXISTS folder_permissions (
    id INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
    folder_id TEXT NOT NULL COLLATE BINARY,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    permission TEXT NOT NULL DEFAULT 'read' COLLATE BINARY,
    created_at INTEGER NOT NULL,
    UNIQUE(folder_id, user_id)
) STRICT;

CREATE INDEX IF NOT EXISTS folder_permissions_folder ON folder_permissions (folder_id);
CREATE INDEX IF NOT EXISTS folder_permissions_user ON folder_permissions (user_id);

CREATE TABLE IF NOT EXISTS device_ownership (
    id INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
    device_id TEXT NOT NULL COLLATE BINARY,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    approved INTEGER NOT NULL DEFAULT 0,
    created_at INTEGER NOT NULL,
    UNIQUE(device_id)
) STRICT;

CREATE INDEX IF NOT EXISTS device_ownership_user ON device_ownership (user_id);
CREATE INDEX IF NOT EXISTS device_ownership_device ON device_ownership (device_id);
