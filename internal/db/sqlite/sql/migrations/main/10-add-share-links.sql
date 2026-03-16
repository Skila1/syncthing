-- Copyright (C) 2026 The Syncthing Authors.
--
-- This Source Code Form is subject to the terms of the Mozilla Public
-- License, v. 2.0. If a copy of the MPL was not distributed with this file,
-- You can obtain one at https://mozilla.org/MPL/2.0/.

CREATE TABLE IF NOT EXISTS share_links (
    id INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
    token TEXT NOT NULL UNIQUE COLLATE BINARY,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    folder_id TEXT NOT NULL COLLATE BINARY,
    file_path TEXT NOT NULL COLLATE BINARY,
    expires_at INTEGER NOT NULL,
    max_downloads INTEGER NOT NULL DEFAULT 0,
    download_count INTEGER NOT NULL DEFAULT 0,
    password_hash TEXT NOT NULL DEFAULT '' COLLATE BINARY,
    created_at INTEGER NOT NULL
) STRICT;

CREATE INDEX IF NOT EXISTS share_links_token ON share_links (token);
CREATE INDEX IF NOT EXISTS share_links_user ON share_links (user_id);
