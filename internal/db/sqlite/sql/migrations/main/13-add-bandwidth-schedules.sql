-- Copyright (C) 2026 The Syncthing Authors.
--
-- This Source Code Form is subject to the terms of the Mozilla Public
-- License, v. 2.0. If a copy of the MPL was not distributed with this file,
-- You can obtain one at https://mozilla.org/MPL/2.0/.

CREATE TABLE IF NOT EXISTS user_bandwidth (
    user_id INTEGER NOT NULL PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    max_send_kbps INTEGER NOT NULL DEFAULT 0,
    max_recv_kbps INTEGER NOT NULL DEFAULT 0
) STRICT;

CREATE TABLE IF NOT EXISTS sync_schedules (
    id INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    folder_id TEXT NOT NULL DEFAULT '' COLLATE BINARY,
    start_time TEXT NOT NULL COLLATE BINARY,
    end_time TEXT NOT NULL COLLATE BINARY,
    days_of_week TEXT NOT NULL DEFAULT '0123456' COLLATE BINARY,
    enabled INTEGER NOT NULL DEFAULT 1
) STRICT;

CREATE INDEX IF NOT EXISTS sync_schedules_user ON sync_schedules (user_id);
