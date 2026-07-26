#!/bin/sh
set -eu
set +x

ROOT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
IMAGE='busybox:1.37.0-uclibc@sha256:39e0df8c4d65953b55c344f017e1ff2e0031a7454b3c24e6b76d402f207e315a'
test_id="nexusrelay-secret-publication-$$"
source_dir=$(mktemp -d)
volumes='postgres migrate redis gateway control-plane worker'

cleanup() {
  for name in $volumes; do
    docker volume rm "$test_id-$name" >/dev/null 2>&1 || true
  done
  rm -rf "$source_dir"
}
trap cleanup EXIT HUP INT TERM

go run "$ROOT_DIR/internal/config/cmd/generate-dev-secrets" --output-dir "$source_dir/inventory" >/dev/null
for name in $volumes; do
  docker volume create "$test_id-$name" >/dev/null
done

run_publisher() {
  docker run --rm --network none --read-only --user 0:0 \
    --mount "type=bind,src=$ROOT_DIR/deploy/secrets/publish.sh,dst=/publish.sh,readonly" \
    --mount "type=bind,src=$source_dir/inventory,dst=/source,readonly" \
    --mount "type=volume,src=$test_id-postgres,dst=/target/postgres" \
    --mount "type=volume,src=$test_id-migrate,dst=/target/migrate" \
    --mount "type=volume,src=$test_id-redis,dst=/target/redis" \
    --mount "type=volume,src=$test_id-gateway,dst=/target/gateway" \
    --mount "type=volume,src=$test_id-control-plane,dst=/target/control-plane" \
    --mount "type=volume,src=$test_id-worker,dst=/target/worker" \
    "$IMAGE" /bin/sh /publish.sh
}

run_publisher >/dev/null
run_publisher >/dev/null

inspect_volume() {
  volume=$1
  expected=$2
  docker run --rm --network none --read-only \
    --mount "type=volume,src=$test_id-$volume,dst=/target,readonly" \
    "$IMAGE" sh -ec '
      actual=$(for path in /target/*; do [ -e "$path" ] || continue; basename "$path"; done | sort | tr "\n" " ")
      [ "$actual" = "$1" ] || exit 1
      for path in /target/*; do [ "$(stat -c "%a|%u:%g" "$path")" = "$2" ]; done
    ' sh "$expected" "$3"
}

inspect_volume postgres 'postgres_cluster_admin_password postgres_control_plane_password postgres_gateway_password postgres_migration_password postgres_worker_password ' '400|999:999'
inspect_volume migrate 'postgres_migration_password ' '400|65532:65532'
inspect_volume redis 'redis_password ' '400|999:999'
inspect_volume gateway 'api_key_pepper_ring postgres_gateway_password provider_master_keyring redis_url ' '400|65532:65532'
inspect_volume control-plane 'api_key_pepper_ring csrf_secret_ring postgres_control_plane_password provider_master_keyring redis_url session_secret ' '400|65532:65532'
inspect_volume worker 'postgres_worker_password provider_master_keyring redis_url ' '400|65532:65532'

docker run --rm --network none --user 0:0 \
  --mount "type=volume,src=$test_id-worker,dst=/target" \
  "$IMAGE" sh -c 'printf changed >/target/redis_url && chown 65532:65532 /target/redis_url && chmod 0400 /target/redis_url'
if run_publisher >/dev/null 2>&1; then
  printf 'publisher accepted a changed target secret\n' >&2
  exit 1
fi

docker run --rm --network none --user 0:0 \
  --mount "type=volume,src=$test_id-worker,dst=/target" \
  "$IMAGE" sh -c 'rm /target/redis_url && cp /target/provider_master_keyring /target/redis_url && chown 65532:65532 /target/redis_url && chmod 0400 /target/redis_url && cp /target/redis_url /target/.extra && chown 65532:65532 /target/.extra && chmod 0400 /target/.extra'
if run_publisher >/dev/null 2>&1; then
  printf 'publisher accepted an extra target file\n' >&2
  exit 1
fi

printf 'protected secret publication contract valid\n'
