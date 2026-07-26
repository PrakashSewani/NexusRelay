#!/bin/sh
set -eu

TRAEFIK_IMAGE='traefik:v3.5.0@sha256:4e7175cfe19be83c6b928cae49dde2f2788fb307189a4dc9550b67acf30c11a5'
ROOT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
container="nexusrelay-traefik-test-$$"
token_file=$(mktemp)
printf '%s' test-token >"$token_file"
chmod 0600 "$token_file"

cleanup() {
  docker rm --force "$container" >/dev/null 2>&1 || true
  rm -f "$token_file"
}
trap cleanup EXIT HUP INT TERM

docker run --detach --name "$container" \
  --read-only \
  --user 65532:65532 \
  --cap-drop ALL \
  --security-opt no-new-privileges:true \
  --tmpfs /tmp:rw,noexec,nosuid,nodev,size=16m,uid=65532,gid=65532,mode=0700 \
  --env PUBLIC_API_HOST=api.example.test \
  --env ADMIN_HOST=admin.example.test \
  --mount "type=bind,src=$ROOT_DIR/deploy/traefik/traefik.yaml,dst=/etc/traefik/traefik.yaml,readonly" \
  --mount "type=bind,src=$ROOT_DIR/deploy/traefik/entrypoint.sh,dst=/usr/local/bin/nexusrelay-traefik-entrypoint,readonly" \
  --entrypoint /usr/local/bin/nexusrelay-traefik-entrypoint \
  "$TRAEFIK_IMAGE" >/dev/null

sleep 2
[ "$(docker inspect --format '{{.State.Running}}' "$container")" = true ] || {
  docker logs "$container" >&2
  exit 1
}
docker exec "$container" grep -F 'Host(`api.example.test`) && (Path(`/v1`) || PathPrefix(`/v1/`))' /tmp/dynamic.yaml >/dev/null
docker exec "$container" grep -F 'Host(`admin.example.test`) && (Path(`/api/control/v1`) || PathPrefix(`/api/control/v1/`))' /tmp/dynamic.yaml >/dev/null
docker exec "$container" grep -F 'public-catchall:' /tmp/dynamic.yaml >/dev/null
docker exec "$container" grep -F 'service: public-not-found' /tmp/dynamic.yaml >/dev/null
docker exec "$container" grep -F 'path: "/__nexusrelay_not_found__"' /tmp/dynamic.yaml >/dev/null
if docker exec "$container" grep -F 'ping:' /etc/traefik/traefik.yaml >/dev/null; then
  printf 'Traefik public entrypoint unexpectedly exposes built-in ping\n' >&2
  exit 1
fi

docker rm --force "$container" >/dev/null
docker run --detach --name "$container" \
  --read-only \
  --user 65532:65532 \
  --cap-drop ALL \
  --security-opt no-new-privileges:true \
  --tmpfs /tmp:rw,noexec,nosuid,nodev,size=16m,uid=65532,gid=65532,mode=0700 \
  --tmpfs /var/lib/traefik:rw,noexec,nosuid,nodev,size=16m,uid=65532,gid=65532,mode=0700 \
  --env PUBLIC_API_HOST=api.example.test \
  --env ADMIN_HOST=admin.example.test \
  --env TLS_MODE=acme \
  --env ACME_EMAIL=operator@example.test \
  --env ACME_DNS_PROVIDER=cloudflare \
  --env CF_DNS_API_TOKEN_FILE=/run/secrets/acme_dns_api_token \
  --mount "type=bind,src=$token_file,dst=/run/secrets/acme_dns_api_token,readonly" \
  --mount "type=bind,src=$ROOT_DIR/deploy/traefik/traefik.yaml,dst=/etc/traefik/traefik.yaml,readonly" \
  --mount "type=bind,src=$ROOT_DIR/deploy/traefik/entrypoint.sh,dst=/usr/local/bin/nexusrelay-traefik-entrypoint,readonly" \
  --entrypoint /usr/local/bin/nexusrelay-traefik-entrypoint \
  "$TRAEFIK_IMAGE" >/dev/null

sleep 2
[ "$(docker inspect --format '{{.State.Running}}' "$container")" = true ] || {
  docker logs "$container" >&2
  exit 1
}
docker exec "$container" grep -F 'entryPoints: [websecure]' /tmp/dynamic.yaml >/dev/null
docker exec "$container" grep -F 'certResolver: cloudflare' /tmp/dynamic.yaml >/dev/null
docker exec "$container" grep -F 'minVersion: VersionTLS12' /tmp/dynamic.yaml >/dev/null
docker exec "$container" grep -F 'sniStrict: true' /tmp/dynamic.yaml >/dev/null

printf 'Traefik configuration contract valid\n'
