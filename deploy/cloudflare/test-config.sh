#!/bin/sh
set -eu

CLOUDFLARED_IMAGE='cloudflare/cloudflared:2026.7.3@sha256:e39ee8da81ad5e05d77f38d2f51c60ca51bf2a8450ac3abab50c17fdb91d91bf'
BUSYBOX_IMAGE='busybox:1.37.0-uclibc@sha256:39e0df8c4d65953b55c344f017e1ff2e0031a7454b3c24e6b76d402f207e315a'
ROOT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
test_id="nexusrelay-cloudflare-test-$$"
work=$(mktemp -d)
source_dir="$work/source"
mkdir "$source_dir"
chmod 0700 "$source_dir"

cleanup() {
  docker volume rm "$test_id-cloudflared" "$test_id-traefik" >/dev/null 2>&1 || true
  rm -rf "$work"
}
trap cleanup EXIT HUP INT TERM

ENABLE_CLOUDFLARE_TUNNEL=true \
PUBLIC_API_HOST=api.example.test \
CLOUDFLARE_TUNNEL_ID=11111111-2222-4333-8444-555555555555 \
  "$ROOT_DIR/deploy/cloudflare/generate-config.sh" "$work/config.yml"

docker run --rm --network none \
  --mount "type=bind,src=$work/config.yml,dst=/etc/cloudflared/config.yml,readonly" \
  "$CLOUDFLARED_IMAGE" tunnel --config /etc/cloudflared/config.yml ingress validate >/dev/null

assert_rule() {
  url=$1
  expected=$2
  output=$(docker run --rm --network none \
    --mount "type=bind,src=$work/config.yml,dst=/etc/cloudflared/config.yml,readonly" \
    "$CLOUDFLARED_IMAGE" tunnel --config /etc/cloudflared/config.yml ingress rule "$url")
  printf '%s\n' "$output" | grep -F "Matched rule #$expected" >/dev/null
}

assert_rule https://api.example.test/v1/models 0
assert_rule https://api.example.test/admin 0
assert_rule https://admin.example.test/v1/models 1

grep -F 'service: https://proxy:8443' "$work/config.yml" >/dev/null
grep -F 'httpHostHeader: api.example.test' "$work/config.yml" >/dev/null
grep -F 'originServerName: api.example.test' "$work/config.yml" >/dev/null
grep -F 'noTLSVerify: false' "$work/config.yml" >/dev/null
[ "$(grep -c '^  - hostname:' "$work/config.yml")" -eq 1 ]
[ "$(grep -c '^  - service: http_status:404$' "$work/config.yml")" -eq 1 ]
if grep -F 'admin.example.test' "$work/config.yml" >/dev/null; then
  printf 'cloudflared configuration contains an admin hostname\n' >&2
  exit 1
fi

printf '%s' token >"$source_dir/acme_dns_api_token"
printf '%s' '{"AccountTag":"account","TunnelSecret":"secret","TunnelID":"11111111-2222-4333-8444-555555555555"}' >"$source_dir/cloudflare_tunnel_credentials.json"
chmod 0600 "$source_dir"/*
docker volume create "$test_id-cloudflared" >/dev/null
docker volume create "$test_id-traefik" >/dev/null

run_publisher() {
  docker run --rm --network none --read-only --user 0:0 \
    --mount "type=bind,src=$ROOT_DIR/deploy/cloudflare/publish-secrets.sh,dst=/publish.sh,readonly" \
    --mount "type=bind,src=$source_dir,dst=/source,readonly" \
    --mount "type=volume,src=$test_id-cloudflared,dst=/target/cloudflared" \
    --mount "type=volume,src=$test_id-traefik,dst=/target/traefik" \
    "$BUSYBOX_IMAGE" /bin/sh /publish.sh
}
run_publisher >/dev/null
run_publisher >/dev/null

for contract in 'cloudflared cloudflare_tunnel_credentials.json' 'traefik acme_dns_api_token'; do
  set -- $contract
  docker run --rm --network none --read-only \
    --mount "type=volume,src=$test_id-$1,dst=/target,readonly" \
    "$BUSYBOX_IMAGE" sh -ec '[ "$(stat -c "%a|%u:%g" "/target/$1")" = "400|65532:65532" ]' sh "$2"
done

printf 'Cloudflare configuration and secret publication contracts valid\n'
