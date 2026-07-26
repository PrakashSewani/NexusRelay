#!/bin/sh
set -eu
set +x
umask 077

cd /artifact
sha256sum -c SHA256SUMS >/dev/null
grep -qx 'FORMAT=nexusrelay-cryptographic-backup-v1' metadata.env
grep -qx 'SCOPE=provider-master-ring,api-pepper-ring,csrf-ring,session-secret' metadata.env
for name in provider_master_keyring api_key_pepper_ring csrf_secret_ring session_secret; do
  cp "$name" "/destination/$name"
  chmod 0600 "/destination/$name"
done
