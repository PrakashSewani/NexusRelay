#!/bin/sh
set -eu

fail() {
  printf 'cloudflared config generation failed: %s\n' "$1" >&2
  exit 1
}

validate_host() {
  host=$1
  case "$host" in
    ''|*[!A-Za-z0-9.-]*|.*|*..*|*.) fail 'PUBLIC_API_HOST has invalid hostname syntax' ;;
  esac
}

[ "${ENABLE_CLOUDFLARE_TUNNEL-}" = true ] || fail 'ENABLE_CLOUDFLARE_TUNNEL must be true'
validate_host "${PUBLIC_API_HOST-}"
[ -n "${CLOUDFLARE_TUNNEL_ID-}" ] || fail 'CLOUDFLARE_TUNNEL_ID is required'
[ "$#" -eq 1 ] || fail 'one output path is required'

output=$1
umask 077
rm -f "$output"
cat >"$output" <<EOF
tunnel: ${CLOUDFLARE_TUNNEL_ID}
credentials-file: /run/secrets/cloudflare_tunnel_credentials.json
metrics: 127.0.0.1:2000
loglevel: info
transport-loglevel: info
ingress:
  - hostname: ${PUBLIC_API_HOST}
    service: https://proxy:8443
    originRequest:
      httpHostHeader: ${PUBLIC_API_HOST}
      originServerName: ${PUBLIC_API_HOST}
      noTLSVerify: false
      disableChunkedEncoding: false
  - service: http_status:404
EOF
chmod 0444 "$output"
