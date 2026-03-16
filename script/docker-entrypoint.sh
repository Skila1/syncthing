#!/bin/sh

set -eu

[ -n "${UMASK:-}" ] && umask "$UMASK"

if [ "$(id -u)" = '0' ]; then
  binary="$1"
  if [ -z "${PCAP:-}" ]; then
    # If Syncthing should have no extra capabilities, make sure to remove them
    # from the binary. This will fail with an error if there are no
    # capabilities to remove, hence the || true etc.
    setcap -r "$binary" 2>/dev/null || true
  else
    # Set capabilities on the Syncthing binary before launching it.
    setcap "$PCAP" "$binary"
  fi

  # Create per-user data directory
  if [ -n "${ST_USER_DATA_DIR:-}" ]; then
    mkdir -p "$ST_USER_DATA_DIR"
    chown "${PUID}:${PGID}" "$ST_USER_DATA_DIR" || true
  fi

  # Create backups directory
  mkdir -p /var/syncthing/backups
  chown "${PUID}:${PGID}" /var/syncthing/backups || true

  # Create config directory for SQLite DB and Syncthing config
  mkdir -p "${STHOMEDIR:-/var/syncthing/config}"
  chown "${PUID}:${PGID}" "${STHOMEDIR:-/var/syncthing/config}" || true

  # Ensure storage pool paths exist and have correct ownership
  if [ -n "${ST_STORAGE_POOLS:-}" ]; then
    echo "$ST_STORAGE_POOLS" | tr ',' '\n' | while read -r entry; do
      pool_path="${entry#*=}"
      if [ -n "$pool_path" ]; then
        mkdir -p "$pool_path"
        chown "${PUID}:${PGID}" "$pool_path" || true
      fi
    done
  fi

  # Chown may fail, which may cause us to be unable to start; but maybe
  # it'll work anyway, so we let the error slide.
  chown "${PUID}:${PGID}" "${HOME}" || true
  exec su-exec "${PUID}:${PGID}" \
       env HOME="$HOME" "$@"
else
  exec "$@"
fi
