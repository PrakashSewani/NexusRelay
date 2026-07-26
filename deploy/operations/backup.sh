#!/bin/sh
set -eu
set +x
umask 077

POSTGRES_IMAGE='postgres:18.4-bookworm@sha256:1961f96e6029a02c3812d7cb329a3b03a3ac2bb067058dec17b0f5596aca9296'
BUSYBOX_IMAGE='busybox:1.37.0-uclibc@sha256:39e0df8c4d65953b55c344f017e1ff2e0031a7454b3c24e6b76d402f207e315a'
ROOT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)

fail() {
  printf 'backup error: %s\n' "$1" >&2
  exit 1
}

mode_of() {
  stat -c %a "$1" 2>/dev/null || stat -f %Lp "$1"
}

usage() {
  printf '%s\n' 'Usage: deploy/operations/backup.sh' \
    'Required environment:' \
    '  BACKUP_ID NEXUSRELAY_RELEASE NEXUSRELAY_REVISION' \
    '  BACKUP_DATABASE_ROOT BACKUP_CRYPTO_ROOT CRYPTO_SOURCE_ROOT' \
    '  DATABASE_HOST DATABASE_PORT DATABASE_NAME POSTGRES_USER' \
    '  POSTGRES_PASSWORD_FILE' \
    'Optional: NEXUSRELAY_POSTGRES_DOCKER_NETWORK'
}

for setting in BACKUP_ID NEXUSRELAY_RELEASE NEXUSRELAY_REVISION BACKUP_DATABASE_ROOT BACKUP_CRYPTO_ROOT CRYPTO_SOURCE_ROOT DATABASE_HOST DATABASE_PORT DATABASE_NAME POSTGRES_USER POSTGRES_PASSWORD_FILE; do
  eval "value=\${$setting-}"
  [ -n "$value" ] || fail "$setting is required"
done
case "$BACKUP_ID" in [A-Za-z0-9]* ) ;; *) fail 'BACKUP_ID must start with an ASCII letter or digit' ;; esac
case "$BACKUP_ID" in *[!A-Za-z0-9._-]* ) fail 'BACKUP_ID contains unsupported characters' ;; esac
case "$NEXUSRELAY_RELEASE" in [A-Za-z0-9]* ) ;; *) fail 'NEXUSRELAY_RELEASE must start with an ASCII letter or digit' ;; esac
case "$NEXUSRELAY_RELEASE" in *[!A-Za-z0-9._+-]* ) fail 'NEXUSRELAY_RELEASE contains unsupported characters' ;; esac
case "$NEXUSRELAY_REVISION" in [A-Za-z0-9]* ) ;; *) fail 'NEXUSRELAY_REVISION must start with an ASCII letter or digit' ;; esac
case "$NEXUSRELAY_REVISION" in *[!A-Za-z0-9._-]* ) fail 'NEXUSRELAY_REVISION contains unsupported characters' ;; esac
[ "$POSTGRES_USER" = nexusrelay_cluster_admin ] || fail 'POSTGRES_USER must be nexusrelay_cluster_admin for backup/recovery only'
[ "$DATABASE_NAME" = nexusrelay ] || fail 'DATABASE_NAME must be nexusrelay'
case "$DATABASE_PORT" in ''|*[!0-9]*) fail 'DATABASE_PORT must be numeric' ;; esac

for root in "$BACKUP_DATABASE_ROOT" "$BACKUP_CRYPTO_ROOT" "$CRYPTO_SOURCE_ROOT"; do
  [ -d "$root" ] && [ ! -L "$root" ] || fail 'backup and source roots must be existing non-symlink directories'
done
[ "$(mode_of "$BACKUP_DATABASE_ROOT")" = 700 ] || fail 'BACKUP_DATABASE_ROOT must have mode 0700'
[ "$(mode_of "$BACKUP_CRYPTO_ROOT")" = 700 ] || fail 'BACKUP_CRYPTO_ROOT must have mode 0700'
[ "$(mode_of "$CRYPTO_SOURCE_ROOT")" = 700 ] || fail 'CRYPTO_SOURCE_ROOT must have mode 0700'
[ "$(cd "$BACKUP_DATABASE_ROOT" && pwd -P)" != "$(cd "$BACKUP_CRYPTO_ROOT" && pwd -P)" ] || fail 'database and cryptographic backup roots must be separate'
[ -f "$POSTGRES_PASSWORD_FILE" ] && [ ! -L "$POSTGRES_PASSWORD_FILE" ] || fail 'POSTGRES_PASSWORD_FILE must be a regular non-symlink file'

database_target="$BACKUP_DATABASE_ROOT/$BACKUP_ID"
crypto_target="$BACKUP_CRYPTO_ROOT/$BACKUP_ID"
[ ! -e "$database_target" ] && [ ! -e "$crypto_target" ] || fail 'BACKUP_ID already exists in a destination root'

set --
if [ -n "${NEXUSRELAY_POSTGRES_DOCKER_NETWORK-}" ]; then
  case "$NEXUSRELAY_POSTGRES_DOCKER_NETWORK" in *[!A-Za-z0-9_.-]*) fail 'NEXUSRELAY_POSTGRES_DOCKER_NETWORK contains unsupported characters' ;; esac
  set -- --network "$NEXUSRELAY_POSTGRES_DOCKER_NETWORK"
fi

docker run --rm "$@" \
  --user "$(id -u):$(id -g)" \
  --read-only \
  --tmpfs /tmp:rw,noexec,nosuid,nodev,size=32m \
  --mount "type=bind,src=$ROOT_DIR/deploy/operations/database-backup-container.sh,dst=/opt/nexusrelay/backup.sh,readonly" \
  --mount "type=bind,src=$ROOT_DIR/deploy/postgres/verify-role-graph.sql,dst=/opt/nexusrelay/verify-role-graph.sql,readonly" \
  --mount "type=bind,src=$POSTGRES_PASSWORD_FILE,dst=/run/secrets/postgres_cluster_admin_password,readonly" \
  --mount "type=bind,src=$BACKUP_DATABASE_ROOT,dst=/backup" \
  --env BACKUP_ID \
  --env NEXUSRELAY_RELEASE \
  --env NEXUSRELAY_REVISION \
  --env DATABASE_HOST \
  --env DATABASE_PORT \
  --env DATABASE_NAME \
  --env POSTGRES_USER \
  --env POSTGRES_PASSWORD_FILE=/run/secrets/postgres_cluster_admin_password \
  "$POSTGRES_IMAGE" /opt/nexusrelay/backup.sh

if ! docker run --rm \
  --network none \
  --user "$(id -u):$(id -g)" \
  --read-only \
  --mount "type=bind,src=$ROOT_DIR/deploy/operations/crypto-backup-container.sh,dst=/opt/nexusrelay/backup.sh,readonly" \
  --mount "type=bind,src=$CRYPTO_SOURCE_ROOT,dst=/source,readonly" \
  --mount "type=bind,src=$BACKUP_CRYPTO_ROOT,dst=/backup" \
  --env BACKUP_ID \
  --env NEXUSRELAY_RELEASE \
  --env NEXUSRELAY_REVISION \
  "$BUSYBOX_IMAGE" /opt/nexusrelay/backup.sh; then
  fail "cryptographic backup failed; retained database artifact $database_target requires an independently completed cryptographic artifact"
fi

printf 'backup %s published as separate database and cryptographic artifacts\n' "$BACKUP_ID"
