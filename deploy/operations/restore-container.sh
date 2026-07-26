#!/bin/sh
set -eu
set +x
umask 077

password=$(awk 'BEGIN { RS="\0" } { sub(/\r?\n$/, ""); print }' "$RECOVERY_ADMIN_PASSWORD_FILE")
[ -n "$password" ] || { printf 'recovery administrator password is empty\n' >&2; exit 1; }
escaped=$(printf '%s' "$password" | sed 's/\\/\\\\/g; s/:/\\:/g')
pgpass=$(mktemp)
trap 'rm -f "$pgpass"' EXIT HUP INT TERM
printf '*:*:*:%s:%s\n' "$RECOVERY_ADMIN_USER" "$escaped" >"$pgpass"
chmod 0600 "$pgpass"
unset password escaped
export PGPASSFILE="$pgpass"

base="--host=$DATABASE_HOST --port=$DATABASE_PORT --username=$RECOVERY_ADMIN_USER --no-password"
psql $base --dbname postgres --set ON_ERROR_STOP=1 --file /backup/roles.sql >/dev/null
pg_restore $base --dbname postgres --exit-on-error --create /backup/database.dump >/dev/null

export DATABASE_NAME=nexusrelay
/opt/nexusrelay/apply-login-passwords.sh
psql $base --dbname nexusrelay --no-align --tuples-only --set ON_ERROR_STOP=1 --file /opt/nexusrelay/verify-role-graph.sql >/dev/null
printf 'logical database restore verified\n'
