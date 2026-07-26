#!/bin/sh
set -eu
set +x

REDIS_IMAGE='redis:7.4.5-bookworm@sha256:90e7a336d044f1abc9e9dbc05d65566850896d11453bbd1dd0fb7e5059f0e8fb'
ROOT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
container="nexusrelay-redis-test-$$"
secret_volume="$container-secrets"
secret_dir=$(mktemp -d)
password=redis_password_0123456789abcdefXYZ

cleanup() {
  docker rm --force "$container" >/dev/null 2>&1 || true
  docker volume rm "$secret_volume" >/dev/null 2>&1 || true
  rm -rf "$secret_dir"
}
trap cleanup EXIT HUP INT TERM

printf '%s\r\n' "$password" >"$secret_dir/password"
chmod 0600 "$secret_dir/password"
docker volume create "$secret_volume" >/dev/null
docker run --rm --user 0:0 \
  --mount "type=bind,src=$secret_dir/password,dst=/source/password,readonly" \
  --mount "type=volume,src=$secret_volume,dst=/target" \
  busybox:1.37.0-uclibc@sha256:39e0df8c4d65953b55c344f017e1ff2e0031a7454b3c24e6b76d402f207e315a \
  sh -c 'cp /source/password /target/redis_password && chown 999:999 /target/redis_password && chmod 0400 /target/redis_password'

docker run --detach --name "$container" \
  --read-only \
  --user 999:999 \
  --cap-drop ALL \
  --security-opt no-new-privileges:true \
  --tmpfs /tmp:rw,noexec,nosuid,nodev,size=16m,uid=999,gid=999,mode=0700 \
  --env REDIS_PASSWORD_FILE=/run/secrets/redis_password \
  --mount "type=bind,src=$ROOT_DIR/deploy/redis/entrypoint.sh,dst=/usr/local/bin/nexusrelay-redis-entrypoint,readonly" \
  --mount "type=volume,src=$secret_volume,dst=/run/secrets,readonly" \
  --entrypoint /usr/local/bin/nexusrelay-redis-entrypoint \
  "$REDIS_IMAGE" >/dev/null

attempt=0
until docker exec --env REDISCLI_AUTH="$password" "$container" redis-cli --no-auth-warning ping | grep -qx PONG; do
  attempt=$((attempt + 1))
  if [ "$attempt" -ge 30 ]; then
    docker logs "$container" >&2
    exit 1
  fi
  sleep 1
done

compose_health_password=$(docker exec "$container" sh -c "tr -d '\\r\\n' </run/secrets/redis_password")
[ "$compose_health_password" = "$password" ] || {
  printf 'Compose Redis healthcheck normalization disagrees with startup\n' >&2
  exit 1
}

flush_output=$(docker exec --env REDISCLI_AUTH="$password" "$container" redis-cli --no-auth-warning flushall 2>&1)
case "$flush_output" in
  *NOPERM*) ;;
  *)
  printf 'Redis bootstrap identity unexpectedly permits FLUSHALL\n' >&2
  exit 1
  ;;
esac
if docker logs "$container" 2>&1 | grep -F "$password" >/dev/null; then
  printf 'Redis logs disclosed the password\n' >&2
  exit 1
fi

printf 'Redis startup contract valid\n'
