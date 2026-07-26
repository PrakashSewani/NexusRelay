#!/bin/sh
set -eu
set +x

ATLAS_IMAGE='arigaio/atlas:1.2.3-community@sha256:d43f072e79dd98554b890fb56d159b14c0a4ccdb8609296da16402d015483cfa'
POSTGRES_IMAGE='postgres:18.4-bookworm@sha256:1961f96e6029a02c3812d7cb329a3b03a3ac2bb067058dec17b0f5596aca9296'
BUSYBOX_IMAGE='busybox:1.37.0-uclibc@sha256:39e0df8c4d65953b55c344f017e1ff2e0031a7454b3c24e6b76d402f207e315a'
ROOT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)

usage() {
  cat <<'EOF'
Usage: deploy/migrate/atlas.sh <command>

Commands:
  version            Print the pinned Atlas Community CLI version.
  hash               Regenerate migrations/atlas.sum.
  validate           Validate migration directory checksums and SQL syntax.
  validate-semantic  Validate migrations against disposable PostgreSQL 18.4.
  build              Build the non-root NexusRelay migration image.
  apply              Apply migrations using DATABASE_MIGRATION_PASSWORD_FILE.

apply requires DATABASE_HOST, DATABASE_PORT, DATABASE_NAME,
DATABASE_MIGRATION_USER, DATABASE_MIGRATION_PASSWORD_FILE, and DATABASE_SSLMODE.
build requires non-secret VERSION and REVISION provenance values.
The image entrypoint validates all migration settings and the mounted password
file before Atlas reads it. The password is not placed in an environment
variable or command argument.
Set NEXUSRELAY_MIGRATE_DOCKER_NETWORK only when this host launcher must join an
existing Docker network. Compose should invoke the image on its service network.
EOF
}

atlas_readonly() {
  docker run --rm \
    --user 65532:65532 \
    --network none \
    --read-only \
    --tmpfs /tmp:rw,noexec,nosuid,nodev,size=16m \
    --mount "type=bind,src=$ROOT_DIR,dst=/workspace,readonly" \
    --workdir /workspace \
    "$ATLAS_IMAGE" "$@"
}

cleanup_semantic_database() {
  docker rm --force "$semantic_container" >/dev/null 2>&1 || true
  docker network rm "$semantic_network" >/dev/null 2>&1 || true
}

validate_build_metadata() {
  case "${VERSION-}" in
    ''|*[!A-Za-z0-9._+-]*)
      printf 'migration build error: VERSION must match [A-Za-z0-9._+-]+\n' >&2
      exit 2
      ;;
  esac
  case "${REVISION-}" in
    [A-Za-z0-9]*) ;;
    *)
      printf 'migration build error: REVISION must start with an ASCII letter or number\n' >&2
      exit 2
      ;;
  esac
  case "$REVISION" in
    *[!A-Za-z0-9._-]*)
      printf 'migration build error: REVISION must match [A-Za-z0-9][A-Za-z0-9._-]{0,127}\n' >&2
      exit 2
      ;;
  esac
  if [ "${#REVISION}" -gt 128 ]; then
    printf 'migration build error: REVISION must be at most 128 characters\n' >&2
    exit 2
  fi
}

case "${1-}" in
  version)
    atlas_readonly version
    ;;
  hash)
    docker run --rm \
      --user "$(id -u):$(id -g)" \
      --network none \
      --read-only \
      --tmpfs /tmp:rw,noexec,nosuid,nodev,size=16m \
      --mount "type=bind,src=$ROOT_DIR/migrations,dst=/migrations" \
      "$ATLAS_IMAGE" migrate hash --dir file:///migrations
    ;;
  validate)
    atlas_readonly migrate validate --dir file://migrations
    ;;
  validate-semantic)
    semantic_network="nexusrelay-atlas-validate-$$"
    semantic_container="nexusrelay-atlas-postgres-$$"
    trap cleanup_semantic_database EXIT HUP INT TERM
    docker network create "$semantic_network" >/dev/null
    docker run --detach --rm \
      --name "$semantic_container" \
      --network "$semantic_network" \
      --env POSTGRES_DB=dev \
      --env POSTGRES_USER=postgres \
      --env POSTGRES_HOST_AUTH_METHOD=trust \
      "$POSTGRES_IMAGE" >/dev/null
    attempts=0
    until docker exec "$semantic_container" pg_isready --username postgres --dbname dev >/dev/null 2>&1; do
      attempts=$((attempts + 1))
      if [ "$attempts" -ge 60 ]; then
        printf 'semantic validation error: PostgreSQL 18 did not become ready\n' >&2
        exit 1
      fi
      sleep 1
    done
    attempts=0
    until docker run --rm --network "$semantic_network" "$BUSYBOX_IMAGE" nc -z "$semantic_container" 5432 >/dev/null 2>&1; do
      attempts=$((attempts + 1))
      if [ "$attempts" -ge 60 ]; then
        printf 'semantic validation error: PostgreSQL 18 was not reachable on its Docker network\n' >&2
        exit 1
      fi
      sleep 1
    done
    docker run --rm \
      --user 65532:65532 \
      --network "$semantic_network" \
      --read-only \
      --tmpfs /tmp:rw,noexec,nosuid,nodev,size=16m \
      --mount "type=bind,src=$ROOT_DIR,dst=/workspace,readonly" \
      --workdir /workspace \
      "$ATLAS_IMAGE" migrate validate \
      --dir file://migrations \
      --dev-url "postgres://postgres@$semantic_container:5432/dev?search_path=public&sslmode=disable"
    cleanup_semantic_database
    trap - EXIT HUP INT TERM
    ;;
  build)
    validate_build_metadata
    docker build --pull \
      --build-arg "VERSION=$VERSION" \
      --build-arg "REVISION=$REVISION" \
      --tag nexusrelay-migrate:local \
      --file "$ROOT_DIR/deploy/migrate/Dockerfile" \
      "$ROOT_DIR"
    ;;
  apply)
    if [ -z "${DATABASE_MIGRATION_PASSWORD_FILE-}" ]; then
      printf 'migration launcher error: DATABASE_MIGRATION_PASSWORD_FILE is required for the bind mount\n' >&2
      exit 2
    fi
    set --
    if [ -n "${NEXUSRELAY_MIGRATE_DOCKER_NETWORK-}" ]; then
      set -- --network "$NEXUSRELAY_MIGRATE_DOCKER_NETWORK"
    fi
    docker run --rm "$@" \
      --user 65532:65532 \
      --read-only \
      --tmpfs /tmp:rw,noexec,nosuid,nodev,size=16m \
      --mount "type=bind,src=$DATABASE_MIGRATION_PASSWORD_FILE,dst=/run/secrets/database_migration_password,readonly" \
      --env DATABASE_HOST \
      --env DATABASE_PORT \
      --env DATABASE_NAME \
      --env DATABASE_MIGRATION_USER \
      --env DATABASE_MIGRATION_PASSWORD_FILE=/run/secrets/database_migration_password \
      --env DATABASE_SSLMODE \
      nexusrelay-migrate:local
    ;;
  -h|--help|help|'')
    usage
    ;;
  *)
    printf 'unknown migration command: %s\n' "$1" >&2
    usage >&2
    exit 2
    ;;
esac
