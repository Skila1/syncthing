-- Copyright (C) 2026 The Syncthing Authors.
--
-- This Source Code Form is subject to the terms of the Mozilla Public
-- License, v. 2.0. If a copy of the MPL was not distributed with this file,
-- You can obtain one at https://mozilla.org/MPL/2.0/.

CREATE TABLE IF NOT EXISTS activity_log (
    id INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    folder_id TEXT NOT NULL DEFAULT '' COLLATE BINARY,
    action TEXT NOT NULL COLLATE BINARY,
    path TEXT NOT NULL DEFAULT '' COLLATE BINARY,
    detail TEXT NOT NULL DEFAULT '' COLLATE BINARY,
    created_at INTEGER NOT NULL
) STRICT;

CREATE INDEX IF NOT EXISTS activity_log_user ON activity_log (user_id, created_at);
CREATE INDEX IF NOT EXISTS activity_log_folder ON activity_log (folder_id, created_at);

CREATE TABLE IF NOT EXISTS notifications (
    id INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    type TEXT NOT NULL COLLATE BINARY,
    title TEXT NOT NULL DEFAULT '' COLLATE BINARY,
    message TEXT NOT NULL COLLATE BINARY,
    read INTEGER NOT NULL DEFAULT 0,
    created_at INTEGER NOT NULL
) STRICT;

CREATE INDEX IF NOT EXISTS notifications_user ON notifications (user_id, read, created_at);

CREATE TABLE IF NOT EXISTS notification_prefs (
    id INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    folder_id TEXT NOT NULL DEFAULT '' COLLATE BINARY,
    notify_create INTEGER NOT NULL DEFAULT 1,
    notify_modify INTEGER NOT NULL DEFAULT 0,
    notify_delete INTEGER NOT NULL DEFAULT 1,
    notify_share INTEGER NOT NULL DEFAULT 1,
    notify_email INTEGER NOT NULL DEFAULT 0,
    UNIQUE(user_id, folder_id)
) STRICT;

CREATE INDEX IF NOT EXISTS notification_prefs_user ON notification_prefs (user_id);
