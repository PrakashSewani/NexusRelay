#!/bin/sh
set -eu
set +x
umask 077

fail() {
  printf 'secret publication failed: %s\n' "$1" >&2
  exit 1
}

for interface_path in /sys/class/net/*; do
  [ "${interface_path##*/}" = lo ] || fail 'publisher must run with network disabled'
done

[ -d /source ] && [ ! -L /source ] || fail 'source root must be a directory'
[ "$(stat -c %a /source)" = 700 ] || fail 'source root must have mode 0700'

source_files='api_key_pepper_ring
csrf_secret_ring
postgres_cluster_admin_password
postgres_control_plane_password
postgres_gateway_password
postgres_migration_password
postgres_worker_password
provider_master_keyring
redis_password
redis_url
session_secret'

actual_source=$(find /source -mindepth 1 -maxdepth 1 -print | while IFS= read -r path; do basename "$path"; done | sort)
[ "$actual_source" = "$source_files" ] || fail 'source inventory differs from the exact allowlist'
for name in $source_files; do
  path=/source/$name
  [ -f "$path" ] && [ ! -L "$path" ] || fail 'source contains a non-regular allowlisted entry'
  [ "$(stat -c %a "$path")" = 600 ] || fail 'source files must have mode 0600'
done

preflight_target() {
  target=$1
  uid=$2
  gid=$3
  shift 3
  [ -d "$target" ] && [ ! -L "$target" ] || fail 'a fixed target root is unavailable'
  actual=$(find "$target" -mindepth 1 -maxdepth 1 -print | while IFS= read -r path; do basename "$path"; done | sort)
  expected=$(printf '%s\n' "$@" | sort)
  if [ -n "$actual" ]; then
    [ "$actual" = "$expected" ] || fail 'target contains missing or extra files'
    for name in "$@"; do
      path=$target/$name
      [ -f "$path" ] && [ ! -L "$path" ] || fail 'target contains a non-regular entry'
      cmp -s "/source/$name" "$path" || fail 'published secret differs from source'
      [ "$(stat -c %a "$path")" = 400 ] || fail 'published secret mode differs from 0400'
      [ "$(stat -c %u:%g "$path")" = "$uid:$gid" ] || fail 'published secret ownership differs from the fixed service identity'
    done
  fi
}

publish_target() {
  target=$1
  uid=$2
  gid=$3
  shift 3
  [ -z "$(find "$target" -mindepth 1 -maxdepth 1 -print -quit)" ] || return 0
  for name in "$@"; do
    cp "/source/$name" "$target/$name"
    chmod 0400 "$target/$name"
    chown "$uid:$gid" "$target/$name"
  done
}

preflight_target /target/postgres 999 999 postgres_cluster_admin_password postgres_migration_password postgres_gateway_password postgres_control_plane_password postgres_worker_password
preflight_target /target/migrate 65532 65532 postgres_migration_password
preflight_target /target/redis 999 999 redis_password
preflight_target /target/gateway 65532 65532 postgres_gateway_password redis_url provider_master_keyring api_key_pepper_ring
preflight_target /target/control-plane 65532 65532 postgres_control_plane_password redis_url provider_master_keyring api_key_pepper_ring session_secret csrf_secret_ring
preflight_target /target/worker 65532 65532 postgres_worker_password redis_url provider_master_keyring

publish_target /target/postgres 999 999 postgres_cluster_admin_password postgres_migration_password postgres_gateway_password postgres_control_plane_password postgres_worker_password
publish_target /target/migrate 65532 65532 postgres_migration_password
publish_target /target/redis 999 999 redis_password
publish_target /target/gateway 65532 65532 postgres_gateway_password redis_url provider_master_keyring api_key_pepper_ring
publish_target /target/control-plane 65532 65532 postgres_control_plane_password redis_url provider_master_keyring api_key_pepper_ring session_secret csrf_secret_ring
publish_target /target/worker 65532 65532 postgres_worker_password redis_url provider_master_keyring

printf 'protected service secret volumes are ready\n'
