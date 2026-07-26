#!/bin/sh
set -eu
set +x
umask 077

fail() { printf 'PostgreSQL credential recovery error: %s\n' "$1" >&2; exit 1; }
: "${RECOVERY_ADMIN_USER:?RECOVERY_ADMIN_USER is required}"
: "${RECOVERY_ADMIN_PASSWORD_FILE:?RECOVERY_ADMIN_PASSWORD_FILE is required}"
: "${DATABASE_HOST:?DATABASE_HOST is required}"
: "${DATABASE_PORT:?DATABASE_PORT is required}"
: "${DATABASE_NAME:?DATABASE_NAME is required}"

case "$RECOVERY_ADMIN_USER" in nexusrelay_cluster_admin|postgres) ;; *) fail 'RECOVERY_ADMIN_USER must be an approved local recovery administrator' ;; esac

read_secret() {
  setting=$1
  eval "path=\${$setting-}"
  [ -f "$path" ] && [ ! -L "$path" ] || fail "$setting must be a regular non-symlink file"
  value=$(awk 'BEGIN { RS="\0" } { sub(/\r?\n$/, ""); print }' "$path")
  [ -n "$value" ] || fail "$setting is empty"
  case "$value" in *'\n'*|*'\r'*) fail "$setting contains a line break" ;; esac
  printf '%s' "$value"
}

admin_password=$(read_secret RECOVERY_ADMIN_PASSWORD_FILE)
escaped=$(printf '%s' "$admin_password" | sed 's/\\/\\\\/g; s/:/\\:/g')
pgpass=$(mktemp)
sql=$(mktemp)
trap 'rm -f "$pgpass" "$sql"' EXIT HUP INT TERM
printf '*:*:*:%s:%s\n' "$RECOVERY_ADMIN_USER" "$escaped" >"$pgpass"
chmod 0600 "$pgpass" "$sql"
unset admin_password escaped

append_password() {
  role=$1
  setting=$2
  value=$(read_secret "$setting")
  escaped_value=$(printf '%s' "$value" | sed "s/'/''/g")
  printf "ALTER ROLE %s PASSWORD '%s';\n" "$role" "$escaped_value" >>"$sql"
  unset value escaped_value
}

append_password nexusrelay_cluster_admin POSTGRES_PASSWORD_FILE
append_password nexusrelay_migration DATABASE_MIGRATION_PASSWORD_FILE
append_password nexusrelay_gateway DATABASE_GATEWAY_PASSWORD_FILE
append_password nexusrelay_control_plane DATABASE_CONTROL_PLANE_PASSWORD_FILE
append_password nexusrelay_worker DATABASE_WORKER_PASSWORD_FILE

PGPASSFILE="$pgpass" psql \
  --host "$DATABASE_HOST" --port "$DATABASE_PORT" \
  --username "$RECOVERY_ADMIN_USER" --dbname "$DATABASE_NAME" \
  --no-password --set ON_ERROR_STOP=1 --file "$sql" >/dev/null
printf 'PostgreSQL login credentials re-established from protected recovery files\n'
