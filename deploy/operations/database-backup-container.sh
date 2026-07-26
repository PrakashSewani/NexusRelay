#!/bin/sh
set -eu
set +x
umask 077

fail() { printf 'database backup error: %s\n' "$1" >&2; exit 1; }
target="/backup/$BACKUP_ID"
candidate=$(mktemp -d "/backup/.$BACKUP_ID.XXXXXX")
trap 'rm -rf "$candidate"' EXIT HUP INT TERM

password=$(awk 'BEGIN { RS="\0" } { sub(/\r?\n$/, ""); print }' "$POSTGRES_PASSWORD_FILE")
[ -n "$password" ] || fail 'cluster-admin password file is empty'
case "$password" in *'\n'*|*'\r'*) fail 'cluster-admin password must not contain line breaks' ;; esac
escaped=$(printf '%s' "$password" | sed 's/\\/\\\\/g; s/:/\\:/g')
pgpass=$(mktemp)
printf '*:*:*:%s:%s\n' "$POSTGRES_USER" "$escaped" >"$pgpass"
chmod 0600 "$pgpass"
unset password escaped
export PGPASSFILE="$pgpass"

psql_args="--host=$DATABASE_HOST --port=$DATABASE_PORT --username=$POSTGRES_USER --dbname=$DATABASE_NAME --no-password"
psql $psql_args --no-align --tuples-only --set ON_ERROR_STOP=1 --file /opt/nexusrelay/verify-role-graph.sql >"$candidate/role-graph.txt"
pg_dumpall --host "$DATABASE_HOST" --port "$DATABASE_PORT" --username "$POSTGRES_USER" --no-password --roles-only --no-role-passwords >"$candidate/roles.sql"
# PostgreSQL 18 emits the historical GRANTED BY superuser on membership edges.
# A different recovery superuser cannot replay that grantor identity when the
# historical grantor is not itself an ADMIN member. Restore needs the exact
# edge/options, not the source cluster's grantor OID, so normalize only the fixed
# NexusRelay bootstrap grantor and let the target recovery superuser grant it.
sed 's/ GRANTED BY nexusrelay_cluster_admin;/;/' "$candidate/roles.sql" >"$candidate/roles.normalized.sql"
mv "$candidate/roles.normalized.sql" "$candidate/roles.sql"
pg_dump --host "$DATABASE_HOST" --port "$DATABASE_PORT" --username "$POSTGRES_USER" --dbname "$DATABASE_NAME" --no-password --format=custom --create --file "$candidate/database.dump"
pg_restore --list "$candidate/database.dump" >/dev/null

postgres_version=$(psql $psql_args --no-align --tuples-only --set ON_ERROR_STOP=1 --command "SELECT current_setting('server_version')")
case "$postgres_version" in 18.*) ;; *) fail 'source is not PostgreSQL 18' ;; esac
cat >"$candidate/metadata.env" <<EOF
FORMAT=nexusrelay-logical-backup-v1
BACKUP_ID=$BACKUP_ID
NEXUSRELAY_RELEASE=$NEXUSRELAY_RELEASE
NEXUSRELAY_REVISION=$NEXUSRELAY_REVISION
ATLAS_VERSION=1.2.3-community
POSTGRESQL_VERSION=$postgres_version
ROLE_GRAPH_CONTRACT=postgresql-18-v1
PASSWORDS_INCLUDED=false
EOF
(cd "$candidate" && sha256sum database.dump metadata.env role-graph.txt roles.sql >SHA256SUMS)
chmod 0600 "$candidate"/*
chmod 0700 "$candidate"
rm -f "$pgpass"
mv "$candidate" "$target"
trap - EXIT HUP INT TERM
printf 'database backup artifact verified and published\n'
