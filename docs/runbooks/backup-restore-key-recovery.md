# Backup, Restore, and Key Recovery

## Scope

This runbook is the Phase 2 reference recovery workflow required by `docs/design/02-persistence-tenancy.md` and `docs/design/11-operations-security-testing.md`. It creates a portable logical PostgreSQL backup and a separate cryptographic recovery artifact. It deliberately does not back up Redis because Redis is reconstructable.

The database artifact contains a custom-format `pg_dump`, password-free global role definitions from `pg_dumpall --roles-only --no-role-passwords`, the exact PostgreSQL 18 role/edge verification result, checksums, and release metadata. PostgreSQL 18's historical `GRANTED BY nexusrelay_cluster_admin` clauses are normalized so a different target recovery superuser can replay the same exact edge/options; the source grantor identity is not part of the authorization graph contract. The cryptographic artifact contains exactly:

- Provider master key ring.
- API-key pepper ring.
- CSRF ring.
- Session secret.

Database login passwords remain independent protected recovery inputs. They are never included in either artifact. The workflow does not use `PGPASSWORD`, credential-bearing URLs, or command-line password values.

## Protection and Retention

Use different storage/security domains for the database and cryptographic roots. Both local roots and every published artifact must be mode `0700`; artifact files are mode `0600`. Move or replicate each artifact using an operator-approved encrypted system with independent access controls.

No script deletes old backups. Retention deletion is intentionally manual and outside this Phase 2 workflow. Before retiring any cryptographic artifact, prove no retained database backup references provider master keys, API peppers, CSRF keys, or session state that requires it. Losing the provider master ring makes encrypted provider credentials unrecoverable.

## Create a Backup

Stop no services for the logical dump, but avoid configuration/key rotation during the reference backup window. Create separate protected destinations and choose an immutable backup ID:

```text
mkdir -m 0700 /protected/database-backups /separately-protected/crypto-backups
BACKUP_ID=2026-07-26T120000Z \
NEXUSRELAY_RELEASE=0.2.0 \
NEXUSRELAY_REVISION=<release-revision> \
BACKUP_DATABASE_ROOT=/protected/database-backups \
BACKUP_CRYPTO_ROOT=/separately-protected/crypto-backups \
CRYPTO_SOURCE_ROOT=.local-secrets \
DATABASE_HOST=postgres \
DATABASE_PORT=5432 \
DATABASE_NAME=nexusrelay \
POSTGRES_USER=nexusrelay_cluster_admin \
POSTGRES_PASSWORD_FILE=.local-secrets/postgres_cluster_admin_password \
NEXUSRELAY_POSTGRES_DOCKER_NETWORK=<compose-backend-network> \
deploy/operations/backup.sh
```

Cluster-admin is mounted into one short-lived PostgreSQL client container only for backup/recovery role and database export. It is never supplied to Atlas or a long-running service. The resulting metadata covers the NexusRelay release and revision, Atlas `1.2.3-community`, exact PostgreSQL 18 minor, and role-graph contract.

Verify the pair offline:

```text
deploy/operations/verify-backup.sh \
  /protected/database-backups/2026-07-26T120000Z \
  /separately-protected/crypto-backups/2026-07-26T120000Z
```

## Restore an Empty PostgreSQL 18 Target

Keep all NexusRelay application and Atlas containers stopped. The target must be an empty PostgreSQL 18 cluster. PostgreSQL major-version restoration is blocked until the target NexusRelay release has a reviewed release-specific major-upgrade plan. A PostgreSQL minor update within major 18 must be covered by release testing and the recorded source metadata.

Use a short-lived local recovery administrator, the database artifact, and five protected database password files. The role dump carries no passwords; `restore.sh` re-establishes each fixed login password after restoring roles and the database.

```text
DATABASE_BACKUP_ARTIFACT=/protected/database-backups/2026-07-26T120000Z \
CRYPTO_BACKUP_ARTIFACT=/separately-protected/crypto-backups/2026-07-26T120000Z \
DATABASE_HOST=<empty-postgres-18-host> \
DATABASE_PORT=5432 \
RECOVERY_ADMIN_USER=postgres \
RECOVERY_ADMIN_PASSWORD_FILE=/protected/recovery-admin-password \
POSTGRES_PASSWORD_FILE=/protected/postgres_cluster_admin_password \
DATABASE_MIGRATION_PASSWORD_FILE=/protected/postgres_migration_password \
DATABASE_GATEWAY_PASSWORD_FILE=/protected/postgres_gateway_password \
DATABASE_CONTROL_PLANE_PASSWORD_FILE=/protected/postgres_control_plane_password \
DATABASE_WORKER_PASSWORD_FILE=/protected/postgres_worker_password \
NEXUSRELAY_POSTGRES_DOCKER_NETWORK=<restore-network> \
CONFIRM_EMPTY_RESTORE_TARGET=yes \
deploy/operations/restore.sh
```

Restore the cryptographic artifact into a separate empty protected directory:

```text
mkdir -m 0700 /protected/recovered-crypto
deploy/operations/restore-crypto.sh \
  /separately-protected/crypto-backups/2026-07-26T120000Z \
  /protected/recovered-crypto
```

Publish those recovered files through the normal protected secret mechanism only after checking that their key IDs/epochs match the restored database. Never merge rings by hand or silently substitute newly generated keys.

## Pre-Startup Verification

Before Atlas runs, execute the reusable PostgreSQL 18 verifier using the short-lived cluster-admin recovery credential:

```text
DATABASE_HOST=<restored-host> \
DATABASE_PORT=5432 \
DATABASE_NAME=nexusrelay \
DATABASE_VERIFY_USER=nexusrelay_cluster_admin \
DATABASE_VERIFY_PASSWORD_FILE=/protected/postgres_cluster_admin_password \
ROLE_GRAPH_SQL=deploy/postgres/verify-role-graph.sql \
deploy/postgres/verify-role-graph.sh
```

The core Compose startup runs the same verifier as `postgres-role-verify` before `migrate`. Any missing/extra NexusRelay role or edge, or any incorrect `LOGIN`, `SUPERUSER`, `BYPASSRLS`, `CREATEROLE`, `INHERIT`, `ADMIN`, per-edge `INHERIT`, or `SET` option blocks Atlas.

Phase 2 completion proves artifact integrity, portable logical restore, exact role graph, schema ownership, and cryptographic artifact recovery. Later phases extend the restore exercise with encrypted provider credential readability, RLS, outbox resumption, aggregate idempotency, and complete product startup assertions as those capabilities exist.

## Automated Exercise

Run the isolated Docker exercise:

```text
make backup-restore-test
```

It initializes a source PostgreSQL 18 cluster, creates both artifacts, verifies them, restores an independent empty PostgreSQL 18 cluster, reapplies protected login credentials, and verifies the exact role graph and schema owner. It uses only generated test secrets and removes its containers and temporary artifacts.
