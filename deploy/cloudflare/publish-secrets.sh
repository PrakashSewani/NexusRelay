#!/bin/sh
set -eu
set +x
umask 077

fail() {
  printf 'Cloudflare secret publication failed: %s\n' "$1" >&2
  exit 1
}

for interface_path in /sys/class/net/*; do
  [ "${interface_path##*/}" = lo ] || fail 'publisher must run with network disabled'
done

[ -d /source ] && [ ! -L /source ] || fail 'source root must be a directory'
[ "$(stat -c %a /source)" = 700 ] || fail 'source root must have mode 0700'
expected='acme_dns_api_token
cloudflare_tunnel_credentials.json'
actual=$(find /source -mindepth 1 -maxdepth 1 -print | while IFS= read -r path; do basename "$path"; done | sort)
[ "$actual" = "$expected" ] || fail 'source inventory differs from the exact allowlist'

for name in acme_dns_api_token cloudflare_tunnel_credentials.json; do
  path=/source/$name
  [ -f "$path" ] && [ ! -L "$path" ] || fail 'source contains a non-regular allowlisted entry'
  [ "$(stat -c %a "$path")" = 600 ] || fail 'source files must have mode 0600'
done

preflight() {
  target=$1
  name=$2
  [ -d "$target" ] && [ ! -L "$target" ] || fail 'a fixed target root is unavailable'
  actual=$(find "$target" -mindepth 1 -maxdepth 1 -print | while IFS= read -r path; do basename "$path"; done | sort)
  if [ -n "$actual" ]; then
    [ "$actual" = "$name" ] || fail 'target contains missing or extra files'
    [ -f "$target/$name" ] && [ ! -L "$target/$name" ] || fail 'target contains a non-regular entry'
    cmp -s "/source/$name" "$target/$name" || fail 'published secret differs from source'
    [ "$(stat -c %a "$target/$name")" = 400 ] || fail 'published secret mode differs from 0400'
    [ "$(stat -c %u:%g "$target/$name")" = 65532:65532 ] || fail 'published secret ownership differs from cloudflared identity'
  fi
}

publish() {
  target=$1
  name=$2
  [ -z "$(find "$target" -mindepth 1 -maxdepth 1 -print -quit)" ] || return 0
  cp "/source/$name" "$target/$name"
  chmod 0400 "$target/$name"
  chown 65532:65532 "$target/$name"
}

preflight /target/cloudflared cloudflare_tunnel_credentials.json
preflight /target/traefik acme_dns_api_token
publish /target/cloudflared cloudflare_tunnel_credentials.json
publish /target/traefik acme_dns_api_token

printf 'protected Cloudflare secret volumes are ready\n'
