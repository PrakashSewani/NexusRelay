#!/bin/sh
set -eu
set +x
umask 077

POSTGRES_IMAGE='postgres:18.4-bookworm@sha256:1961f96e6029a02c3812d7cb329a3b03a3ac2bb067058dec17b0f5596aca9296'
ROOT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
fail() { printf 'graph upgrade error: %s\n' "$1" >&2; exit 1; }
mode_of() { stat -c %a "$1" 2>/dev/null || stat -f %Lp "$1"; }

for setting in GRAPH_UPGRADE_REQUEST GRAPH_UPGRADE_MUTATION_SQL GRAPH_UPGRADE_EVIDENCE_ROOT DATABASE_HOST DATABASE_PORT DATABASE_NAME POSTGRES_USER POSTGRES_PASSWORD_FILE; do
  eval "value=\${$setting-}"
  [ -n "$value" ] || fail "$setting is required"
done
[ "${CONFIRM_SERVICES_STOPPED-}" = yes ] || fail 'CONFIRM_SERVICES_STOPPED=yes is required'
[ "$POSTGRES_USER" = nexusrelay_cluster_admin ] || fail 'POSTGRES_USER must be nexusrelay_cluster_admin'
[ "$DATABASE_NAME" = nexusrelay ] || fail 'DATABASE_NAME must be nexusrelay'
for path in "$GRAPH_UPGRADE_REQUEST" "$GRAPH_UPGRADE_MUTATION_SQL" "$POSTGRES_PASSWORD_FILE"; do [ -f "$path" ] && [ ! -L "$path" ] || fail 'request, SQL, and password inputs must be regular non-symlink files'; done
[ -d "$GRAPH_UPGRADE_EVIDENCE_ROOT" ] && [ ! -L "$GRAPH_UPGRADE_EVIDENCE_ROOT" ] || fail 'GRAPH_UPGRADE_EVIDENCE_ROOT must be an existing non-symlink directory'
[ "$(mode_of "$GRAPH_UPGRADE_EVIDENCE_ROOT")" = 700 ] || fail 'GRAPH_UPGRADE_EVIDENCE_ROOT must have mode 0700'

get_field() { sed -n "s/^$1=//p" "$GRAPH_UPGRADE_REQUEST"; }
expected_fields='APPROVED_AT_UTC
APPROVER
ATLAS_VERSION
CHANGE_ID
FORMAT
FROM_RELEASE
MUTATION_SQL_SHA256
POSTGRESQL_MAJOR
POSTGRESQL_MINOR
ROLE_GRAPH_CONTRACT
TO_RELEASE'
actual_fields=$(sed -n 's/^\([A-Z][A-Z0-9_]*\)=.*$/\1/p' "$GRAPH_UPGRADE_REQUEST" | sort)
[ "$actual_fields" = "$expected_fields" ] || fail 'request fields differ from the exact contract'
[ "$(get_field FORMAT)" = nexusrelay-graph-upgrade-request-v1 ] || fail 'request format is unsupported'
[ "$(get_field POSTGRESQL_MAJOR)" = 18 ] || fail 'PostgreSQL major upgrades require a release-specific plan and are blocked here'
[ "$(get_field POSTGRESQL_MINOR)" = 18.4 ] || fail 'request PostgreSQL minor differs from the pinned release'
[ "$(get_field ATLAS_VERSION)" = 1.2.3-community ] || fail 'request Atlas version differs from the pinned release'
[ "$(get_field ROLE_GRAPH_CONTRACT)" = postgresql-18-v1 ] || fail 'request role-graph contract differs from the release verifier'
[ -n "$(get_field FROM_RELEASE)" ] && [ -n "$(get_field TO_RELEASE)" ] || fail 'release transition is required'
[ -n "$(get_field APPROVER)" ] && [ -n "$(get_field APPROVED_AT_UTC)" ] || fail 'approval identity and UTC time are required'
change_id=$(get_field CHANGE_ID)
case "$change_id" in [A-Za-z0-9]* ) ;; *) fail 'CHANGE_ID has invalid format' ;; esac
case "$change_id" in *[!A-Za-z0-9._-]* ) fail 'CHANGE_ID has invalid format' ;; esac
expected_sql=$(get_field MUTATION_SQL_SHA256)
actual_sql=$(shasum -a 256 "$GRAPH_UPGRADE_MUTATION_SQL" | cut -d ' ' -f 1)
[ "$expected_sql" = "$actual_sql" ] || fail 'reviewed mutation SQL digest differs from the request'
if grep -E '^[[:space:]]*\\' "$GRAPH_UPGRADE_MUTATION_SQL" >/dev/null; then fail 'psql meta-commands are prohibited in graph mutation SQL'; fi
[ ! -e "$GRAPH_UPGRADE_EVIDENCE_ROOT/$change_id" ] || fail 'evidence bundle already exists'

case "${NEXUSRELAY_POSTGRES_DOCKER_NETWORK-}" in *[!A-Za-z0-9_.-]* ) fail 'NEXUSRELAY_POSTGRES_DOCKER_NETWORK contains unsupported characters' ;; esac
set --
if [ -n "${NEXUSRELAY_POSTGRES_DOCKER_NETWORK-}" ]; then set -- --network "$NEXUSRELAY_POSTGRES_DOCKER_NETWORK"; fi
docker run --rm "$@" --read-only --tmpfs /tmp:rw,noexec,nosuid,nodev,size=16m \
  --mount "type=bind,src=$ROOT_DIR/deploy/operations/graph-upgrade-container.sh,dst=/opt/nexusrelay/run.sh,readonly" \
  --mount "type=bind,src=$ROOT_DIR/deploy/postgres/verify-role-graph.sql,dst=/opt/nexusrelay/verify-role-graph.sql,readonly" \
  --mount "type=bind,src=$GRAPH_UPGRADE_REQUEST,dst=/request/request.env,readonly" \
  --mount "type=bind,src=$GRAPH_UPGRADE_MUTATION_SQL,dst=/request/mutation.sql,readonly" \
  --mount "type=bind,src=$POSTGRES_PASSWORD_FILE,dst=/run/secrets/postgres_cluster_admin_password,readonly" \
  --mount "type=bind,src=$GRAPH_UPGRADE_EVIDENCE_ROOT,dst=/evidence" \
  --env CHANGE_ID="$change_id" --env DATABASE_HOST --env DATABASE_PORT --env DATABASE_NAME --env POSTGRES_USER \
  "$POSTGRES_IMAGE" /opt/nexusrelay/run.sh
printf 'atomic protected graph-upgrade evidence bundle published for %s\n' "$change_id"
