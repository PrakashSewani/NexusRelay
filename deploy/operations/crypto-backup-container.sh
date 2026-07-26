#!/bin/sh
set -eu
set +x
umask 077

target="/backup/$BACKUP_ID"
candidate=$(mktemp -d "/backup/.$BACKUP_ID.XXXXXX")
trap 'rm -rf "$candidate"' EXIT HUP INT TERM
files='provider_master_keyring api_key_pepper_ring csrf_secret_ring session_secret'
for name in $files; do
  [ -f "/source/$name" ] && [ ! -L "/source/$name" ] || { printf 'cryptographic backup source is incomplete\n' >&2; exit 1; }
  [ "$(stat -c %a "/source/$name")" = 600 ] || { printf 'cryptographic backup source files must have mode 0600\n' >&2; exit 1; }
  cp "/source/$name" "$candidate/$name"
done
cat >"$candidate/metadata.env" <<EOF
FORMAT=nexusrelay-cryptographic-backup-v1
BACKUP_ID=$BACKUP_ID
NEXUSRELAY_RELEASE=$NEXUSRELAY_RELEASE
NEXUSRELAY_REVISION=$NEXUSRELAY_REVISION
SCOPE=provider-master-ring,api-pepper-ring,csrf-ring,session-secret
EOF
(cd "$candidate" && sha256sum $files metadata.env >SHA256SUMS)
chmod 0600 "$candidate"/*
chmod 0700 "$candidate"
mv "$candidate" "$target"
trap - EXIT HUP INT TERM
printf 'cryptographic backup artifact verified and published\n'
