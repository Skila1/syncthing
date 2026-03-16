-- Copyright (C) 2026 The Syncthing Authors.
--
-- This Source Code Form is subject to the terms of the Mozilla Public
-- License, v. 2.0. If a copy of the MPL was not distributed with this file,
-- You can obtain one at https://mozilla.org/MPL/2.0/.

-- Migration: add multi-user support tables
-- These are created by the schema scripts (10-users.sql) via CREATE IF NOT EXISTS,
-- so this migration is a no-op for fresh installs. It exists to bump the schema
-- version and trigger re-application of schema scripts on existing databases.
SELECT 1
;
