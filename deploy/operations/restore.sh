#!/bin/sh
set -eu
set +x
umask 077

POSTGRES_IMAGE='postgres:18.4-bookworm@sha256:1961f96e6029a02c3812d7cb329a3b03a3ac2bb067058dec17b0f5596aca9296'
ROOT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)

fail() { printf 'restore error: %s\n' "$1" >&2; exit 1; }
for setting in DATABASE_BACKUP_ARTIFACT CRYPTO_BACKUP_ARTIFACT DATABASE_HOST DATABASE_PORT RECOVERY_ADMIN_USER RECOVERY_ADMIN_PASSWORD_FILE POSTGRES_PASSWORD_FILE DATABASE_MIGRATION_PASSWORD_FILE DATABASE_GATEWAY_PASSWORD_FILE DATABASE_CONTROL_PLANE_PASSWORD_FILE DATABASE_WORKER_PASSWORD_FILE; do
  eval "value=\${$setting-}"
  [ -n "$value" ] || fail "$setting is required"
done
[ "${CONFIRM_EMPTY_RESTORE_TARGET-}" = yes ] || fail 'CONFIRM_EMPTY_RESTORE_TARGET=yes is required'
"$ROOT_DIR/deploy/operations/verify-backup.sh" "$DATABASE_BACKUP_ARTIFACT" "$CRYPTO_BACKUP_ARTIFACT"

case "${NEXUSRELAY_POSTGRES_DOCKER_NETWORK-}" in *[!A-Za-z0-9_.-]* ) fail 'NEXUSRELAY_POSTGRES_DOCKER_NETWORK contains unsupported characters' ;; esac
set --
if [ -n "${NEXUSRELAY_POSTGRES_DOCKER_NETWORK-}" ]; then set -- --network "$NEXUSRELAY_POSTGRES_DOCKER_NETWORK"; fi

docker run --rm "$@" \
  --read-only --tmpfs /tmp:rw,noexec,nosuid,nodev,size=32m \
  --mount "type=bind,src=$ROOT_DIR/deploy/operations/restore-container.sh,dst=/opt/nexusrelay/restore.sh,readonly" \
  --mount "type=bind,src=$ROOT_DIR/deploy/postgres/verify-role-graph.sql,dst=/opt/nexusrelay/verify-role-graph.sql,readonly" \
  --mount "type=bind,src=$ROOT_DIR/deploy/postgres/apply-login-passwords.sh,dst=/opt/nexusrelay/apply-login-passwords.sh,readonly" \
  --mount "type=bind,src=$DATABASE_BACKUP_ARTIFACT,dst=/backup,readonly" \
  --mount "type=bind,src=$RECOVERY_ADMIN_PASSWORD_FILE,dst=/run/recovery/admin_password,readonly" \
  --mount "type=bind,src=$POSTGRES_PASSWORD_FILE,dst=/run/recovery/postgres_cluster_admin_password,readonly" \
  --mount "type=bind,src=$DATABASE_MIGRATION_PASSWORD_FILE,dst=/run/recovery/postgres_migration_password,readonly" \
  --mount "type=bind,src=$DATABASE_GATEWAY_PASSWORD_FILE,dst=/run/recovery/postgres_gateway_password,readonly" \
  --mount "type=bind,src=$DATABASE_CONTROL_PLANE_PASSWORD_FILE,dst=/run/recovery/postgres_control_plane_password,readonly" \
  --mount "type=bind,src=$DATABASE_WORKER_PASSWORD_FILE,dst=/run/recovery/postgres_worker_password,readonly" \
  --env DATABASE_HOST --env DATABASE_PORT --env RECOVERY_ADMIN_USER \
  --env RECOVERY_ADMIN_PASSWORD_FILE=/run/recovery/admin_password \
  --env POSTGRES_PASSWORD_FILE=/run/recovery/postgres_cluster_admin_password \
  --env DATABASE_MIGRATION_PASSWORD_FILE=/run/recovery/postgres_migration_password \
  --env DATABASE_GATEWAY_PASSWORD_FILE=/run/recovery/postgres_gateway_password \
  --env DATABASE_CONTROL_PLANE_PASSWORD_FILE=/run/recovery/postgres_control_plane_password \
  --env DATABASE_WORKER_PASSWORD_FILE=/run/recovery/postgres_worker_password \
  "$POSTGRES_IMAGE" /opt/nexusrelay/restore.sh

printf 'database restored and PostgreSQL 18 role graph verified; restore cryptographic files separately before runtime startup\n'
