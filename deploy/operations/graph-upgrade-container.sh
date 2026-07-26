#!/bin/sh
set -eu
set +x
umask 077

candidate=$(mktemp -d "/evidence/.$CHANGE_ID.XXXXXX")
trap 'rm -rf "$candidate"' EXIT HUP INT TERM
password=$(awk 'BEGIN { RS="\0" } { sub(/\r?\n$/, ""); print }' /run/secrets/postgres_cluster_admin_password)
escaped=$(printf '%s' "$password" | sed 's/\\/\\\\/g; s/:/\\:/g')
pgpass=$(mktemp)
printf '*:*:*:%s:%s\n' "$POSTGRES_USER" "$escaped" >"$pgpass"
chmod 0600 "$pgpass"
unset password escaped
export PGPASSFILE="$pgpass"
base="--no-psqlrc --host=$DATABASE_HOST --port=$DATABASE_PORT --username=$POSTGRES_USER --dbname=$DATABASE_NAME --no-password --set=ON_ERROR_STOP=1"

cp /request/request.env "$candidate/request.env"
cp /request/mutation.sql "$candidate/mutation.sql"
psql $base --no-align --tuples-only --file /opt/nexusrelay/verify-role-graph.sql >"$candidate/before.txt"
psql $base --single-transaction --file /request/mutation.sql >/dev/null
printf 'status=committed\n' >"$candidate/mutation-result.env"
psql $base --no-align --tuples-only --file /opt/nexusrelay/verify-role-graph.sql >"$candidate/after.txt"
cat >"$candidate/runner.env" <<EOF
FORMAT=nexusrelay-graph-upgrade-evidence-v1
CHANGE_ID=$CHANGE_ID
EXECUTED_AS=$POSTGRES_USER
ATLAS_EXECUTED=false
EOF
(cd "$candidate" && sha256sum after.txt before.txt mutation-result.env mutation.sql request.env runner.env >SHA256SUMS)
chmod 0600 "$candidate"/*
chmod 0700 "$candidate"
rm -f "$pgpass"
mv "$candidate" "/evidence/$CHANGE_ID"
trap - EXIT HUP INT TERM
