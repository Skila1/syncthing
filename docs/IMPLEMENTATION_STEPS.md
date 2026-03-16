# Implementation Steps

Step-by-step implementation roadmap for the multi-user Syncthing platform.
Each step is a self-contained unit of work that can be completed, tested, and deployed before moving to the next.

Reference: [REQUIREMENTS.md](REQUIREMENTS.md) for full feature specifications.

---

## Step 1: Multi-User Foundation & Separate Folders

**Status:** COMPLETE
**Est. time:** 4-6 hours
**Depends on:** Nothing (this is the foundation)

The core transformation: replace single-user auth with a multi-user system where each user gets their own isolated storage root.

- [x] Database migration system -- added `users` and `user_sessions` tables via existing SQLite schema/migration pipeline (`internal/db/sqlite/sql/schema/main/10-users.sql`, migration `06-add-users.sql`, bumped schema version to 6)
- [x] `users` table: id, username, email, password_hash, role (admin/user), root_path, created_at, updated_at, status
- [x] `user_sessions` table: token, user_id, created_at, expires_at, ip_address
- [x] User CRUD package (`lib/users/users.go`) -- Manager with create, read, update, delete, password hashing (bcrypt), session cache
- [x] SQLite store implementation (`internal/db/sqlite/db_users.go`) -- full Store interface backed by main database
- [x] Auth overhaul (`lib/api/api_multiauth.go`) -- new multi-user auth middleware; legacy single-user auth preserved as fallback
- [x] Session management -- user-associated sessions with in-memory cache, DB persistence, expiry, limits
- [x] Role-based API middleware (`lib/api/api_usercontext.go`) -- `requireAdmin()` guard, `userFromRequest()` context injection
- [x] Per-user root folders -- `Manager.CreateUser()` creates `<userDataDir>/<username>/` directory
- [x] Initial admin account creation on first run (via `ST_ADMIN_USER` / `ST_ADMIN_PASSWORD` env vars)
- [x] GUI: multi-user login form -- existing login form works; metadata.js now returns username/role/userId
- [x] GUI: admin user management page (`syncthing/admin/userManagementModalView.html`) -- user list, create/edit/delete, role toggle, password change
- [x] GUI: conditional rendering -- admin-only actions hidden from regular users, user badge in navbar
- [x] Docker: update `docker-entrypoint.sh` for per-user directory creation
- [x] Docker: add `ST_ADMIN_USER`, `ST_ADMIN_PASSWORD`, `ST_USER_DATA_DIR` env vars
- [x] User management REST API endpoints: `GET/POST /rest/users`, `GET/PUT/DELETE /rest/users/:id`, `POST /rest/users/:id/password`, `GET /rest/system/user`
- [x] Threaded `*sqlite.DB` from `OpenDatabase()` through `syncthing.New()` and `api.New()` to enable UserStore creation

---

## Step 2: User Quotas & Disk Usage Dashboard

**Status:** COMPLETE
**Est. time:** 3-4 hours
**Depends on:** Step 1

Give admins control over how much storage each user can consume.

- [x] Add `quota_bytes` and `used_bytes` columns to `users` table
- [x] Quota enforcement in sync engine -- reject incoming blocks that would exceed quota
- [x] Quota enforcement on file uploads
- [x] Real-time disk usage calculation per user (scan root folder)
- [x] Warning thresholds (80%, 95%) with in-app alerts
- [x] Admin GUI: set/edit quota per user
- [x] User GUI: storage usage dashboard (used / total bar, per-folder breakdown)
- [x] API endpoints: `GET /rest/storage/usage`, admin quota management on `PUT /rest/users/:id`

---

## Step 3: Password Reset & MFA/2FA

**Status:** COMPLETE
**Est. time:** 3-4 hours
**Depends on:** Step 1

Harden authentication with password recovery and two-factor authentication.

- [x] `password_resets` table: id, user_id, token, expires_at, used
- [x] `mfa_recovery` table: id, user_id, code_hash, used
- [x] Admin-initiated password reset (generate reset token)
- [x] Self-service password reset via email (SMTP configuration)
- [x] TOTP-based MFA (`lib/mfa/`) -- enroll, verify, recovery codes
- [x] MFA login flow: password -> TOTP code prompt -> session
- [x] "Remember this device" cookie (30 days)
- [x] Admin can disable MFA for a user (account recovery)
- [x] GUI: MFA enrollment (QR code, confirm code, show recovery codes)
- [x] GUI: password change form, reset token entry
- [x] Docker: add `ST_SMTP_*` env vars

---

## Step 4: Access Control & Folder Permissions

**Status:** COMPLETE
**Est. time:** 4-5 hours
**Depends on:** Step 1

Per-folder permission system: private, shared with specific users, read-only.

- [x] `folder_permissions` table: folder_id, user_id, permission (private/read/readwrite)
- [x] `device_ownership` table: device_id, user_id, approved, created_at
- [x] Permission enforcement at API layer -- check access before any folder operation
- [x] Permission enforcement at sync engine layer -- block unauthorized writes
- [x] Device ownership -- link each device to a user, approval workflow for new devices
- [x] GUI: folder permissions dialog (share with users, set access level)
- [x] GUI: device ownership display in device edit modal
- [x] API endpoints: `GET/PUT /rest/folders/:id/permissions`

---

## Step 5: Share Links

**Status:** COMPLETE
**Est. time:** 2-3 hours
**Depends on:** Step 4

Temporary download links for files and folders.

- [x] `share_links` table: id, token, user_id, folder_id, file_path, expires_at, max_downloads, download_count, password_hash
- [x] Share link generation (`lib/sharing/`) -- token creation, expiry, optional password
- [x] Public download endpoint: `GET /rest/noauth/share/:token` (no auth required)
- [x] Admin can view and revoke all active share links
- [x] GUI: share link dialog (generate link, set expiry/password/download limit, copy link)
- [x] GUI: admin share link management view

---

## Step 6: File Browser, Uploads & Search

**Status:** COMPLETE
**Est. time:** 5-6 hours
**Depends on:** Step 4

Web-based file management: browse, upload, download, search.

- [x] File browser backend (`lib/filebrowser/`) -- list directories, file metadata, previews
- [x] File upload endpoint with chunked upload support (`POST /rest/files/upload`)
- [x] File download endpoint (`GET /rest/files/download`)
- [x] File search endpoint (`GET /rest/files/search`) -- filename search, optional content indexing
- [x] GUI: file browser view (tree navigation via FancyTree, file list, metadata panel)
- [x] GUI: drag-and-drop upload with progress bar
- [x] GUI: search bar with results list
- [x] Respect quotas and folder permissions on all operations

---

## Step 7: Trash, Version History & Cleanup Policies

**Status:** COMPLETE
**Est. time:** 3-4 hours
**Depends on:** Step 1, Step 6

Soft deletes, file versioning, and automatic cleanup.

- [x] Per-user trash folder (`.sttrash/` inside folder root)
- [x] Move-to-trash on delete instead of permanent removal
- [x] Trash retention period (configurable, default 30 days)
- [x] Per-file version history (`.stversions/` directory, max 20 versions per file)
- [x] `cleanup_policies` table: id, user_id, max_age_days, max_size_bytes, pattern, enabled
- [x] Cleanup scheduler (background goroutine, 6-hour interval)
- [x] GUI: trash browser (list, restore, permanent delete, empty trash)
- [x] GUI: file version history dialog (timeline, restore, download)
- [x] GUI: cleanup policy editor (add/delete policies per user)
- [x] API endpoints: `GET /rest/trash`, `POST /rest/trash/restore`, `DELETE /rest/trash/item`, `POST /rest/trash/empty`, `GET /rest/versions`, `POST /rest/versions/restore`, `GET/POST /rest/cleanup-policies`

---

## Step 8: Activity Feed & Notifications

**Status:** Not started
**Est. time:** 3-4 hours
**Depends on:** Step 1

User-scoped activity tracking and change notifications.

- [ ] `notifications` table: id, user_id, type, message, read, created_at
- [ ] Extend Syncthing event system (`lib/events/`) with user-scoped event types
- [ ] Activity feed backend -- record file create/modify/delete/share/sync events per user
- [ ] Notification dispatch (`lib/notifications/`) -- in-app and optional email (SMTP)
- [ ] Notification preferences per user per folder (all changes, deletions only, new files only)
- [ ] GUI: activity feed view (filterable by folder, action, date range)
- [ ] GUI: notifications panel (bell icon, list, mark read)
- [ ] API endpoints: `GET /rest/activity`, `GET /rest/notifications`, `PUT /rest/notifications/:id/read`

---

## Step 9: Sync Enhancements

**Status:** Not started
**Est. time:** 5-6 hours
**Depends on:** Step 1, Step 4

Selective sync, scheduling, bandwidth limits, and conflict resolution.

- [ ] Selective sync per device -- per-device folder inclusion/exclusion (extend `FolderDeviceConfiguration`)
- [ ] Pause/resume from dashboard -- per-folder, per-device, and global controls
- [ ] `user_bandwidth` table: user_id, max_upload_kbps, max_download_kbps
- [ ] Per-user bandwidth limits enforced at connection layer (`lib/connections/`)
- [ ] Admin-set ceiling on per-user limits
- [ ] `sync_schedules` table: id, user_id, folder_id, start_time, end_time, days_of_week
- [ ] Sync scheduler (`lib/scheduler/`) -- pause/resume sync based on time windows
- [ ] Conflict resolution UI -- surface conflicts in dashboard, side-by-side text comparison
- [ ] Conflict actions: keep local, keep remote, keep both, merge
- [ ] GUI: selective sync checkboxes in device/folder edit modals
- [ ] GUI: bandwidth limit controls (admin + user)
- [ ] GUI: sync schedule time-window picker
- [ ] GUI: conflict resolution dialog

---

## Step 10: Security Hardening

**Status:** Not started
**Est. time:** 4-5 hours
**Depends on:** Step 1, Step 3, Step 4

Encryption, IP restrictions, and comprehensive audit logging.

- [ ] Per-user encryption keys (`lib/encryption/`) -- key derivation, at-rest encryption per user folder
- [ ] Key escrow option for admin recovery
- [ ] `ip_restrictions` table: id, user_id, cidr, action (allow/deny)
- [ ] IP restriction middleware in `lib/api/` -- enforce per-user and global allow/deny lists
- [ ] `audit_log` table: id, user_id, action, target_type, target_id, ip_address, details_json, created_at
- [ ] Audit logging (`lib/audit/`) -- record all security-relevant events (login, logout, password change, permission change, share, admin actions)
- [ ] Admin activity log -- separate immutable (append-only) trail for admin actions
- [ ] GUI: audit log viewer with filtering and export (CSV/JSON)
- [ ] GUI: IP restriction management (admin)
- [ ] API endpoints: `GET /rest/audit`, `GET/POST /rest/ip-restrictions`

---

## Step 11: Collaboration Features

**Status:** Not started
**Est. time:** 4-5 hours
**Depends on:** Step 4, Step 8

Shared folders, groups, comments, and change notifications.

- [ ] Shared folders between users -- share any folder with read-only or read-write access
- [ ] Shared folder sync -- handle multi-user writes with conflict detection
- [ ] `groups` table: id, name, created_at
- [ ] `group_members` table: group_id, user_id
- [ ] Group folders -- share a folder with a group, all members get access
- [ ] `comments` table: id, user_id, folder_id, file_path, content, created_at
- [ ] File/folder comments (`lib/comments/`) -- attach text notes to any accessible file
- [ ] Change notifications for shared folders -- notify when others modify shared files
- [ ] GUI: shared folder indicator in folder list
- [ ] GUI: group management (admin -- create/edit groups, manage membership)
- [ ] GUI: comment thread view in file browser
- [ ] GUI: notification preferences per shared folder
- [ ] API endpoints: `GET/POST/DELETE /rest/groups`, `GET/POST/DELETE /rest/comments`

---

## Step 12: Admin Platform

**Status:** Not started
**Est. time:** 4-5 hours
**Depends on:** Step 1, Step 2, Step 4, Step 9

Full admin dashboard with provisioning, analytics, and system monitoring.

- [ ] User provisioning -- bulk create via CSV import, invite links
- [ ] Account lifecycle management -- active, suspended, deleted states with data retention
- [ ] Storage analytics dashboard -- total usage, per-user breakdown, growth trends, largest files
- [ ] Exportable reports (CSV/JSON)
- [ ] Admin device management -- view all devices across all users, force-disconnect, remove, reassign
- [ ] Global bandwidth control -- instance-wide upload/download limits
- [ ] Real-time bandwidth usage display
- [ ] System health monitoring -- CPU, memory, disk I/O, network, active connections, sync queue
- [ ] Extended health check endpoint (builds on existing `/rest/noauth/health`)
- [ ] Alerting -- configurable thresholds trigger admin notifications
- [ ] GUI: admin dashboard home with health overview
- [ ] GUI: storage analytics charts
- [ ] GUI: device management table (cross-user)
- [ ] GUI: bandwidth control panel
- [ ] GUI: system health charts
- [ ] API endpoints: `GET /rest/admin/analytics`, `GET /rest/admin/health`, `GET /rest/admin/devices`, `PUT /rest/admin/bandwidth`

---

## Step 13: Storage Pools (Multi-Disk)

**Status:** Not started
**Est. time:** 3-4 hours
**Depends on:** Step 2

Multiple disk support for Proxmox, ZFS, RAID, and LVM environments.

- [ ] `storage_pools` table: id, name, path, total_bytes, used_bytes, status, strategy
- [ ] Storage pool management (`lib/storage/`) -- add/remove pools, health monitoring
- [ ] Pool selection strategy: per-user assignment, round-robin, or fill-first
- [ ] Pool health detection -- offline/degraded alerts
- [ ] Docker: multiple volume mount points for pools
- [ ] Docker: `ST_STORAGE_POOLS` env var for comma-separated pool paths
- [ ] GUI: admin storage pool management (add/remove pools, view health)
- [ ] GUI: per-user pool assignment in user edit
- [ ] API endpoints: `GET/POST/DELETE /rest/storage/pools`

---

## Step 14: Backup & Deduplication

**Status:** Not started
**Est. time:** 5-6 hours
**Depends on:** Step 1, Step 4, Step 13

Device backups, snapshots, and duplicate detection.

- [ ] `backups` table: id, user_id, device_id, type (full/incremental), status, started_at, completed_at, size_bytes
- [ ] Automatic device backups (`lib/backup/`) -- scheduled full/incremental backups to server
- [ ] Backup history with restore points
- [ ] `snapshots` table: id, user_id, folder_id, created_at, metadata_json
- [ ] Snapshot backups -- point-in-time folder tree snapshots (leverage ZFS snapshots where available)
- [ ] Manual and scheduled snapshot creation
- [ ] Duplicate detection (`lib/dedup/`) -- background worker scanning by content hash
- [ ] Duplicate report -- show potential space savings
- [ ] One-click deduplication -- replace duplicates with hardlinks/symlinks
- [ ] GUI: backup management (list, create, restore)
- [ ] GUI: snapshot management (list, create, restore)
- [ ] GUI: duplicate report with resolve actions
- [ ] API endpoints: `GET/POST /rest/backups`, `GET/POST /rest/snapshots`, `GET /rest/duplicates`, `POST /rest/duplicates/resolve`

---

## Step 15: Docker & Deployment Finalization

**Status:** Not started
**Est. time:** 2-3 hours
**Depends on:** All previous steps

Ensure everything works cleanly in the Docker deployment model.

- [ ] Update `Dockerfile` -- additional volume mount points, migration step on startup
- [ ] Update `docker-entrypoint.sh` -- per-user directory creation, multi-pool volume ownership
- [ ] Update `README-Docker.md` -- new compose example with all env vars and volumes
- [ ] Verify CI/CD pipeline (`.github/workflows/build-syncthing.yaml`) builds correctly
- [ ] Full end-to-end test: build image, run container, create users, sync files, verify isolation
- [ ] Document all new environment variables

---

## Summary

| Step | Feature Group                       | Est. Time  | Depends On         |
|------|-------------------------------------|------------|--------------------|
| 1    | Multi-User Foundation & Folders     | 4-6 hours  | --                 |
| 2    | User Quotas & Disk Usage            | 3-4 hours  | Step 1             |
| 3    | Password Reset & MFA/2FA            | 3-4 hours  | Step 1             |
| 4    | Access Control & Folder Permissions | 4-5 hours  | Step 1             |
| 5    | Share Links                         | 2-3 hours  | Step 4             |
| 6    | File Browser, Uploads & Search      | 5-6 hours  | Step 4             |
| 7    | Trash, Versions & Cleanup           | 3-4 hours  | Step 1, 6          |
| 8    | Activity Feed & Notifications       | 3-4 hours  | Step 1             |
| 9    | Sync Enhancements                   | 5-6 hours  | Step 1, 4          |
| 10   | Security Hardening                  | 4-5 hours  | Step 1, 3, 4       |
| 11   | Collaboration Features              | 4-5 hours  | Step 4, 8          |
| 12   | Admin Platform                      | 4-5 hours  | Step 1, 2, 4, 9    |
| 13   | Storage Pools (Multi-Disk)          | 3-4 hours  | Step 2             |
| 14   | Backup & Deduplication              | 5-6 hours  | Step 1, 4, 13      |
| 15   | Docker & Deployment Finalization    | 2-3 hours  | All                |
|      | **Total**                           | **55-70 hours** |                |

---

*Document generated: 2026-03-16*
