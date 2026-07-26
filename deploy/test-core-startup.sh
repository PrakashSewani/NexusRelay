#!/bin/sh
set -eu

ROOT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
project="nexusrelay-core-test-$$"

cleanup() {
  docker compose --project-name "$project" --env-file "$ROOT_DIR/.env" -f "$ROOT_DIR/deploy/compose.yaml" --profile core down --volumes --remove-orphans >/dev/null 2>&1 || true
}
trap cleanup EXIT HUP INT TERM

"$ROOT_DIR/deploy/local-core.sh" init
docker compose --project-name "$project" --env-file "$ROOT_DIR/.env" -f "$ROOT_DIR/deploy/compose.yaml" --profile core up --build --wait --wait-timeout 180

COMPOSE_PROJECT_NAME="$project" "$ROOT_DIR/deploy/local-core.sh" check

printf '%s\n' 'clean core startup contract valid'
