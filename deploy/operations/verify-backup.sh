#!/bin/sh
set -eu
set +x

POSTGRES_IMAGE='postgres:18.4-bookworm@sha256:1961f96e6029a02c3812d7cb329a3b03a3ac2bb067058dec17b0f5596aca9296'
BUSYBOX_IMAGE='busybox:1.37.0-uclibc@sha256:39e0df8c4d65953b55c344f017e1ff2e0031a7454b3c24e6b76d402f207e315a'

mode_of() {
  stat -f %Lp "$1" 2>/dev/null || stat -c %a "$1"
}

[ "$#" -eq 2 ] || { printf 'Usage: deploy/operations/verify-backup.sh DATABASE_ARTIFACT CRYPTO_ARTIFACT\n' >&2; exit 2; }
database_artifact=$1
crypto_artifact=$2
for artifact in "$database_artifact" "$crypto_artifact"; do
  [ -d "$artifact" ] && [ ! -L "$artifact" ] || { printf 'backup artifact must be a non-symlink directory\n' >&2; exit 1; }
  [ "$(mode_of "$artifact")" = 700 ] || { printf 'backup artifact must have mode 0700\n' >&2; exit 1; }
done

docker run --rm --network none --read-only --mount "type=bind,src=$database_artifact,dst=/artifact,readonly" "$POSTGRES_IMAGE" sh -ec '
  cd /artifact
  sha256sum -c SHA256SUMS
  grep -qx "FORMAT=nexusrelay-logical-backup-v1" metadata.env
  grep -qx "PASSWORDS_INCLUDED=false" metadata.env
  grep -q "^POSTGRESQL_VERSION=18\\." metadata.env
  pg_restore --list database.dump >/dev/null
' >/dev/null
docker run --rm --network none --read-only --mount "type=bind,src=$crypto_artifact,dst=/artifact,readonly" "$BUSYBOX_IMAGE" sh -ec '
  cd /artifact
  sha256sum -c SHA256SUMS
  grep -qx "FORMAT=nexusrelay-cryptographic-backup-v1" metadata.env
  grep -qx "SCOPE=provider-master-ring,api-pepper-ring,csrf-ring,session-secret" metadata.env
' >/dev/null

database_id=$(sed -n 's/^BACKUP_ID=//p' "$database_artifact/metadata.env")
crypto_id=$(sed -n 's/^BACKUP_ID=//p' "$crypto_artifact/metadata.env")
[ -n "$database_id" ] && [ "$database_id" = "$crypto_id" ] || { printf 'database and cryptographic backup IDs differ\n' >&2; exit 1; }
printf 'backup %s integrity and scope verified\n' "$database_id"
