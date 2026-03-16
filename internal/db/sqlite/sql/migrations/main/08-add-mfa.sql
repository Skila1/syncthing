-- Copyright (C) 2026 The Syncthing Authors.
--
-- This Source Code Form is subject to the terms of the Mozilla Public
-- License, v. 2.0. If a copy of the MPL was not distributed with this file,
-- You can obtain one at https://mozilla.org/MPL/2.0/.

ALTER TABLE users ADD COLUMN mfa_enabled INTEGER NOT NULL DEFAULT 0;
ALTER TABLE users ADD COLUMN mfa_secret TEXT NOT NULL DEFAULT '' COLLATE BINARY;

CREATE TABLE IF NOT EXISTS password_resets (
    id INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token TEXT NOT NULL UNIQUE COLLATE BINARY,
    expires_at INTEGER NOT NULL,
    used INTEGER NOT NULL DEFAULT 0
) STRICT;

CREATE INDEX IF NOT EXISTS password_resets_token ON password_resets (token);
CREATE INDEX IF NOT EXISTS password_resets_user_id ON password_resets (user_id);

CREATE TABLE IF NOT EXISTS mfa_recovery (
    id INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    code_hash TEXT NOT NULL COLLATE BINARY,
    used INTEGER NOT NULL DEFAULT 0
) STRICT;

CREATE INDEX IF NOT EXISTS mfa_recovery_user_id ON mfa_recovery (user_id);
