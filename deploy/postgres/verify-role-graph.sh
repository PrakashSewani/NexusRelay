#!/bin/sh
set -eu
set +x
umask 077

fail() {
  printf 'PostgreSQL role verification error: %s\n' "$1" >&2
  exit 1
}

for setting in DATABASE_HOST DATABASE_PORT DATABASE_NAME DATABASE_VERIFY_USER DATABASE_VERIFY_PASSWORD_FILE; do
  eval "value=\${$setting-}"
  [ -n "$value" ] || fail "$setting is required"
done
[ "${DATABASE_NAME}" = nexusrelay ] || fail 'DATABASE_NAME must be nexusrelay'
[ -f "$DATABASE_VERIFY_PASSWORD_FILE" ] && [ ! -L "$DATABASE_VERIFY_PASSWORD_FILE" ] || fail 'DATABASE_VERIFY_PASSWORD_FILE must be a regular non-symlink file'
[ -f "${ROLE_GRAPH_SQL:-/opt/nexusrelay/verify-role-graph.sql}" ] || fail 'ROLE_GRAPH_SQL is unavailable'

password=$(awk 'BEGIN { RS="\0" } { sub(/\r?\n$/, ""); print }' "$DATABASE_VERIFY_PASSWORD_FILE")
[ -n "$password" ] || fail 'DATABASE_VERIFY_PASSWORD_FILE is empty'
case "$password" in *'\n'*|*'\r'*) fail 'database password must not contain line breaks' ;; esac
escaped=$(printf '%s' "$password" | sed 's/\\/\\\\/g; s/:/\\:/g')
pgpass=$(mktemp)
trap 'rm -f "$pgpass"' EXIT HUP INT TERM
printf '*:*:*:%s:%s\n' "$DATABASE_VERIFY_USER" "$escaped" >"$pgpass"
chmod 0600 "$pgpass"
unset password escaped

PGPASSFILE="$pgpass" psql \
  --host "$DATABASE_HOST" \
  --port "$DATABASE_PORT" \
  --username "$DATABASE_VERIFY_USER" \
  --dbname "$DATABASE_NAME" \
  --no-password \
  --no-align \
  --tuples-only \
  --set ON_ERROR_STOP=1 \
  --file "${ROLE_GRAPH_SQL:-/opt/nexusrelay/verify-role-graph.sql}"
