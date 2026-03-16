# Multi-User Syncthing Platform -- Requirements & Implementation Plan

## Table of Contents

- [1. Project Overview](#1-project-overview)
- [2. Current Architecture Summary](#2-current-architecture-summary)
- [3. Feature Requirements](#3-feature-requirements)
  - [3.1 Multi-User System](#31-multi-user-system)
  - [3.2 Access Control](#32-access-control)
  - [3.3 Storage Management](#33-storage-management)
  - [3.4 Sync Enhancements](#34-sync-enhancements)
  - [3.5 Security Features](#35-security-features)
  - [3.6 Collaboration Features](#36-collaboration-features)
  - [3.7 Admin Platform](#37-admin-platform)
  - [3.8 Backup & Deduplication](#38-backup--deduplication)
- [4. Technical Architecture Impact](#4-technical-architecture-impact)
- [5. Effort Estimates](#5-effort-estimates)
- [6. Implementation Phases](#6-implementation-phases)
- [7. Risk Assessment](#7-risk-assessment)

---

## 1. Project Overview

This document specifies the requirements for transforming the Syncthing open-source file synchronization tool into a **multi-user file synchronization platform**. The goal is to support multiple users within a single instance, each with isolated storage, quota enforcement, access control, collaboration features, and a full administrative layer -- all deployable via Docker.

The scope of this transformation is comparable to building a Nextcloud-like overlay on top of Syncthing's proven Block Exchange Protocol (BEP) sync engine.

---

## 2. Current Architecture Summary

### Technology Stack

| Layer       | Technology                                          | Key Files                                    |
|-------------|-----------------------------------------------------|----------------------------------------------|
| Backend     | Go 1.25+                                            | `cmd/syncthing/main.go`, `build.go`          |
| Frontend    | AngularJS 1.x, Bootstrap, jQuery, FancyTree         | `gui/default/`                               |
| Protocol    | BEP (Block Exchange Protocol) over TLS/QUIC         | `lib/protocol/`, `proto/bep/bep.proto`       |
| Database    | SQLite (primary), LevelDB (legacy)                  | `internal/db/`                               |
| Config      | XML (`config.xml`)                                  | `lib/config/config.go`                       |
| API         | REST via `julienschmidt/httprouter`                 | `lib/api/api.go`                             |
| Docker      | Multi-stage (golang -> alpine), volume `/var/syncthing` | `Dockerfile`                             |
| CI/CD       | GitHub Actions, GHCR + Docker Hub via regsync       | `.github/workflows/`                         |

### Current Auth Model

Syncthing is a **single-user, peer-to-peer** sync tool. The existing auth system supports:

- One admin user with bcrypt password (`lib/config/guiconfiguration.go` -- `User`/`Password` fields)
- API key authentication (`APIKey` field, `X-API-Key` header)
- Optional LDAP backend (`AuthMode` = `AuthModeLDAP`)
- Session tokens stored in DB with 7-day max lifetime, 25 max active sessions (`lib/api/tokenmanager.go`)
- No role system, no multi-user isolation, no per-user folders

### Current Config Model

The `Configuration` struct (`lib/config/config.go`, line 94) contains:

```go
type Configuration struct {
    Version        int                   `xml:"version,attr"`
    Folders        []FolderConfiguration `xml:"folder"`
    Devices        []DeviceConfiguration `xml:"device"`
    GUI            GUIConfiguration      `xml:"gui"`
    LDAP           LDAPConfiguration     `xml:"ldap"`
    Options        OptionsConfiguration  `xml:"options"`
    IgnoredDevices []ObservedDevice      `xml:"remoteIgnoredDevice"`
    Defaults       Defaults              `xml:"defaults"`
}
```

There is no concept of user ownership on folders or devices. All folders and devices are global to the instance.

### Current Docker Deployment

The `Dockerfile` produces an Alpine-based image exposing ports 8384 (GUI), 22000 (sync), 21027 (discovery). A single volume at `/var/syncthing` holds both config (`/var/syncthing/config`) and sync data. The entrypoint script (`script/docker-entrypoint.sh`) runs as `PUID:PGID` via `su-exec`.

---

## 3. Feature Requirements

### 3.1 Multi-User System

#### 3.1.1 Multiple Users Within One Instance

- The platform must support creating and managing multiple user accounts within a single running instance.
- Each user authenticates independently with their own credentials.
- Users are isolated from one another by default; a user cannot see another user's files, devices, or configuration unless explicitly shared.

#### 3.1.2 Per-User Root Folders

- Each user is assigned a dedicated root storage directory on the filesystem (e.g., `/var/syncthing/users/<username>/`).
- The root folder is created automatically upon user provisioning.
- Users cannot navigate or access paths outside their root folder unless granted explicit permission.

#### 3.1.3 User Quotas

- Administrators can assign storage quotas per user (e.g., 10 GB, 100 GB).
- Quota tiers should be configurable (named presets such as "Free", "Standard", "Premium" mapping to byte limits).
- The system must enforce quotas: sync operations and uploads that would exceed the quota are rejected with a clear error.
- Quota usage must be tracked in real time and displayed in both the user and admin dashboards.

#### 3.1.4 Role System

- Two roles: **admin** and **user**.
- **Admin**: Full access to all platform settings, user management, device management, storage analytics, system health monitoring. Can impersonate users for support.
- **User**: Access to own files, devices, sync settings, and shared resources only. Cannot access any admin-level functionality.
- Role is stored per user account and enforced on every API endpoint and GUI route.

#### 3.1.5 User Management Panel

- Integrated into the existing web GUI but **conditionally rendered** -- admin-only components do not render for regular users.
- Admin panel sections: user list, create/edit/delete users, assign roles, set quotas, view usage, force password reset, enable/disable accounts.
- User panel sections: profile settings, change password, manage MFA, view own devices, view own storage usage.

#### 3.1.6 Password Reset

- Admin-initiated password reset: admin can force a password reset for any user.
- Self-service password reset: user can request a reset via email (requires SMTP configuration) or via a reset token generated by an admin.
- Reset tokens must be single-use and time-limited (configurable, default 24 hours).

#### 3.1.7 Multi-Factor Authentication (MFA)

- TOTP-based MFA (compatible with Google Authenticator, Authy, etc.).
- MFA enrollment flow: user enables MFA, scans QR code, confirms with a code.
- Backup/recovery codes generated at enrollment time.
- Admin can disable MFA for a user (for account recovery).

---

### 3.2 Access Control

#### 3.2.1 Folder Permissions

Each folder supports three permission levels:

| Level                      | Behavior                                                       |
|----------------------------|----------------------------------------------------------------|
| **Private**                | Only the owner can access the folder.                          |
| **Shared with specific users** | Owner grants access to named users with read-write or read-only. |
| **Read-only access**       | Recipient can view and download but not modify or delete.      |

- Permissions are stored per folder and enforced at the API and sync-engine level.
- A folder's permission set is editable by the owner and by admins.

#### 3.2.2 Device Ownership

- Every device is linked to exactly one user account.
- A user can only manage (pause, remove, configure) their own devices.
- Admins can view and manage all devices across all users.
- Device approval workflow: when a new device connects, it is pending until the owning user or an admin approves it.

#### 3.2.3 Share Links

- Users can generate temporary download links for individual files or folders.
- Links are token-based with configurable expiry (default 7 days) and optional download count limits.
- Links can be password-protected.
- Links are served via a dedicated `/rest/noauth/share/<token>` endpoint that does not require authentication.
- Admins can view and revoke all active share links system-wide.

---

### 3.3 Storage Management

#### 3.3.1 User Disk Usage Dashboard

- Each user sees a dashboard showing: total quota, used space, free space, breakdown by folder, recent growth trends.
- Visual bar/chart for at-a-glance status.

#### 3.3.2 Quota Enforcement

- Real-time enforcement: incoming sync blocks and uploads are checked against remaining quota before writing.
- Graceful handling: if a sync would exceed quota, the file is marked as "quota exceeded" in the sync state rather than silently failing.
- Warning thresholds (configurable, default 80% and 95%) trigger notifications.

#### 3.3.3 Automatic Cleanup Policies

- Per-user configurable policies: delete files older than N days, delete files larger than N MB, delete files matching a pattern.
- Global admin policies that apply to all users as defaults.
- Cleanup runs on a schedule (configurable cron-like interval) and can be triggered manually.

#### 3.3.4 Trash / File Version History

- Deleted files are moved to a per-user trash folder rather than being permanently removed.
- Trash retention period is configurable (default 30 days).
- File version history: configurable number of previous versions retained per file (leveraging and extending Syncthing's existing `VersioningConfiguration` in `lib/config/folderconfiguration.go`).
- Users can browse and restore previous versions or trashed files via the GUI.

#### 3.3.5 Storage Pools (Multiple Disks)

- Support defining multiple storage pool paths (e.g., `/mnt/pool1`, `/mnt/pool2`).
- Compatible with Proxmox ZFS datasets, hardware RAID arrays, and Linux LVM.
- Pool selection strategy: per-user assignment, round-robin, or fill-first (configurable).
- Pool health monitoring: detect offline/degraded pools and alert admins.
- Each pool is exposed as a Docker volume mount point for container deployments.

#### 3.3.6 File Browser in the UI

- Web-based file browser embedded in the user dashboard.
- Directory tree navigation, file previews (images, text, PDF), file metadata display (size, modified date, sync status).
- Built on top of the existing FancyTree dependency already present in the frontend.

#### 3.3.7 Drag-and-Drop Uploads

- Users can drag files from their desktop onto the file browser to upload directly via the web UI.
- Chunked upload support for large files with progress indication.
- Upload respects the user's quota and folder permissions.

#### 3.3.8 Activity Feed

- Per-user feed of recent changes: file created, modified, deleted, shared, synced.
- Filterable by folder, action type, and date range.
- Backed by the existing Syncthing event system (`lib/events/`), extended with user-scoped events.

#### 3.3.9 Search Across Files

- Full-text filename search across all of a user's folders.
- Optional content-based search for text files (indexing via a background worker).
- Search API endpoint: `GET /rest/search?q=<query>&folder=<optional>`.

---

### 3.4 Sync Enhancements

#### 3.4.1 Selective Sync Per Device

- Users can choose which folders sync to which of their devices.
- Per-device folder inclusion/exclusion list, manageable from the dashboard.
- Extends the existing `FolderDeviceConfiguration` (`lib/config/folderconfiguration.go`, line 44).

#### 3.4.2 Pause/Resume Sync from Dashboard

- Per-folder and per-device pause/resume controls in the user dashboard.
- Global pause/resume for all of a user's sync activity.
- Extends the existing `Paused` field on `FolderConfiguration` and the `POST /rest/system/pause` / `POST /rest/system/resume` endpoints.

#### 3.4.3 Bandwidth Limits Per User

- Admin can set per-user upload and download bandwidth limits (KB/s).
- Users can further restrict their own limits within the admin-set ceiling.
- Enforced at the connection layer (`lib/connections/`).

#### 3.4.4 Sync Scheduling

- Users can define sync windows (e.g., "sync only between 22:00 and 06:00").
- Per-folder or global schedule.
- Scheduled pauses/resumes managed by a background scheduler goroutine.

#### 3.4.5 Conflict Resolution UI

- When sync conflicts occur, they are surfaced in the user dashboard with a clear UI.
- Side-by-side comparison for text files.
- User can choose: keep local, keep remote, keep both (renamed), or merge (text files).
- Extends the existing `MaxConflicts` field on `FolderConfiguration`.

---

### 3.5 Security Features

#### 3.5.1 Per-User Encryption Keys

- Each user's data at rest can be encrypted with a unique key.
- Key derivation from user password or a separate passphrase.
- Extends the existing `EncryptionPassword` field on `FolderDeviceConfiguration` (`lib/config/folderconfiguration.go`, line 47) to be user-scoped.
- Key escrow option for admin recovery.

#### 3.5.2 IP Restrictions

- Admin can define allowed/denied IP ranges per user or globally.
- Enforced at the API middleware layer (new middleware in `lib/api/`).
- Failed attempts from restricted IPs are logged.

#### 3.5.3 Two-Factor Authentication (2FA)

- Same as MFA in section 3.1.7 (TOTP-based).
- Enforced at login: after password verification, user must provide a TOTP code.
- "Remember this device" option (30-day cookie) to reduce friction.

#### 3.5.4 Audit Logs

- All security-relevant events logged: login, logout, password change, MFA change, permission change, file share, admin actions.
- Structured log entries stored in the database with timestamp, user ID, action, target, IP address, and result.
- Log viewer in the admin panel with filtering and export (CSV/JSON).

#### 3.5.5 Admin Activity Logs

- Separate audit trail for admin actions: user creation/deletion, quota changes, role changes, device management, system configuration changes.
- Immutable (append-only) log table to prevent tampering.

---

### 3.6 Collaboration Features

#### 3.6.1 Shared Folders Between Users

- A user can share any of their folders with one or more other users on the same instance.
- Share permissions: read-only or read-write.
- Shared folders appear in the recipient's folder list with a "shared" indicator.
- Sync engine must handle multi-user writes with conflict detection.

#### 3.6.2 Group Folders

- Admins can create named groups (e.g., "Engineering", "Marketing").
- A folder can be shared with a group, granting access to all group members.
- Group membership is managed by admins; users can be in multiple groups.

#### 3.6.3 Comments / File Notes

- Users can attach text comments to any file or folder they have access to.
- Comments are stored in the database, not in the filesystem.
- Comment thread view in the file browser.

#### 3.6.4 Notifications When Files Change

- Users receive in-app notifications when files in their shared folders are modified by others.
- Notification preferences: in-app, email (requires SMTP), or both.
- Configurable per folder (e.g., notify on all changes, only deletions, only new files).

---

### 3.7 Admin Platform

#### 3.7.1 User Provisioning

- Admin can create users individually or in bulk (CSV import).
- User creation sets: username, email, password (or invite link), role, quota, group memberships.
- Account lifecycle: active, suspended, deleted (with data retention policy).

#### 3.7.2 Storage Analytics

- Admin dashboard showing: total storage used, per-user breakdown, growth trends over time, largest files/folders.
- Exportable reports (CSV/JSON).

#### 3.7.3 Device Management

- Admin can view all connected devices across all users.
- Device details: name, last seen, OS, Syncthing version, connected folders, sync status.
- Admin can force-disconnect, remove, or reassign devices.

#### 3.7.4 Global Bandwidth Control

- Admin can set instance-wide upload and download bandwidth limits.
- Per-user limits (section 3.4.3) are capped at the global limit.
- Real-time bandwidth usage display in admin dashboard.

#### 3.7.5 System Health Monitoring

- Dashboard showing: CPU usage, memory usage, disk I/O, network throughput, active connections, sync queue depth.
- Health check endpoint (extends existing `/rest/noauth/health`).
- Alerting: configurable thresholds that trigger admin notifications (in-app or email).

---

### 3.8 Backup & Deduplication

#### 3.8.1 Automatic Device Backups

- Devices can be configured to perform scheduled full or incremental backups to the server.
- Backup data stored in the user's storage pool, subject to quota.
- Backup history with restore points.

#### 3.8.2 Snapshot Backups

- Point-in-time snapshots of a user's entire folder tree.
- Snapshot metadata stored in the database; actual data leverages filesystem snapshots (ZFS) or copy-on-write where available.
- Manual and scheduled snapshot creation.

#### 3.8.3 System Duplicate Detection

- Background worker scans user files for duplicates (by content hash).
- Duplicate report in the user dashboard showing potential space savings.
- One-click deduplication: replace duplicates with hardlinks or symlinks (configurable).

---

### 3.9 GUI Settings for All Features

- Every feature listed above must have corresponding GUI controls accessible to the appropriate role (admin or user).
- Admin GUI sections are **not rendered** for regular users -- the frontend conditionally excludes admin-only routes and components based on the authenticated user's role.
- All settings must also be accessible via REST API for automation and scripting.

### 3.10 Docker Deployment Compatibility

- All features must work within the existing Docker deployment model.
- New environment variables for multi-user configuration (e.g., `ST_MULTI_USER=true`, `ST_STORAGE_POOLS=/mnt/pool1,/mnt/pool2`).
- Updated `Dockerfile` and `docker-entrypoint.sh` to support multiple storage pool volume mounts and user data isolation.
- Updated `docker-compose` example in `README-Docker.md`.
- CI/CD pipelines (`.github/workflows/build-syncthing.yaml`) must build and push the updated image.

---

## 4. Technical Architecture Impact

### 4.1 Database Schema Changes

The existing SQLite database must be extended with new tables. A migration system should be added for schema versioning.

**New tables:**

| Table              | Columns (key fields)                                                                                     |
|--------------------|----------------------------------------------------------------------------------------------------------|
| `users`            | `id`, `username`, `email`, `password_hash`, `role`, `quota_bytes`, `used_bytes`, `mfa_secret`, `mfa_enabled`, `created_at`, `updated_at`, `status` |
| `sessions`         | `id`, `user_id`, `token`, `created_at`, `expires_at`, `ip_address`, `user_agent`                         |
| `groups`           | `id`, `name`, `created_at`                                                                               |
| `group_members`    | `group_id`, `user_id`                                                                                    |
| `folder_permissions` | `folder_id`, `user_id`, `group_id`, `permission` (private/read/readwrite)                              |
| `share_links`      | `id`, `token`, `user_id`, `folder_id`, `file_path`, `expires_at`, `max_downloads`, `download_count`, `password_hash` |
| `comments`         | `id`, `user_id`, `folder_id`, `file_path`, `content`, `created_at`                                      |
| `audit_log`        | `id`, `user_id`, `action`, `target_type`, `target_id`, `ip_address`, `details_json`, `created_at`       |
| `notifications`    | `id`, `user_id`, `type`, `message`, `read`, `created_at`                                                 |
| `cleanup_policies` | `id`, `user_id`, `max_age_days`, `max_size_bytes`, `pattern`, `enabled`                                  |
| `storage_pools`    | `id`, `name`, `path`, `total_bytes`, `used_bytes`, `status`, `strategy`                                  |
| `device_ownership` | `device_id`, `user_id`, `approved`, `created_at`                                                         |
| `user_bandwidth`   | `user_id`, `max_upload_kbps`, `max_download_kbps`                                                        |
| `sync_schedules`   | `id`, `user_id`, `folder_id`, `start_time`, `end_time`, `days_of_week`                                  |
| `backups`          | `id`, `user_id`, `device_id`, `type`, `status`, `started_at`, `completed_at`, `size_bytes`               |
| `snapshots`        | `id`, `user_id`, `folder_id`, `created_at`, `metadata_json`                                              |
| `password_resets`  | `id`, `user_id`, `token`, `expires_at`, `used`                                                           |
| `mfa_recovery`     | `id`, `user_id`, `code_hash`, `used`                                                                     |
| `ip_restrictions`  | `id`, `user_id`, `cidr`, `action` (allow/deny)                                                           |

### 4.2 Backend Changes (Go)

#### New packages to create

| Package                  | Purpose                                                    |
|--------------------------|------------------------------------------------------------|
| `lib/users`              | User CRUD, password hashing, quota tracking                |
| `lib/roles`              | Role definitions, permission checking                      |
| `lib/mfa`                | TOTP generation, verification, recovery codes              |
| `lib/sharing`            | Share link generation, validation, download serving        |
| `lib/storage`            | Storage pool management, quota enforcement, cleanup        |
| `lib/audit`              | Audit log recording and querying                           |
| `lib/notifications`      | In-app and email notification dispatch                     |
| `lib/search`             | File name and content indexing/search                      |
| `lib/backup`             | Backup scheduling, execution, and restore                  |
| `lib/dedup`              | Duplicate detection by content hash                        |
| `lib/groups`             | Group CRUD and membership                                  |
| `lib/scheduler`          | Sync scheduling, cleanup scheduling                        |
| `lib/migration`          | Database schema migration runner                           |
| `lib/filebrowser`        | File listing, preview generation, upload handling          |
| `lib/comments`           | File/folder comment CRUD                                   |

#### Existing packages to modify

| Package             | Changes Required                                                                                      |
|---------------------|-------------------------------------------------------------------------------------------------------|
| `lib/api/`          | Add user-scoped middleware, new REST endpoints for all features, role-based route guards               |
| `lib/api/api_auth.go` | Replace single-user auth with multi-user auth; integrate MFA verification step                      |
| `lib/api/tokenmanager.go` | Associate sessions with user IDs; enforce per-user session limits                              |
| `lib/config/`       | Extend `Configuration` with multi-user settings; add storage pool config; add user-scoped folder ownership |
| `lib/config/guiconfiguration.go` | Remove single-user `User`/`Password`; add reference to user DB                        |
| `lib/config/folderconfiguration.go` | Add `OwnerUserID` field; extend `VersioningConfiguration` for trash                 |
| `lib/connections/`  | Add per-user bandwidth throttling at the connection level                                              |
| `lib/model/`        | Enforce folder access by user; handle multi-user conflict detection                                    |
| `lib/events/`       | Add user-scoped event types for activity feed and notifications                                        |
| `internal/db/`      | Add new table schemas, migration support, DAOs for all new tables                                      |
| `cmd/syncthing/`    | Add CLI flags for multi-user mode, initial admin setup                                                 |

#### New API Endpoints

| Method | Path                                     | Description                          | Auth      |
|--------|------------------------------------------|--------------------------------------|-----------|
| POST   | `/rest/users`                            | Create user                          | Admin     |
| GET    | `/rest/users`                            | List users                           | Admin     |
| GET    | `/rest/users/:id`                        | Get user details                     | Admin/Self|
| PUT    | `/rest/users/:id`                        | Update user                          | Admin/Self|
| DELETE | `/rest/users/:id`                        | Delete user                          | Admin     |
| POST   | `/rest/users/:id/reset-password`         | Initiate password reset              | Admin     |
| POST   | `/rest/users/:id/mfa/enable`             | Enable MFA                           | Self      |
| POST   | `/rest/users/:id/mfa/disable`            | Disable MFA                          | Admin/Self|
| POST   | `/rest/users/:id/mfa/verify`             | Verify MFA code at login             | Self      |
| GET    | `/rest/groups`                           | List groups                          | Admin     |
| POST   | `/rest/groups`                           | Create group                         | Admin     |
| PUT    | `/rest/groups/:id`                       | Update group                         | Admin     |
| DELETE | `/rest/groups/:id`                       | Delete group                         | Admin     |
| POST   | `/rest/groups/:id/members`               | Add member                           | Admin     |
| DELETE | `/rest/groups/:id/members/:uid`          | Remove member                        | Admin     |
| GET    | `/rest/folders/:id/permissions`          | Get folder permissions               | Owner/Admin|
| PUT    | `/rest/folders/:id/permissions`          | Set folder permissions               | Owner/Admin|
| POST   | `/rest/share-links`                      | Create share link                    | User      |
| GET    | `/rest/share-links`                      | List own share links                 | User      |
| DELETE | `/rest/share-links/:id`                  | Revoke share link                    | User/Admin|
| GET    | `/rest/noauth/share/:token`              | Download shared file (no auth)       | Public    |
| GET    | `/rest/storage/usage`                    | Get own storage usage                | User      |
| GET    | `/rest/storage/pools`                    | List storage pools                   | Admin     |
| POST   | `/rest/storage/pools`                    | Add storage pool                     | Admin     |
| DELETE | `/rest/storage/pools/:id`                | Remove storage pool                  | Admin     |
| GET    | `/rest/files/browse`                     | Browse files in a folder             | User      |
| POST   | `/rest/files/upload`                     | Upload file                          | User      |
| GET    | `/rest/files/download`                   | Download file                        | User      |
| GET    | `/rest/files/search`                     | Search files                         | User      |
| GET    | `/rest/files/:id/versions`               | Get file version history             | User      |
| POST   | `/rest/files/:id/restore`                | Restore file version                 | User      |
| GET    | `/rest/trash`                            | List trashed files                   | User      |
| POST   | `/rest/trash/:id/restore`                | Restore from trash                   | User      |
| DELETE | `/rest/trash/:id`                        | Permanently delete from trash        | User      |
| GET    | `/rest/activity`                         | Get activity feed                    | User      |
| GET    | `/rest/comments`                         | Get comments for a file              | User      |
| POST   | `/rest/comments`                         | Add comment                          | User      |
| DELETE | `/rest/comments/:id`                     | Delete comment                       | User/Admin|
| GET    | `/rest/notifications`                    | Get notifications                    | User      |
| PUT    | `/rest/notifications/:id/read`           | Mark notification read               | User      |
| GET    | `/rest/audit`                            | Query audit log                      | Admin     |
| GET    | `/rest/admin/analytics`                  | Storage analytics                    | Admin     |
| GET    | `/rest/admin/health`                     | System health metrics                | Admin     |
| GET    | `/rest/admin/devices`                    | All devices across users             | Admin     |
| PUT    | `/rest/admin/bandwidth`                  | Set global bandwidth limits          | Admin     |
| GET    | `/rest/sync/schedule`                    | Get sync schedules                   | User      |
| POST   | `/rest/sync/schedule`                    | Set sync schedule                    | User      |
| GET    | `/rest/backups`                          | List backups                         | User      |
| POST   | `/rest/backups`                          | Create backup                        | User      |
| POST   | `/rest/backups/:id/restore`              | Restore backup                       | User      |
| GET    | `/rest/snapshots`                        | List snapshots                       | User      |
| POST   | `/rest/snapshots`                        | Create snapshot                      | User      |
| GET    | `/rest/duplicates`                       | Get duplicate report                 | User      |
| POST   | `/rest/duplicates/resolve`               | Deduplicate files                    | User      |
| GET    | `/rest/cleanup-policies`                 | Get cleanup policies                 | User/Admin|
| POST   | `/rest/cleanup-policies`                 | Create/update cleanup policy         | User/Admin|
| GET    | `/rest/ip-restrictions`                  | Get IP restrictions                  | Admin     |
| POST   | `/rest/ip-restrictions`                  | Set IP restrictions                  | Admin     |

### 4.3 Frontend Changes (AngularJS)

#### New views/components to create

| Component                        | Location (under `gui/default/syncthing/`)      | Purpose                                   |
|----------------------------------|------------------------------------------------|-------------------------------------------|
| User login (multi-user)          | `auth/loginView.html`                          | Multi-user login form with MFA step       |
| User dashboard                   | `user/dashboardView.html`                      | Per-user home: storage usage, activity    |
| File browser                     | `filebrowser/fileBrowserView.html`             | Tree + list view, previews, drag-drop     |
| User settings                    | `user/settingsView.html`                       | Profile, password, MFA, notifications     |
| Admin: user management           | `admin/usersView.html`                         | User CRUD, quotas, roles                  |
| Admin: group management          | `admin/groupsView.html`                        | Group CRUD, membership                    |
| Admin: storage analytics         | `admin/storageView.html`                       | Charts, pool management                   |
| Admin: device management         | `admin/devicesView.html`                       | Cross-user device list                    |
| Admin: audit logs                | `admin/auditView.html`                         | Log viewer with filters                   |
| Admin: system health             | `admin/healthView.html`                        | CPU, memory, disk, network charts         |
| Admin: bandwidth control         | `admin/bandwidthView.html`                     | Global + per-user limits                  |
| Folder permissions dialog        | `folder/permissionsModalView.html`             | Share with users/groups, set access level |
| Share link dialog                | `sharing/shareLinkModalView.html`              | Generate/manage share links               |
| Conflict resolution dialog       | `sync/conflictResolutionView.html`             | Side-by-side conflict UI                  |
| Sync schedule dialog             | `sync/scheduleModalView.html`                  | Time-window picker                        |
| Trash browser                    | `trash/trashView.html`                         | Browse and restore trashed files          |
| Version history dialog           | `versions/versionHistoryView.html`             | File version timeline, restore            |
| Comments panel                   | `comments/commentsView.html`                   | Threaded comments on files                |
| Notifications panel              | `notifications/notificationsView.html`         | Notification list, mark read              |
| Backup management                | `backup/backupView.html`                       | Backup list, create, restore              |
| Duplicate report                 | `dedup/dedupView.html`                         | Duplicate files list, resolve             |
| Cleanup policy editor            | `cleanup/cleanupPolicyView.html`               | Policy CRUD                               |

#### Existing views to modify

| File                                                          | Changes                                                          |
|---------------------------------------------------------------|------------------------------------------------------------------|
| `gui/default/index.html`                                     | Add role-based conditional rendering, new nav items, user menu   |
| `gui/default/syncthing/core/syncthingController.js`          | Add user context, role checks, new API calls                     |
| `gui/default/syncthing/settings/settingsModalView.html`      | Add user-level settings, remove single-user auth fields          |
| `gui/default/syncthing/folder/editFolderModalView.html`      | Add permission controls, owner display                           |
| `gui/default/syncthing/device/editDeviceModalView.html`      | Add device ownership display, user assignment                    |
| `gui/default/syncthing/core/eventService.js`                 | Subscribe to user-scoped events                                  |
| `gui/default/syncthing/app.js`                               | Add new routes, lazy-load admin modules                          |

### 4.4 Docker / Deployment Changes

| File                              | Changes                                                                    |
|-----------------------------------|----------------------------------------------------------------------------|
| `Dockerfile`                      | Add additional volume mount points for storage pools; add migration step   |
| `script/docker-entrypoint.sh`     | Create per-user directories; handle multiple volume ownership              |
| `README-Docker.md`                | Updated compose example with multi-user env vars, multiple volumes         |
| `.github/workflows/build-syncthing.yaml` | No structural changes needed; build continues to produce single binary |

**New environment variables:**

| Variable                   | Purpose                                        | Default          |
|----------------------------|------------------------------------------------|------------------|
| `ST_MULTI_USER`            | Enable multi-user mode                         | `false`          |
| `ST_ADMIN_USER`            | Initial admin username                         | `admin`          |
| `ST_ADMIN_PASSWORD`        | Initial admin password (first run only)        | (required)       |
| `ST_STORAGE_POOLS`         | Comma-separated storage pool paths             | `/var/syncthing`  |
| `ST_USER_DATA_DIR`         | Base path for per-user root folders            | `/var/syncthing/users` |
| `ST_SMTP_HOST`             | SMTP server for email notifications/resets     | (optional)       |
| `ST_SMTP_PORT`             | SMTP port                                      | `587`            |
| `ST_SMTP_USER`             | SMTP username                                  | (optional)       |
| `ST_SMTP_PASSWORD`         | SMTP password                                  | (optional)       |
| `ST_SMTP_FROM`             | From address for emails                        | (optional)       |

---

## 5. Effort Estimates

Estimates assume **1 senior full-stack developer** experienced in Go and frontend development, working full-time. Includes design, implementation, unit testing, and integration testing. Does not include extensive QA, documentation, or community review cycles.

### 5.1 Per-Category Breakdown

| #   | Category                    | Complexity | Dev-Weeks | Dependencies                              |
|-----|-----------------------------|------------|-----------|-------------------------------------------|
| 1   | Database & Migration System | High       | 3         | None                                      |
| 2   | Multi-User System           | Very High  | 6         | #1 (Database)                             |
| 3   | Role System & Auth Overhaul | Very High  | 4         | #1, #2                                    |
| 4   | User Management Panel (GUI) | High       | 4         | #2, #3                                    |
| 5   | Password Reset & MFA        | High       | 3         | #2, #3                                    |
| 6   | Access Control (Permissions) | Very High | 5         | #2, #3                                    |
| 7   | Device Ownership            | Medium     | 2         | #2                                        |
| 8   | Share Links                 | Medium     | 2         | #2, #6                                    |
| 9   | Quota System                | High       | 3         | #1, #2                                    |
| 10  | Storage Pools               | Very High  | 4         | #1                                        |
| 11  | File Browser UI             | High       | 4         | #2, #6                                    |
| 12  | Drag-and-Drop Uploads       | Medium     | 2         | #11                                       |
| 13  | Trash & Version History     | High       | 3         | #1, #2                                    |
| 14  | Automatic Cleanup Policies  | Medium     | 2         | #1, #2, #9                                |
| 15  | Activity Feed               | Medium     | 2         | #1, #2                                    |
| 16  | File Search                 | High       | 3         | #1, #2, #11                               |
| 17  | Selective Sync Per Device   | Medium     | 2         | #2, #7                                    |
| 18  | Pause/Resume from Dashboard | Low        | 1         | #2                                        |
| 19  | Per-User Bandwidth Limits   | High       | 3         | #2, #3                                    |
| 20  | Sync Scheduling             | Medium     | 2         | #2                                        |
| 21  | Conflict Resolution UI      | High       | 3         | #2, #11                                   |
| 22  | Per-User Encryption Keys    | Very High  | 4         | #2, #6                                    |
| 23  | IP Restrictions             | Low        | 1         | #2, #3                                    |
| 24  | 2FA (TOTP)                  | (included in #5) | 0   | --                                        |
| 25  | Audit Logs                  | Medium     | 2         | #1, #2                                    |
| 26  | Admin Activity Logs         | Low        | 1         | #25                                       |
| 27  | Shared Folders Between Users| High       | 4         | #2, #6                                    |
| 28  | Group Folders               | Medium     | 2         | #27                                       |
| 29  | Comments / File Notes       | Low        | 1.5       | #1, #2, #11                               |
| 30  | Change Notifications        | Medium     | 2.5       | #1, #2, #15                               |
| 31  | User Provisioning (Bulk)    | Medium     | 1.5       | #2, #4                                    |
| 32  | Storage Analytics           | Medium     | 2         | #9, #10                                   |
| 33  | Admin Device Management     | Medium     | 1.5       | #7                                        |
| 34  | Global Bandwidth Control    | Medium     | 1.5       | #19                                       |
| 35  | System Health Monitoring    | Medium     | 2         | None                                      |
| 36  | Automatic Device Backups    | Very High  | 4         | #2, #7, #10                               |
| 37  | Snapshot Backups            | Very High  | 4         | #10, #13                                  |
| 38  | Duplicate Detection         | High       | 3         | #1, #2                                    |
| 39  | Docker Updates              | Medium     | 2         | All above                                 |
| 40  | User Disk Usage Dashboard   | Medium     | 1.5       | #9                                        |

### 5.2 Total Estimate

| Metric                          | Value               |
|---------------------------------|----------------------|
| **Total dev-weeks (sequential)**| ~103 weeks           |
| **Total dev-months**            | ~24 months           |
| **With 2 developers (parallel)**| ~14-16 months        |
| **With 3 developers (parallel)**| ~10-12 months        |

These estimates assume that the AngularJS 1.x frontend is retained. If a frontend migration (e.g., to React or Vue 3) is desired, add ~8-12 weeks.

### 5.3 Complexity Legend

| Rating    | Meaning                                                                       |
|-----------|-------------------------------------------------------------------------------|
| Low       | Straightforward; touches 1-2 files; no architectural changes                  |
| Medium    | Moderate; new API endpoints + GUI views; isolated from sync engine            |
| High      | Significant; touches multiple layers (DB, API, GUI); requires careful design  |
| Very High | Architectural; changes core assumptions (single-user -> multi-user, storage model) |

---

## 6. Implementation Phases

### Phase 1: Foundation (Weeks 1-13)

**Goal:** Multi-user system is functional with basic auth and role separation.

| Week  | Deliverable                                                    | Items      |
|-------|----------------------------------------------------------------|------------|
| 1-3   | Database migration system, new schema tables                   | #1         |
| 4-9   | User model, multi-user auth, session overhaul                  | #2, #3     |
| 10-13 | Admin user management panel, user settings panel               | #4         |

**Milestone:** Multiple users can log in, each sees only their own folders and devices. Admin can create/delete users and assign roles.

### Phase 2: Storage & Access Control (Weeks 14-28)

**Goal:** Users have quota-enforced storage with permissions and file management.

| Week  | Deliverable                                                    | Items      |
|-------|----------------------------------------------------------------|------------|
| 14-16 | Quota system, disk usage tracking, dashboard                   | #9, #40    |
| 17-20 | Storage pools, multi-disk support                              | #10        |
| 21-25 | Folder permissions, device ownership, share links              | #6, #7, #8 |
| 26-28 | Password reset, MFA/2FA                                        | #5         |

**Milestone:** Users have enforced quotas, can share folders with specific permissions, and have MFA. Storage pools are operational for multi-disk setups.

### Phase 3: File Management (Weeks 29-40)

**Goal:** Full file browser, upload/download, trash, versioning, search.

| Week  | Deliverable                                                    | Items      |
|-------|----------------------------------------------------------------|------------|
| 29-32 | File browser UI                                                | #11        |
| 33-34 | Drag-and-drop uploads                                          | #12        |
| 35-37 | Trash, file version history                                    | #13        |
| 38-40 | File search, activity feed                                     | #15, #16   |

**Milestone:** Users can browse, upload, download, search, and restore files entirely from the web UI. Activity feed shows recent changes.

### Phase 4: Sync & Security (Weeks 41-54)

**Goal:** Enhanced sync controls and security hardening.

| Week  | Deliverable                                                    | Items      |
|-------|----------------------------------------------------------------|------------|
| 41-42 | Selective sync per device, pause/resume                        | #17, #18   |
| 43-45 | Per-user bandwidth limits, global bandwidth control            | #19, #34   |
| 46-47 | Sync scheduling                                                | #20        |
| 48-50 | Conflict resolution UI                                         | #21        |
| 51-52 | Per-user encryption keys                                       | #22        |
| 53    | IP restrictions                                                | #23        |
| 54    | Audit logs, admin activity logs                                | #25, #26   |

**Milestone:** Users have fine-grained sync control. Security features (encryption, IP restrictions, audit trails) are in place.

### Phase 5: Collaboration & Admin (Weeks 55-67)

**Goal:** Multi-user collaboration features and admin analytics.

| Week  | Deliverable                                                    | Items      |
|-------|----------------------------------------------------------------|------------|
| 55-58 | Shared folders between users                                   | #27        |
| 59-60 | Group folders                                                  | #28        |
| 61-62 | Comments, file notes                                           | #29        |
| 63-65 | Change notifications                                           | #30        |
| 66    | Bulk user provisioning                                         | #31        |
| 67    | Admin storage analytics, device management                     | #32, #33   |

**Milestone:** Users can collaborate via shared/group folders with comments and notifications. Admin has full analytics and provisioning tools.

### Phase 6: Backup, Dedup & Polish (Weeks 68-80)

**Goal:** Backup system, deduplication, automatic cleanup, system health, Docker finalization.

| Week  | Deliverable                                                    | Items      |
|-------|----------------------------------------------------------------|------------|
| 68-71 | Automatic device backups                                       | #36        |
| 72-75 | Snapshot backups                                                | #37        |
| 76-78 | Duplicate detection                                            | #38        |
| 79    | Automatic cleanup policies                                     | #14        |
| 80    | System health monitoring, Docker updates                       | #35, #39   |

**Milestone:** Full platform is operational with backup, deduplication, cleanup policies, health monitoring, and production-ready Docker images.

---

## 7. Risk Assessment

| Risk                                         | Impact | Likelihood | Mitigation                                                    |
|----------------------------------------------|--------|------------|---------------------------------------------------------------|
| AngularJS 1.x is EOL; ecosystem decay        | High   | High       | Plan frontend migration to modern framework in Phase 7        |
| Multi-user sync conflicts at scale           | High   | Medium     | Extensive conflict resolution testing; leverage BEP's existing vector clocks |
| Storage pool filesystem compatibility (ZFS, RAID) | Medium | Medium | Abstract behind interface; test on Proxmox, Linux LVM, ZFS    |
| Performance degradation with many users      | High   | Medium     | Connection pooling, DB indexing, profiling, load testing       |
| Breaking changes to upstream Syncthing       | High   | Medium     | Maintain fork with clear merge strategy; minimize core BEP changes |
| Scope creep across 80+ weeks                 | High   | High       | Strict phase gates; MVP per phase; user feedback loops         |
| Security vulnerabilities in new auth layer   | Critical | Medium   | Security audit after Phase 2; follow OWASP guidelines          |
| Docker volume permission issues with multi-user | Medium | Medium  | Thorough testing of UID/GID mapping; init container for permissions |

---

*Document generated: 2026-03-16*
*Based on Syncthing codebase analysis (config version 52, Go 1.25+, AngularJS 1.x)*
