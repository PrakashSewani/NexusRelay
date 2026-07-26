#!/bin/sh
set -eu

TAILSCALE_IMAGE='tailscale/tailscale:v1.98.9@sha256:f15d5d3f4a68773a853180b72496f70ba614b64de0878c43fe3da39fe0afba47'
COREDNS_IMAGE='coredns/coredns:1.12.4@sha256:986f04c2e15e147d00bdd51e8c51bcef3644b13ff806be7d2ff1b261d6dfbae1'
ROOT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
test_id="nexusrelay-private-feasibility-$$"
work=$(mktemp -d)
network="$test_id-network"
coredns="$test_id-coredns"

cleanup() {
  docker rm -f "$coredns" >/dev/null 2>&1 || true
  docker network rm "$network" >/dev/null 2>&1 || true
  rm -rf "$work"
}
trap cleanup EXIT HUP INT TERM

docker run --rm \
  --cap-drop ALL \
  --cap-add NET_ADMIN \
  --device /dev/net/tun \
  --sysctl net.ipv4.ip_forward=1 \
  --security-opt no-new-privileges:true \
  "$TAILSCALE_IMAGE" sh -ec '
    tailscaled --state=mem: --socket=/tmp/tailscaled.sock --tun=tailscale0 --no-logs-no-support >/tmp/tailscaled.log 2>&1 &
    pid=$!
    trap "kill $pid 2>/dev/null || true" EXIT
    attempt=0
    while ! ip link show tailscale0 >/dev/null 2>&1; do
      attempt=$((attempt + 1))
      [ "$attempt" -lt 50 ] || { printf "%s\n" "kernel TUN interface was not created" >&2; exit 1; }
      sleep 0.1
    done
    [ "$(cat /proc/sys/net/ipv4/ip_forward)" = 1 ]
  '

ADMIN_HOST=admin.example.test \
PRIVATE_TRAEFIK_IP=172.30.0.10 \
PRIVATE_DNS_IP=172.30.0.53 \
  "$ROOT_DIR/deploy/private-admin/generate-corefile.sh" "$work/Corefile"

docker network create --subnet 172.30.0.0/24 "$network" >/dev/null
docker run -d --name "$coredns" \
  --network "$network" \
  --ip 172.30.0.53 \
  --read-only \
  --cap-drop ALL \
  --cap-add NET_BIND_SERVICE \
  --security-opt no-new-privileges:true \
  --mount "type=bind,src=$work/Corefile,dst=/Corefile,readonly" \
  "$COREDNS_IMAGE" -conf /Corefile >/dev/null

attempt=0
while ! docker run --rm --network "$network" "$TAILSCALE_IMAGE" \
  timeout 2 nslookup admin.example.test 172.30.0.53 2>/dev/null | grep -F 'Address: 172.30.0.10' >/dev/null; do
  attempt=$((attempt + 1))
  [ "$attempt" -lt 30 ] || { printf '%s\n' 'CoreDNS exact-host answer was not ready' >&2; exit 1; }
  sleep 0.2
done

if docker run --rm --network "$network" "$TAILSCALE_IMAGE" \
  timeout 2 nslookup other.example.test 172.30.0.53 >/dev/null 2>&1; then
  printf '%s\n' 'CoreDNS answered a hostname outside the exact allowlist' >&2
  exit 1
fi

printf '%s\n' 'private-admin kernel TUN and exact split-DNS feasibility valid'
