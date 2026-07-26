#!/bin/sh
set -eu
set +x

POSTGRES_IMAGE='postgres:18.4-bookworm@sha256:1961f96e6029a02c3812d7cb329a3b03a3ac2bb067058dec17b0f5596aca9296'
ROOT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
test_id="nexusrelay-postgres-init-$$"
container="$test_id"
volume="$test_id-data"
network="$test_id-network"
migrate_image="$test_id-migrate"
migrate_secret_volume="$test_id-migrate-secrets"
postgres_secret_volume="$test_id-postgres-secrets"
collision_secret_volume="$test_id-collision-secrets"
secret_dir=$(mktemp -d)

cleanup() {
  docker rm --force "$container" >/dev/null 2>&1 || true
  docker volume rm "$volume" >/dev/null 2>&1 || true
  docker network rm "$network" >/dev/null 2>&1 || true
  docker image rm "$migrate_image" >/dev/null 2>&1 || true
  docker volume rm "$migrate_secret_volume" >/dev/null 2>&1 || true
  docker volume rm "$postgres_secret_volume" "$collision_secret_volume" >/dev/null 2>&1 || true
  rm -rf "$secret_dir"
}
trap cleanup EXIT HUP INT TERM

write_secret() {
  name=$1
  value=$2
  printf '%s' "$value" >"$secret_dir/$name"
  chmod 0600 "$secret_dir/$name"
}

publish_postgres_secrets() {
  target_volume=$1
  docker volume create "$target_volume" >/dev/null
  docker run --rm --user 0:0 --network none \
    --mount "type=bind,src=$secret_dir,dst=/source,readonly" \
    --mount "type=volume,src=$target_volume,dst=/target" \
    busybox:1.37.0-uclibc@sha256:39e0df8c4d65953b55c344f017e1ff2e0031a7454b3c24e6b76d402f207e315a \
    sh -ec 'cp /source/postgres_*_password /target/ && chown 999:999 /target/* && chmod 0400 /target/*'
}

expect_equivalent_passwords_rejected() {
  collision_container="$test_id-collision"
  collision_volume="$test_id-collision-data"
  printf '%s\n' same-password >"$secret_dir/postgres_gateway_password"
  printf '%s\r\n' same-password >"$secret_dir/postgres_worker_password"
  chmod 0600 "$secret_dir/postgres_gateway_password" "$secret_dir/postgres_worker_password"
  publish_postgres_secrets "$collision_secret_volume"
  docker volume create "$collision_volume" >/dev/null
  if docker run --name "$collision_container" \
    --env POSTGRES_DB=nexusrelay \
    --env POSTGRES_USER=nexusrelay_cluster_admin \
    --env POSTGRES_PASSWORD_FILE=/run/secrets/postgres_cluster_admin_password \
    --env DATABASE_NAME=nexusrelay \
    --env DATABASE_MIGRATION_USER=nexusrelay_migration \
    --env DATABASE_MIGRATION_PASSWORD_FILE=/run/secrets/postgres_migration_password \
    --env DATABASE_GATEWAY_USER=nexusrelay_gateway \
    --env DATABASE_GATEWAY_PASSWORD_FILE=/run/secrets/postgres_gateway_password \
    --env DATABASE_CONTROL_PLANE_USER=nexusrelay_control_plane \
    --env DATABASE_CONTROL_PLANE_PASSWORD_FILE=/run/secrets/postgres_control_plane_password \
    --env DATABASE_WORKER_USER=nexusrelay_worker \
    --env DATABASE_WORKER_PASSWORD_FILE=/run/secrets/postgres_worker_password \
    --mount "type=volume,src=$collision_volume,dst=/var/lib/postgresql" \
    --mount "type=bind,src=$ROOT_DIR/deploy/postgres/init/10-nexusrelay-roles.sh,dst=/docker-entrypoint-initdb.d/10-nexusrelay-roles.sh,readonly" \
    --mount "type=volume,src=$collision_secret_volume,dst=/run/secrets,readonly" \
    "$POSTGRES_IMAGE" >/dev/null 2>&1; then
    printf 'equivalent LF/CRLF database passwords unexpectedly accepted\n' >&2
    exit 1
  fi
  docker rm "$collision_container" >/dev/null 2>&1 || true
  docker volume rm "$collision_volume" >/dev/null 2>&1 || true
  docker volume rm "$collision_secret_volume" >/dev/null 2>&1 || true
  write_secret postgres_gateway_password gateway-test-password
  write_secret postgres_worker_password worker-test-password
}

write_secret postgres_cluster_admin_password cluster-test-password
write_secret postgres_migration_password migration-test-password
write_secret postgres_gateway_password gateway-test-password
write_secret postgres_control_plane_password control-test-password
write_secret postgres_worker_password worker-test-password
expect_equivalent_passwords_rejected
publish_postgres_secrets "$postgres_secret_volume"

docker volume create "$volume" >/dev/null
docker network create "$network" >/dev/null
docker run --detach --name "$container" \
  --network "$network" \
  --env POSTGRES_DB=nexusrelay \
  --env POSTGRES_USER=nexusrelay_cluster_admin \
  --env POSTGRES_PASSWORD_FILE=/run/secrets/postgres_cluster_admin_password \
  --env DATABASE_NAME=nexusrelay \
  --env DATABASE_MIGRATION_USER=nexusrelay_migration \
  --env DATABASE_MIGRATION_PASSWORD_FILE=/run/secrets/postgres_migration_password \
  --env DATABASE_GATEWAY_USER=nexusrelay_gateway \
  --env DATABASE_GATEWAY_PASSWORD_FILE=/run/secrets/postgres_gateway_password \
  --env DATABASE_CONTROL_PLANE_USER=nexusrelay_control_plane \
  --env DATABASE_CONTROL_PLANE_PASSWORD_FILE=/run/secrets/postgres_control_plane_password \
  --env DATABASE_WORKER_USER=nexusrelay_worker \
  --env DATABASE_WORKER_PASSWORD_FILE=/run/secrets/postgres_worker_password \
  --mount "type=volume,src=$volume,dst=/var/lib/postgresql" \
  --mount "type=bind,src=$ROOT_DIR/deploy/postgres/init/10-nexusrelay-roles.sh,dst=/docker-entrypoint-initdb.d/10-nexusrelay-roles.sh,readonly" \
  --mount "type=volume,src=$postgres_secret_volume,dst=/run/secrets,readonly" \
  "$POSTGRES_IMAGE" >/dev/null

attempt=0
until docker exec --user postgres "$container" sh -c 'test -f "$PGDATA/.nexusrelay-initialized" && pg_isready -q -d nexusrelay'; do
  attempt=$((attempt + 1))
  if [ "$(docker inspect --format '{{.State.Running}}' "$container")" != true ]; then
    docker logs "$container" >&2
    exit 1
  fi
  if [ "$attempt" -ge 60 ]; then
    docker logs "$container" >&2
    exit 1
  fi
  sleep 1
done

query() {
  docker exec --user postgres "$container" psql --username nexusrelay_cluster_admin --dbname nexusrelay --tuples-only --no-align --set ON_ERROR_STOP=1 --command "$1"
}

docker exec --user postgres -i "$container" psql --username nexusrelay_cluster_admin --dbname nexusrelay --no-align --tuples-only --set ON_ERROR_STOP=1 <"$ROOT_DIR/deploy/postgres/verify-role-graph.sql" >/dev/null
query 'CREATE ROLE nexusrelay_unexpected NOLOGIN' >/dev/null
if docker exec --user postgres -i "$container" psql --username nexusrelay_cluster_admin --dbname nexusrelay --no-align --tuples-only --set ON_ERROR_STOP=1 <"$ROOT_DIR/deploy/postgres/verify-role-graph.sql" >/dev/null 2>&1; then
  printf 'PostgreSQL role verifier accepted an extra NexusRelay role\n' >&2
  exit 1
fi
query 'DROP ROLE nexusrelay_unexpected' >/dev/null
docker exec --user postgres -i "$container" psql --username nexusrelay_cluster_admin --dbname nexusrelay --no-align --tuples-only --set ON_ERROR_STOP=1 <"$ROOT_DIR/deploy/postgres/verify-role-graph.sql" >/dev/null

schemas=$(query "SELECT nspname || '|' || pg_get_userbyid(nspowner) FROM pg_namespace WHERE nspname IN ('public', 'nexusrelay', 'nexusrelay_migration') ORDER BY nspname")
expected_schemas='nexusrelay|nexusrelay_schema_owner
nexusrelay_migration|nexusrelay_migration
public|pg_database_owner'
[ "$schemas" = "$expected_schemas" ] || {
  printf 'unexpected schema ownership:\n%s\n' "$schemas" >&2
  exit 1
}

docker build --quiet \
  --file "$ROOT_DIR/deploy/migrate/Dockerfile" \
  --build-arg VERSION=postgres-init-test \
  --build-arg REVISION=postgres-init-test \
  --tag "$migrate_image" \
  "$ROOT_DIR" >/dev/null
docker volume create "$migrate_secret_volume" >/dev/null
docker run --rm --user 0:0 \
  --mount "type=bind,src=$secret_dir/postgres_migration_password,dst=/source/password,readonly" \
  --mount "type=volume,src=$migrate_secret_volume,dst=/target" \
  busybox:1.37.0-uclibc@sha256:39e0df8c4d65953b55c344f017e1ff2e0031a7454b3c24e6b76d402f207e315a \
  sh -c 'cp /source/password /target/postgres_migration_password && chown 65532:65532 /target/postgres_migration_password && chmod 0400 /target/postgres_migration_password'
docker run --rm \
  --user 65532:65532 \
  --network "$network" \
  --read-only \
  --tmpfs /tmp:rw,noexec,nosuid,nodev,size=16m \
  --env DATABASE_HOST="$container" \
  --env DATABASE_PORT=5432 \
  --env DATABASE_NAME=nexusrelay \
  --env DATABASE_MIGRATION_USER=nexusrelay_migration \
  --env DATABASE_MIGRATION_PASSWORD_FILE=/run/secrets/postgres_migration_password \
  --env DATABASE_SSLMODE=disable \
  --mount "type=volume,src=$migrate_secret_volume,dst=/run/secrets,readonly" \
  "$migrate_image" >/dev/null

revision_owner=$(query "SELECT pg_get_userbyid(relowner) FROM pg_class JOIN pg_namespace ON pg_namespace.oid = pg_class.relnamespace WHERE nspname = 'nexusrelay_migration' AND relname = 'atlas_schema_revisions'")
[ "$revision_owner" = nexusrelay_migration ] || {
  printf 'Atlas revision table owner = %s, want nexusrelay_migration\n' "$revision_owner" >&2
  exit 1
}

query "SET SESSION AUTHORIZATION nexusrelay_migration; SET ROLE nexusrelay_schema_owner; RESET ROLE; SET ROLE nexusrelay_security_definer_owner;" >/dev/null
if query "SET SESSION AUTHORIZATION nexusrelay_gateway; SET ROLE nexusrelay_gateway_runtime;" >/dev/null 2>&1; then
  printf 'gateway unexpectedly used SET ROLE for its runtime role\n' >&2
  exit 1
fi
if query "SET SESSION AUTHORIZATION nexusrelay_migration; CREATE ROLE nexusrelay_forbidden;" >/dev/null 2>&1; then
  printf 'migration unexpectedly created a role\n' >&2
  exit 1
fi

public_create=$(query "SELECT has_schema_privilege('nexusrelay_gateway', 'public', 'CREATE')")
[ "$public_create" = f ] || {
  printf 'PUBLIC retains CREATE on the public schema\n' >&2
  exit 1
}

database_privileges=$(query "SELECT rolname || '|' || has_database_privilege(rolname, 'nexusrelay', 'CONNECT') || '|' || has_database_privilege(rolname, 'nexusrelay', 'TEMPORARY') FROM pg_roles WHERE rolname IN ('nexusrelay_migration', 'nexusrelay_gateway', 'nexusrelay_control_plane', 'nexusrelay_worker') ORDER BY rolname")
expected_database_privileges='nexusrelay_control_plane|true|false
nexusrelay_gateway|true|false
nexusrelay_migration|true|true
nexusrelay_worker|true|false'
[ "$database_privileges" = "$expected_database_privileges" ] || {
  printf 'unexpected effective database privileges:\n%s\n' "$database_privileges" >&2
  exit 1
}

initialized_before=$(docker exec --user postgres "$container" stat -c %Y /var/lib/postgresql/18/docker/.nexusrelay-initialized)
docker restart "$container" >/dev/null
attempt=0
until docker exec --user postgres "$container" pg_isready -q -d nexusrelay; do
  attempt=$((attempt + 1))
  [ "$attempt" -lt 60 ] || exit 1
  sleep 1
done
initialized_after=$(docker exec --user postgres "$container" stat -c %Y /var/lib/postgresql/18/docker/.nexusrelay-initialized)
[ "$initialized_before" = "$initialized_after" ] || {
  printf 'empty-volume initialization unexpectedly reran\n' >&2
  exit 1
}

if docker logs "$container" 2>&1 | grep -F 'test-password' >/dev/null; then
  printf 'PostgreSQL logs disclosed a test password\n' >&2
  exit 1
fi

printf 'PostgreSQL initialization contract valid\n'
