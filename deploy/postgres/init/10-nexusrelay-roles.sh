#!/bin/sh
set -eu
set +x

fail() {
  printf 'PostgreSQL initialization error: %s\n' "$1" >&2
  exit 1
}

read_secret_path() {
  setting=$1
  eval "path=\${$setting-}"
  [ -n "$path" ] || fail "$setting is required"
  case "$path" in
    /*) ;;
    *) fail "$setting must be an absolute path" ;;
  esac
  [ -f "$path" ] || fail "$setting must name a regular file"
  [ ! -L "$path" ] || fail "$setting must not name a symbolic link"
  [ -s "$path" ] || fail "$setting must not be empty"
  mode=$(stat -c %a "$path")
  case "$mode" in
    400|600) ;;
    *) fail "$setting must not be accessible by group or other users" ;;
  esac
  od -An -v -tu1 "$path" | awk '
    {
      for (i = 1; i <= NF; i++) bytes[++n] = $i
    }
    END {
      if (n < 1 || n > 4098) exit 1
      content = n
      if (bytes[content] == 10) {
        content--
        if (content > 0 && bytes[content] == 13) content--
      }
      if (content < 1 || content > 4096) exit 1
      if (bytes[content] == 10 || bytes[content] == 13) exit 1
      if (bytes[1] == 9 || bytes[1] == 10 || bytes[1] == 11 || bytes[1] == 12 || bytes[1] == 13 || bytes[1] == 32) exit 1
      if (bytes[content] == 9 || bytes[content] == 10 || bytes[content] == 11 || bytes[content] == 12 || bytes[content] == 13 || bytes[content] == 32) exit 1
    }
  ' || fail "$setting has an invalid size, newline, or surrounding-whitespace format"
}

POSTGRES_PASSWORD_FILE=${POSTGRES_PASSWORD_FILE:-/run/secrets/postgres_cluster_admin_password}

for setting in \
  DATABASE_MIGRATION_PASSWORD_FILE \
  DATABASE_GATEWAY_PASSWORD_FILE \
  DATABASE_CONTROL_PLANE_PASSWORD_FILE \
  DATABASE_WORKER_PASSWORD_FILE
do
  read_secret_path "$setting"
done
read_secret_path POSTGRES_PASSWORD_FILE

[ "${POSTGRES_USER-}" = nexusrelay_cluster_admin ] || fail 'POSTGRES_USER must be nexusrelay_cluster_admin'
[ "${DATABASE_MIGRATION_USER-}" = nexusrelay_migration ] || fail 'DATABASE_MIGRATION_USER must be nexusrelay_migration'
[ "${DATABASE_GATEWAY_USER-}" = nexusrelay_gateway ] || fail 'DATABASE_GATEWAY_USER must be nexusrelay_gateway'
[ "${DATABASE_CONTROL_PLANE_USER-}" = nexusrelay_control_plane ] || fail 'DATABASE_CONTROL_PLANE_USER must be nexusrelay_control_plane'
[ "${DATABASE_WORKER_USER-}" = nexusrelay_worker ] || fail 'DATABASE_WORKER_USER must be nexusrelay_worker'
[ "${POSTGRES_DB-}" = nexusrelay ] || fail 'POSTGRES_DB must be nexusrelay'
[ "${DATABASE_NAME-}" = "$POSTGRES_DB" ] || fail 'DATABASE_NAME must equal POSTGRES_DB'

password_files="
$POSTGRES_PASSWORD_FILE
$DATABASE_MIGRATION_PASSWORD_FILE
$DATABASE_GATEWAY_PASSWORD_FILE
$DATABASE_CONTROL_PLANE_PASSWORD_FILE
$DATABASE_WORKER_PASSWORD_FILE
"
unique_paths=$(printf '%s' "$password_files" | sed '/^$/d' | sort -u | wc -l | tr -d ' ')
[ "$unique_paths" = 5 ] || fail 'database password file paths must be distinct'

normalized_secret() {
  od -An -v -tu1 "$1" | awk '
    {
      for (i = 1; i <= NF; i++) bytes[++n] = $i
    }
    END {
      if (bytes[n] == 10) {
        n--
        if (n > 0 && bytes[n] == 13) n--
      }
      for (i = 1; i <= n; i++) printf "%c", bytes[i]
    }
  '
}

assert_distinct_values() {
  left=$(normalized_secret "$1")
  right=$(normalized_secret "$2")
  if [ "$left" = "$right" ]; then
    fail 'database password values must be distinct'
  fi
  unset left right
}
assert_distinct_values "$POSTGRES_PASSWORD_FILE" "$DATABASE_MIGRATION_PASSWORD_FILE"
assert_distinct_values "$POSTGRES_PASSWORD_FILE" "$DATABASE_GATEWAY_PASSWORD_FILE"
assert_distinct_values "$POSTGRES_PASSWORD_FILE" "$DATABASE_CONTROL_PLANE_PASSWORD_FILE"
assert_distinct_values "$POSTGRES_PASSWORD_FILE" "$DATABASE_WORKER_PASSWORD_FILE"
assert_distinct_values "$DATABASE_MIGRATION_PASSWORD_FILE" "$DATABASE_GATEWAY_PASSWORD_FILE"
assert_distinct_values "$DATABASE_MIGRATION_PASSWORD_FILE" "$DATABASE_CONTROL_PLANE_PASSWORD_FILE"
assert_distinct_values "$DATABASE_MIGRATION_PASSWORD_FILE" "$DATABASE_WORKER_PASSWORD_FILE"
assert_distinct_values "$DATABASE_GATEWAY_PASSWORD_FILE" "$DATABASE_CONTROL_PLANE_PASSWORD_FILE"
assert_distinct_values "$DATABASE_GATEWAY_PASSWORD_FILE" "$DATABASE_WORKER_PASSWORD_FILE"
assert_distinct_values "$DATABASE_CONTROL_PLANE_PASSWORD_FILE" "$DATABASE_WORKER_PASSWORD_FILE"

password_sql=/tmp/nexusrelay-passwords.sql
: >"$password_sql"
chmod 0600 "$password_sql"
cleanup() {
  rm -f "$password_sql"
}
trap cleanup EXIT HUP INT TERM

append_password_sql() {
  role=$1
  password=$(normalized_secret "$2")
  escaped=$(printf '%s' "$password" | sed "s/'/''/g")
  printf "ALTER ROLE %s PASSWORD '%s';\n" "$role" "$escaped" >>"$password_sql"
  unset password escaped
}
append_password_sql nexusrelay_migration "$DATABASE_MIGRATION_PASSWORD_FILE"
append_password_sql nexusrelay_gateway "$DATABASE_GATEWAY_PASSWORD_FILE"
append_password_sql nexusrelay_control_plane "$DATABASE_CONTROL_PLANE_PASSWORD_FILE"
append_password_sql nexusrelay_worker "$DATABASE_WORKER_PASSWORD_FILE"

# Passwords are read by the PostgreSQL server from fixed mounted paths and are
# quoted with format(%L). They never enter SQL source, process arguments, or logs.
psql --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" --set ON_ERROR_STOP=1 <<'SQL'
CREATE ROLE nexusrelay_schema_owner
  NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT NOREPLICATION NOBYPASSRLS;
CREATE ROLE nexusrelay_security_definer_owner
  NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT NOREPLICATION NOBYPASSRLS;
CREATE ROLE nexusrelay_gateway_runtime
  NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT NOREPLICATION NOBYPASSRLS;
CREATE ROLE nexusrelay_control_plane_runtime
  NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT NOREPLICATION NOBYPASSRLS;
CREATE ROLE nexusrelay_worker_runtime
  NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT NOREPLICATION NOBYPASSRLS;

CREATE ROLE nexusrelay_migration
  LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT NOREPLICATION NOBYPASSRLS;
CREATE ROLE nexusrelay_gateway
  LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE INHERIT NOREPLICATION NOBYPASSRLS;
CREATE ROLE nexusrelay_control_plane
  LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE INHERIT NOREPLICATION NOBYPASSRLS;
CREATE ROLE nexusrelay_worker
  LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE INHERIT NOREPLICATION NOBYPASSRLS;

\i /tmp/nexusrelay-passwords.sql

GRANT nexusrelay_schema_owner TO nexusrelay_migration
  WITH ADMIN FALSE, INHERIT FALSE, SET TRUE;
GRANT nexusrelay_security_definer_owner TO nexusrelay_migration
  WITH ADMIN FALSE, INHERIT FALSE, SET TRUE;
GRANT nexusrelay_gateway_runtime TO nexusrelay_gateway
  WITH ADMIN FALSE, INHERIT TRUE, SET FALSE;
GRANT nexusrelay_control_plane_runtime TO nexusrelay_control_plane
  WITH ADMIN FALSE, INHERIT TRUE, SET FALSE;
GRANT nexusrelay_worker_runtime TO nexusrelay_worker
  WITH ADMIN FALSE, INHERIT TRUE, SET FALSE;

REVOKE CONNECT, CREATE, TEMPORARY ON DATABASE nexusrelay FROM PUBLIC;
REVOKE CREATE ON SCHEMA public FROM PUBLIC;
GRANT CONNECT ON DATABASE nexusrelay TO
  nexusrelay_migration,
  nexusrelay_gateway,
  nexusrelay_control_plane,
  nexusrelay_worker;
GRANT TEMPORARY ON DATABASE nexusrelay TO nexusrelay_migration;

CREATE SCHEMA nexusrelay AUTHORIZATION nexusrelay_schema_owner;
CREATE SCHEMA nexusrelay_migration AUTHORIZATION nexusrelay_migration;
REVOKE ALL ON SCHEMA nexusrelay, nexusrelay_migration FROM PUBLIC;
GRANT USAGE ON SCHEMA nexusrelay TO nexusrelay_migration;
SQL

touch "$PGDATA/.nexusrelay-initialized"
