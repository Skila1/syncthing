-- Copyright (C) 2026 The Syncthing Authors.
--
-- This Source Code Form is subject to the terms of the Mozilla Public
-- License, v. 2.0. If a copy of the MPL was not distributed with this file,
-- You can obtain one at https://mozilla.org/MPL/2.0/.

CREATE TABLE IF NOT EXISTS cleanup_policies (
    id INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    max_age_days INTEGER NOT NULL DEFAULT 30,
    max_size_bytes INTEGER NOT NULL DEFAULT 0,
    pattern TEXT NOT NULL DEFAULT '' COLLATE BINARY,
    enabled INTEGER NOT NULL DEFAULT 1,
    UNIQUE(user_id, pattern)
) STRICT;

CREATE INDEX IF NOT EXISTS cleanup_policies_user ON cleanup_policies (user_id);
