#!/bin/sh
set -eu
set +x
umask 077

POSTGRES_IMAGE='postgres:18.4-bookworm@sha256:1961f96e6029a02c3812d7cb329a3b03a3ac2bb067058dec17b0f5596aca9296'
ROOT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
test_id="nexusrelay-restore-$$"
network="$test_id"
source_container="$test_id-source"
restore_container="$test_id-target"
source_secret_volume="$test_id-source-secrets"
work=$(mktemp -d)

cleanup() {
  docker rm --force "$source_container" "$restore_container" >/dev/null 2>&1 || true
  docker network rm "$network" >/dev/null 2>&1 || true
  docker volume rm "$source_secret_volume" >/dev/null 2>&1 || true
  rm -rf "$work"
}
trap cleanup EXIT HUP INT TERM
mkdir "$work/database" "$work/crypto" "$work/secrets"
chmod 0700 "$work/database" "$work/crypto" "$work/secrets"

write_secret() { printf '%s' "$2" >"$work/secrets/$1"; chmod 0600 "$work/secrets/$1"; }
write_secret postgres_cluster_admin_password source-cluster-password
write_secret postgres_migration_password source-migration-password
write_secret postgres_gateway_password source-gateway-password
write_secret postgres_control_plane_password source-control-password
write_secret postgres_worker_password source-worker-password
write_secret provider_master_keyring '{"version":1,"expected_epoch":1,"active_key_id":"master-test","keys":[{"key_id":"master-test","key":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="}]}'
write_secret api_key_pepper_ring '{"version":1,"active_key_id":"pepper-test","keys":[{"key_id":"pepper-test","key":"BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB="}]}'
write_secret csrf_secret_ring '{"version":1,"active_key_id":"csrf-test","keys":[{"key_id":"csrf-test","key":"CCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCC="}]}'
write_secret session_secret 'REREREREREREREREREREREREREREREREREREREREREQ='
write_secret restore_admin_password restore-admin-password

docker volume create "$source_secret_volume" >/dev/null
docker run --rm --user 0:0 --network none \
  --mount "type=bind,src=$work/secrets,dst=/source,readonly" \
  --mount "type=volume,src=$source_secret_volume,dst=/target" \
  busybox:1.37.0-uclibc@sha256:39e0df8c4d65953b55c344f017e1ff2e0031a7454b3c24e6b76d402f207e315a \
  sh -ec 'cp /source/postgres_*_password /target/ && chown 999:999 /target/* && chmod 0400 /target/*'

docker network create "$network" >/dev/null
docker run --detach --name "$source_container" --network "$network" \
  --env POSTGRES_DB=nexusrelay --env POSTGRES_USER=nexusrelay_cluster_admin \
  --env POSTGRES_PASSWORD_FILE=/run/secrets/postgres_cluster_admin_password \
  --env DATABASE_NAME=nexusrelay --env DATABASE_MIGRATION_USER=nexusrelay_migration \
  --env DATABASE_MIGRATION_PASSWORD_FILE=/run/secrets/postgres_migration_password \
  --env DATABASE_GATEWAY_USER=nexusrelay_gateway --env DATABASE_GATEWAY_PASSWORD_FILE=/run/secrets/postgres_gateway_password \
  --env DATABASE_CONTROL_PLANE_USER=nexusrelay_control_plane --env DATABASE_CONTROL_PLANE_PASSWORD_FILE=/run/secrets/postgres_control_plane_password \
  --env DATABASE_WORKER_USER=nexusrelay_worker --env DATABASE_WORKER_PASSWORD_FILE=/run/secrets/postgres_worker_password \
  --mount "type=bind,src=$ROOT_DIR/deploy/postgres/init/10-nexusrelay-roles.sh,dst=/docker-entrypoint-initdb.d/10-nexusrelay-roles.sh,readonly" \
  --mount "type=volume,src=$source_secret_volume,dst=/run/secrets,readonly" "$POSTGRES_IMAGE" >/dev/null

attempt=0
until docker run --rm --network "$network" "$POSTGRES_IMAGE" pg_isready -q -h "$source_container" -U nexusrelay_cluster_admin -d nexusrelay; do attempt=$((attempt + 1)); [ "$attempt" -lt 60 ] || exit 1; sleep 1; done
BACKUP_ID=restore-test NEXUSRELAY_RELEASE=phase2-test NEXUSRELAY_REVISION=restore-test \
BACKUP_DATABASE_ROOT="$work/database" BACKUP_CRYPTO_ROOT="$work/crypto" CRYPTO_SOURCE_ROOT="$work/secrets" \
DATABASE_HOST="$source_container" DATABASE_PORT=5432 DATABASE_NAME=nexusrelay POSTGRES_USER=nexusrelay_cluster_admin \
POSTGRES_PASSWORD_FILE="$work/secrets/postgres_cluster_admin_password" NEXUSRELAY_POSTGRES_DOCKER_NETWORK="$network" \
  "$ROOT_DIR/deploy/operations/backup.sh"

docker run --detach --name "$restore_container" --network "$network" \
  --env POSTGRES_DB=postgres --env POSTGRES_USER=postgres --env POSTGRES_PASSWORD_FILE=/run/secrets/restore_admin_password \
  --mount "type=bind,src=$work/secrets/restore_admin_password,dst=/run/secrets/restore_admin_password,readonly" \
  "$POSTGRES_IMAGE" >/dev/null
attempt=0
until docker run --rm --network "$network" "$POSTGRES_IMAGE" pg_isready -q -h "$restore_container" -U postgres -d postgres; do attempt=$((attempt + 1)); [ "$attempt" -lt 60 ] || exit 1; sleep 1; done
DATABASE_BACKUP_ARTIFACT="$work/database/restore-test" CRYPTO_BACKUP_ARTIFACT="$work/crypto/restore-test" \
DATABASE_HOST="$restore_container" DATABASE_PORT=5432 RECOVERY_ADMIN_USER=postgres RECOVERY_ADMIN_PASSWORD_FILE="$work/secrets/restore_admin_password" \
POSTGRES_PASSWORD_FILE="$work/secrets/postgres_cluster_admin_password" DATABASE_MIGRATION_PASSWORD_FILE="$work/secrets/postgres_migration_password" \
DATABASE_GATEWAY_PASSWORD_FILE="$work/secrets/postgres_gateway_password" DATABASE_CONTROL_PLANE_PASSWORD_FILE="$work/secrets/postgres_control_plane_password" \
DATABASE_WORKER_PASSWORD_FILE="$work/secrets/postgres_worker_password" CONFIRM_EMPTY_RESTORE_TARGET=yes NEXUSRELAY_POSTGRES_DOCKER_NETWORK="$network" \
  "$ROOT_DIR/deploy/operations/restore.sh"

docker exec "$restore_container" psql -U postgres -d nexusrelay -Atqc "SELECT pg_get_userbyid(nspowner) FROM pg_namespace WHERE nspname='nexusrelay'" | grep -qx nexusrelay_schema_owner
mkdir "$work/recovered-crypto"
chmod 0700 "$work/recovered-crypto"
"$ROOT_DIR/deploy/operations/restore-crypto.sh" "$work/crypto/restore-test" "$work/recovered-crypto"
for name in provider_master_keyring api_key_pepper_ring csrf_secret_ring session_secret; do cmp "$work/secrets/$name" "$work/recovered-crypto/$name" >/dev/null; done

mkdir "$work/graph-evidence"
chmod 0700 "$work/graph-evidence"
printf 'SELECT 1;\n' >"$work/no-op.sql"
sql_digest=$(shasum -a 256 "$work/no-op.sql" | cut -d ' ' -f 1)
cat >"$work/graph-request.env" <<EOF
FORMAT=nexusrelay-graph-upgrade-request-v1
CHANGE_ID=restore-test-no-op
FROM_RELEASE=phase2-test
TO_RELEASE=phase2-test
POSTGRESQL_MAJOR=18
POSTGRESQL_MINOR=18.4
ATLAS_VERSION=1.2.3-community
ROLE_GRAPH_CONTRACT=postgresql-18-v1
MUTATION_SQL_SHA256=$sql_digest
APPROVER=automated-test
APPROVED_AT_UTC=2026-07-26T00:00:00Z
EOF
GRAPH_UPGRADE_REQUEST="$work/graph-request.env" GRAPH_UPGRADE_MUTATION_SQL="$work/no-op.sql" GRAPH_UPGRADE_EVIDENCE_ROOT="$work/graph-evidence" \
DATABASE_HOST="$restore_container" DATABASE_PORT=5432 DATABASE_NAME=nexusrelay POSTGRES_USER=nexusrelay_cluster_admin \
POSTGRES_PASSWORD_FILE="$work/secrets/postgres_cluster_admin_password" NEXUSRELAY_POSTGRES_DOCKER_NETWORK="$network" CONFIRM_SERVICES_STOPPED=yes \
  "$ROOT_DIR/deploy/operations/graph-upgrade.sh"
(cd "$work/graph-evidence/restore-test-no-op" && shasum -a 256 -c SHA256SUMS >/dev/null)
grep -qx 'ATLAS_EXECUTED=false' "$work/graph-evidence/restore-test-no-op/runner.env"
printf 'isolated PostgreSQL logical backup and restore exercise passed\n'
