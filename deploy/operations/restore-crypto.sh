#!/bin/sh
set -eu
set +x
umask 077

BUSYBOX_IMAGE='busybox:1.37.0-uclibc@sha256:39e0df8c4d65953b55c344f017e1ff2e0031a7454b3c24e6b76d402f207e315a'
ROOT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
fail() { printf 'cryptographic restore error: %s\n' "$1" >&2; exit 1; }
mode_of() { stat -c %a "$1" 2>/dev/null || stat -f %Lp "$1"; }

[ "$#" -eq 2 ] || { printf 'Usage: deploy/operations/restore-crypto.sh CRYPTO_ARTIFACT EMPTY_DESTINATION\n' >&2; exit 2; }
artifact=$1
destination=$2
[ -d "$artifact" ] && [ ! -L "$artifact" ] || fail 'CRYPTO_ARTIFACT must be a non-symlink directory'
[ -d "$destination" ] && [ ! -L "$destination" ] || fail 'EMPTY_DESTINATION must be a non-symlink directory'
[ "$(mode_of "$destination")" = 700 ] || fail 'EMPTY_DESTINATION must have mode 0700'
[ -z "$(ls -A "$destination")" ] || fail 'EMPTY_DESTINATION must be empty'

docker run --rm --network none --read-only --user "$(id -u):$(id -g)" \
  --mount "type=bind,src=$ROOT_DIR/deploy/operations/restore-crypto-container.sh,dst=/opt/nexusrelay/restore.sh,readonly" \
  --mount "type=bind,src=$artifact,dst=/artifact,readonly" \
  --mount "type=bind,src=$destination,dst=/destination" \
  "$BUSYBOX_IMAGE" /opt/nexusrelay/restore.sh
printf 'cryptographic recovery files restored to the protected destination\n'
