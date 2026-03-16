-- Copyright (C) 2026 The Syncthing Authors.
--
-- This Source Code Form is subject to the terms of the Mozilla Public
-- License, v. 2.0. If a copy of the MPL was not distributed with this file,
-- You can obtain one at https://mozilla.org/MPL/2.0/.

CREATE TABLE IF NOT EXISTS groups (
    id INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE COLLATE BINARY,
    description TEXT NOT NULL DEFAULT '' COLLATE BINARY,
    created_at INTEGER NOT NULL
) STRICT;

CREATE TABLE IF NOT EXISTS group_members (
    group_id INTEGER NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    PRIMARY KEY (group_id, user_id)
) STRICT;

CREATE INDEX IF NOT EXISTS group_members_user ON group_members (user_id);

CREATE TABLE IF NOT EXISTS group_folders (
    group_id INTEGER NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    folder_id TEXT NOT NULL COLLATE BINARY,
    permission TEXT NOT NULL DEFAULT 'read' COLLATE BINARY,
    PRIMARY KEY (group_id, folder_id)
) STRICT;

CREATE TABLE IF NOT EXISTS comments (
    id INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    folder_id TEXT NOT NULL COLLATE BINARY,
    file_path TEXT NOT NULL DEFAULT '' COLLATE BINARY,
    content TEXT NOT NULL COLLATE BINARY,
    created_at INTEGER NOT NULL
) STRICT;

CREATE INDEX IF NOT EXISTS comments_folder_file ON comments (folder_id, file_path, created_at);
CREATE INDEX IF NOT EXISTS comments_user ON comments (user_id);
