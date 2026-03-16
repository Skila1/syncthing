# Docker Container for Syncthing (Multi-User)

Use the Dockerfile in this repo, or pull the `syncthing/syncthing` image
from Docker Hub.

Use the `/var/syncthing` volume to have the synchronized files available on the
host. You can add more folders and map them as you prefer.

Note that Syncthing runs as UID 1000 and GID 1000 by default. These may be
altered with the `PUID` and `PGID` environment variables. In addition
the name of the Syncthing instance can be optionally defined by using
`--hostname=syncthing` parameter.

To grant Syncthing additional capabilities without running as root, use the
`PCAP` environment variable with the same syntax as that for `setcap(8)`.
For example, `PCAP=cap_chown,cap_fowner+ep`.

To set a different umask value, use the `UMASK` environment variable. For
example `UMASK=002`.

## Quick Start (Multi-User)

```bash
docker run -d --name syncthing \
  --network=host \
  -e ST_ADMIN_USER=admin \
  -e ST_ADMIN_PASSWORD=changeme \
  -v /data/syncthing:/var/syncthing \
  -v /data/syncthing/users:/var/syncthing/users \
  syncthing/syncthing:latest
```

Log into the web GUI at `http://localhost:8384`, sign in as the admin user,
then create additional users from the admin menu.

## Docker Compose (Full Example)

```yml
version: "3.8"
services:
  syncthing:
    build: .
    container_name: syncthing
    hostname: my-syncthing
    environment:
      - PUID=1000
      - PGID=1000

      # GUI listen address (0.0.0.0 exposes to all interfaces)
      - STGUIADDRESS=0.0.0.0:8384

      # Admin account (created on first launch)
      - ST_ADMIN_USER=admin
      - ST_ADMIN_PASSWORD=changeme

      # Per-user data isolation directory
      - ST_USER_DATA_DIR=/var/syncthing/users

      # Storage pools: comma-separated name=path pairs
      # - ST_STORAGE_POOLS=fast=/mnt/ssd,bulk=/mnt/hdd

      # Email / SMTP (for password reset & notifications)
      # - ST_SMTP_HOST=smtp.example.com
      # - ST_SMTP_PORT=587
      # - ST_SMTP_USER=syncthing@example.com
      # - ST_SMTP_PASSWORD=secret
      # - ST_SMTP_FROM=syncthing@example.com
      # - ST_SMTP_TLS=true

    volumes:
      # Main Syncthing data + config
      - syncthing-data:/var/syncthing

      # Per-user isolated data
      - syncthing-users:/var/syncthing/users

      # Backups volume
      - syncthing-backups:/var/syncthing/backups

      # Optional: storage pool volumes
      # - /mnt/ssd:/mnt/ssd
      # - /mnt/hdd:/mnt/hdd

    network_mode: host
    restart: unless-stopped
    healthcheck:
      test: curl -fkLsS -m 2 127.0.0.1:8384/rest/noauth/health | grep -o --color=never OK || exit 1
      interval: 1m
      timeout: 10s
      retries: 3

volumes:
  syncthing-data:
  syncthing-users:
  syncthing-backups:
```

## Environment Variables

### Core

| Variable | Default | Description |
|----------|---------|-------------|
| `PUID` | `1000` | User ID to run Syncthing as |
| `PGID` | `1000` | Group ID to run Syncthing as |
| `STGUIADDRESS` | `0.0.0.0:8384` | GUI/API listen address |
| `STHOMEDIR` | `/var/syncthing/config` | Syncthing configuration and database directory |
| `UMASK` | *(unset)* | Custom umask value |
| `PCAP` | *(unset)* | Linux capabilities for the binary (e.g. `cap_chown+ep`) |

### Multi-User

| Variable | Default | Description |
|----------|---------|-------------|
| `ST_ADMIN_USER` | `admin` | Username for the initial admin account (created on first launch) |
| `ST_ADMIN_PASSWORD` | *(empty)* | Password for the initial admin account. **Set this on first launch.** |
| `ST_USER_DATA_DIR` | `/var/syncthing/users` | Root directory for per-user isolated data |

### Storage Pools

| Variable | Default | Description |
|----------|---------|-------------|
| `ST_STORAGE_POOLS` | *(empty)* | Comma-separated `name=path` pairs for multi-disk storage pools. Example: `fast=/mnt/ssd,bulk=/mnt/hdd`. Pools are auto-registered on startup. |

### Email / SMTP

| Variable | Default | Description |
|----------|---------|-------------|
| `ST_SMTP_HOST` | *(empty)* | SMTP server hostname. Email features are disabled when empty. |
| `ST_SMTP_PORT` | `587` | SMTP server port |
| `ST_SMTP_USER` | *(empty)* | SMTP authentication username |
| `ST_SMTP_PASSWORD` | *(empty)* | SMTP authentication password |
| `ST_SMTP_FROM` | `syncthing@localhost` | Sender address for outgoing emails |
| `ST_SMTP_TLS` | `true` | Use TLS for SMTP connection (`true`/`false`) |

## Volumes

| Path | Purpose |
|------|---------|
| `/var/syncthing` | Main data directory (config, default sync folders) |
| `/var/syncthing/config` | Syncthing configuration and SQLite database |
| `/var/syncthing/users` | Per-user isolated data directories |
| `/var/syncthing/backups` | Backup and snapshot storage |

When using storage pools, mount each pool path as a separate volume (e.g.
`/mnt/ssd`, `/mnt/hdd`) and set `ST_STORAGE_POOLS` accordingly.

## Database

The multi-user platform uses SQLite, stored at `$STHOMEDIR/index-v0.14.0.db`
alongside the existing Syncthing database. Schema migrations run automatically
on startup -- no manual steps required.

## Discovery

Please note that Docker's default network mode prevents local IP addresses
from being discovered, as Syncthing can only see the internal IP address of
the container on the `172.17.0.0/16` subnet. This would likely break the ability
for nodes to establish LAN connections properly, resulting in poor transfer
rates unless local device addresses are configured manually.

It is therefore strongly recommended to stick to the [host network mode](https://docs.docker.com/network/host/),
as shown above.

Be aware that syncthing alone is now in control of what interfaces and ports it
listens on. You can edit the syncthing configuration to change the defaults if
there are conflicts.

## GUI Security

By default Syncthing inside the Docker image listens on `0.0.0.0:8384`. This
allows GUI connections when running without host network mode. The example
above unsets the `STGUIADDRESS` environment variable to have Syncthing fall
back to listening on what has been configured in the configuration file or the
GUI settings dialog. By default this is the localhost IP address `127.0.0.1`.
If you configure your GUI to be externally reachable, make sure you set up
authentication and enable TLS.

## Multi-User Features

Once deployed, the admin user can manage the platform through the web GUI:

- **Users**: Create/delete users, assign roles (admin/user), set storage quotas
- **Folders**: Per-user folder isolation, shared folders via groups, granular permissions
- **Groups**: Create groups, assign members, share folders with read/read-write access
- **Share Links**: Generate temporary download links with optional passwords and expiry
- **File Browser**: Web-based file browsing, upload, download, preview, and search
- **Backups**: Scheduled full/incremental backups with restore points
- **Snapshots**: Point-in-time folder snapshots with one-click restore
- **Deduplication**: Content-hash scanning to detect and resolve duplicate files
- **Security**: MFA/TOTP, audit logging, IP restrictions, per-user encryption
- **Notifications**: In-app and email notifications for sync events
- **Storage Pools**: Multi-disk storage management with assignment strategies
- **Admin Dashboard**: Analytics, health monitoring, user provisioning, alerting
