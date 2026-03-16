[![Syncthing][14]][15]

---

> **Skila1 Fork** -- This fork is focused exclusively on delivering a multi-user file synchronization platform as a **Docker-only deployment**. We are not targeting or supporting native installs across multiple operating systems. The server runs in a container; clients connect to it. That's it.

---

## What Skila1 Has Changed

This fork transforms Syncthing from a single-user peer-to-peer sync tool into a multi-user, admin-managed file platform. Below is the full roadmap. Checked items are complete.

### Step 1: Multi-User Foundation & Separate Folders -- COMPLETE

Multi-user authentication, per-user isolated storage roots, role-based access (admin/user), user management GUI, session management, and REST API for user CRUD.

- [x] Multi-user database (users + sessions tables in SQLite)
- [x] User CRUD with bcrypt password hashing
- [x] Multi-user auth middleware (session cookies, Basic auth, API key fallback)
- [x] Role-based API guards (admin-only routes)
- [x] Per-user root folder creation on signup
- [x] Admin user management panel in GUI
- [x] Conditional rendering (admin vs regular user)
- [x] Initial admin account via `ST_ADMIN_USER` / `ST_ADMIN_PASSWORD` env vars
- [x] Docker entrypoint updates for per-user directories

### Step 2: User Quotas & Disk Usage Dashboard

Admin-controlled storage quotas per user with real-time usage tracking.

- [x] Quota enforcement in sync engine and uploads
- [x] Per-user disk usage calculation
- [x] Warning thresholds (80%, 95%)
- [x] Admin + user storage dashboards

### Step 3: Password Reset & MFA/2FA

Password recovery and TOTP-based two-factor authentication.

- [x] Admin-initiated and self-service password reset
- [x] TOTP MFA enrollment, verification, and recovery codes
- [x] "Remember this device" support

### Step 4: Access Control & Folder Permissions

Per-folder permissions: private, shared with specific users, read-only.

- [x] Folder permission system (private / read / read-write per user)
- [x] Device ownership linked to users
- [x] Permission enforcement at API and sync engine layers

### Step 5: Share Links

Temporary, optionally password-protected download links.

- [x] Expiring share links with download limits
- [x] Public download endpoint (no auth)
- [x] Admin link management

### Step 6: File Browser, Uploads & Search

Web-based file management directly in the GUI.

- [x] Directory browsing, file metadata, previews
- [x] Chunked file upload with drag-and-drop
- [x] Filename search

### Step 7: Trash, Version History & Cleanup Policies

Soft deletes, file versioning, and automatic cleanup.

- [x] Per-user trash folder with configurable retention
- [x] File version history with restore
- [x] Cleanup policy scheduler

### Step 8: Activity Feed & Notifications

User-scoped activity tracking and change alerts.

- [x] Activity feed (file changes, shares, syncs)
- [x] In-app and optional email notifications
- [x] Per-folder notification preferences

### Step 9: Sync Enhancements

Selective sync, scheduling, bandwidth limits, and conflict resolution.

- [ ] Per-device selective sync
- [ ] Per-user bandwidth limits
- [ ] Time-window sync scheduling
- [ ] Conflict resolution UI (keep local, keep remote, merge)

### Step 10: Security Hardening

Encryption, IP restrictions, and audit logging.

- [ ] Per-user at-rest encryption
- [ ] IP allow/deny lists per user
- [ ] Immutable audit log for all security events

### Step 11: Collaboration Features

Shared folders, groups, comments, and change notifications.

- [ ] Shared folders between users with multi-writer conflict detection
- [ ] Group-based folder sharing
- [ ] File/folder comments

### Step 12: Admin Platform

Full admin dashboard with provisioning, analytics, and monitoring.

- [ ] Bulk user provisioning (CSV import, invite links)
- [ ] Storage analytics and exportable reports
- [ ] Cross-user device management
- [ ] System health monitoring and alerting

### Step 13: Storage Pools (Multi-Disk)

Multiple disk support for ZFS, RAID, and LVM environments.

- [ ] Storage pool management (add/remove, health monitoring)
- [ ] Per-user pool assignment strategies
- [ ] Docker multi-volume support

### Step 14: Backup & Deduplication

Device backups, snapshots, and duplicate detection.

- [ ] Scheduled full/incremental device backups
- [ ] Point-in-time folder snapshots
- [ ] Content-hash based duplicate detection and resolution

### Step 15: Docker & Deployment Finalization

Final integration testing and documentation.

- [ ] Updated Dockerfile with all volume mounts and migration steps
- [ ] Full end-to-end test suite
- [ ] Complete environment variable documentation

### Roadmap Summary

| Step | Feature | Status |
|------|---------|--------|
| 1 | Multi-User Foundation & Folders | **Complete** |
| 2 | User Quotas & Disk Usage | **Complete** |
| 3 | Password Reset & MFA/2FA | **Complete** |
| 4 | Access Control & Folder Permissions | **Complete** |
| 5 | Share Links | **Complete** |
| 6 | File Browser, Uploads & Search | **Complete** |
| 7 | Trash, Versions & Cleanup | **Complete** |
| 8 | Activity Feed & Notifications | **Complete** |
| 9 | Sync Enhancements | Planned |
| 10 | Security Hardening | Planned |
| 11 | Collaboration Features | Planned |
| 12 | Admin Platform | Planned |
| 13 | Storage Pools (Multi-Disk) | Planned |
| 14 | Backup & Deduplication | Planned |
| 15 | Docker & Deployment Finalization | Planned |

---

## About Syncthing

Syncthing is a **continuous file synchronization program**. It synchronizes
files between two or more computers. For more details, see the full [Goals document][13].

## Docker

To run Syncthing in Docker, see [the Docker README][16].

## Building

Building from source: `go run build.go` -- binaries are created in `./bin`. See the [build guide][5] for details.

## Documentation

Please see the Syncthing [documentation site][6] [[source]][17].

All code is licensed under the [MPLv2 License][7].

[1]: https://docs.syncthing.net/specs/bep-v1.html
[2]: https://docs.syncthing.net/intro/getting-started.html
[3]: https://github.com/syncthing/syncthing/blob/main/etc
[5]: https://docs.syncthing.net/dev/building.html
[6]: https://docs.syncthing.net/
[7]: https://github.com/syncthing/syncthing/blob/main/LICENSE
[8]: https://forum.syncthing.net/
[10]: https://github.com/syncthing/syncthing/issues
[11]: https://docs.syncthing.net/users/contrib.html#gui-wrappers
[13]: https://github.com/syncthing/syncthing/blob/main/GOALS.md
[14]: assets/logo-text-128.png
[15]: https://syncthing.net/
[16]: https://github.com/syncthing/syncthing/blob/main/README-Docker.md
[17]: https://github.com/syncthing/docs
