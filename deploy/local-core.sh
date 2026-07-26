#!/bin/sh
set -eu

ROOT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
COMPOSE_FILE="$ROOT_DIR/deploy/compose.yaml"
ENV_FILE="$ROOT_DIR/.env"
SECRET_DIR="$ROOT_DIR/.local-secrets"

usage() {
  printf '%s\n' 'usage: deploy/local-core.sh init|up|check|down'
}

initialize() {
  if [ ! -e "$ENV_FILE" ]; then
    cp "$ROOT_DIR/.env.example" "$ENV_FILE"
    chmod 0600 "$ENV_FILE"
    printf '%s\n' 'created .env from .env.example'
  elif [ ! -f "$ENV_FILE" ] || [ -L "$ENV_FILE" ]; then
    printf '%s\n' '.env must be a regular file, not a symbolic link' >&2
    exit 1
  fi

  go run "$ROOT_DIR/internal/config/cmd/generate-dev-secrets" --output-dir "$SECRET_DIR"
  go run "$ROOT_DIR/internal/config/cmd/validate-deployment" --env-file "$ENV_FILE" --secret-root "$SECRET_DIR"
}

compose() {
  docker compose --env-file "$ENV_FILE" -f "$COMPOSE_FILE" --profile core "$@"
}

environment_value() {
  name=$1
  fallback=$2
  while IFS='=' read -r key value; do
    [ "$key" = "$name" ] || continue
    printf '%s\n' "$value"
    return
  done < "$ENV_FILE"
  printf '%s\n' "$fallback"
}

check() {
  expected='control-plane
gateway
postgres
proxy
redis
web
worker'
  actual=$(compose ps --status running --services | sort)
  [ "$actual" = "$expected" ] || {
    printf '%s\n' 'not all core services are running' >&2
    compose ps >&2
    exit 1
  }
  port=$(environment_value HTTP_PORT 8080)
  host=$(environment_value ADMIN_HOST localhost)
  response=$(curl --fail --silent --show-error --max-time 5 -H "Host: $host" "http://127.0.0.1:$port/")
  printf '%s' "$response" | grep -q 'NexusRelay' || {
    printf '%s\n' 'dashboard response did not contain the expected NexusRelay marker' >&2
    exit 1
  }
  printf 'NexusRelay core is healthy at http://%s:%s\n' "$host" "$port"
}

case "${1-}" in
  init)
    initialize
    ;;
  up)
    initialize
    compose up --build --wait --wait-timeout 180
    check
    ;;
  check)
    check
    ;;
  down)
    [ -f "$ENV_FILE" ] || { printf '%s\n' '.env does not exist; nothing to stop'; exit 0; }
    compose down --remove-orphans
    ;;
  *)
    usage >&2
    exit 2
    ;;
esac
