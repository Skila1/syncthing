ARG GOVERSION=1.26

#
# Maybe build Syncthing. This is a bit ugly as we can't make an entire
# section of the Dockerfile conditional, so we end up always pulling the
# golang image as builder. Then we check if the executable we need already
# exists (pre-built) otherwise we build it.
#

FROM golang:${GOVERSION}-alpine AS builder
ARG BUILD_USER
ARG BUILD_HOST
ARG TARGETARCH

RUN apk add --no-cache gcc musl-dev sqlite-dev build-base

WORKDIR /src
COPY . .

ENV CGO_ENABLED=1
RUN if [ ! -f syncthing-linux-$TARGETARCH ] ; then \
    go run build.go -no-upgrade build syncthing ; \
    mv syncthing syncthing-linux-$TARGETARCH ; \
  fi

#
# The rest of the Dockerfile uses the binary from the builder, prebuilt or
# not.
#

FROM alpine
ARG TARGETARCH

LABEL org.opencontainers.image.authors="The Syncthing Project" \
      org.opencontainers.image.url="https://syncthing.net" \
      org.opencontainers.image.documentation="https://docs.syncthing.net" \
      org.opencontainers.image.source="https://github.com/syncthing/syncthing" \
      org.opencontainers.image.vendor="The Syncthing Project" \
      org.opencontainers.image.licenses="MPL-2.0" \
      org.opencontainers.image.title="Syncthing"

EXPOSE 8384 22000/tcp 22000/udp 21027/udp

VOLUME ["/var/syncthing"]
VOLUME ["/var/syncthing/users"]
VOLUME ["/var/syncthing/backups"]

RUN apk add --no-cache ca-certificates curl libcap su-exec tzdata sqlite-libs

COPY --from=builder /src/syncthing-linux-$TARGETARCH /bin/syncthing
COPY --from=builder /src/script/docker-entrypoint.sh /bin/entrypoint.sh

ENV PUID=1000 PGID=1000 HOME=/var/syncthing

HEALTHCHECK --interval=1m --timeout=10s \
  CMD curl -fkLsS -m 2 127.0.0.1:8384/rest/noauth/health | grep -o --color=never OK || exit 1

# Core settings
ENV STGUIADDRESS=0.0.0.0:8384
ENV STHOMEDIR=/var/syncthing/config

# Multi-user settings
ENV ST_USER_DATA_DIR=/var/syncthing/users
ENV ST_ADMIN_USER=admin
ENV ST_ADMIN_PASSWORD=

# Storage pools (comma-separated name=path pairs, e.g. "fast=/mnt/ssd,bulk=/mnt/hdd")
ENV ST_STORAGE_POOLS=

# Email / SMTP settings (for password reset & notifications)
ENV ST_SMTP_HOST=
ENV ST_SMTP_PORT=587
ENV ST_SMTP_USER=
ENV ST_SMTP_PASSWORD=
ENV ST_SMTP_FROM=
ENV ST_SMTP_TLS=true

RUN chmod 755 /bin/entrypoint.sh
ENTRYPOINT ["/bin/entrypoint.sh", "/bin/syncthing"]
