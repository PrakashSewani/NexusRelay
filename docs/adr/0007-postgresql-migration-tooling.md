# ADR 0007: PostgreSQL Migration Tooling

- Status: Accepted
- Date: 2026-07-26

## Context

NexusRelay requires forward-only PostgreSQL migrations that run before application rollout, serialize concurrent deployers, detect changes to already-applied files, record execution metadata, and can be tested from both an empty database and the previous supported release schema. The deployment design uses a one-shot `migrate` container and a separately privileged migration database role.

The tooling choice is consequential because it owns migration ordering, integrity verification, transaction behavior, failure recovery, schema-history data, and the artifact executed during deployment. A custom migrator would make NexusRelay responsible for security- and recovery-sensitive behavior that established tooling already provides.

## Decision

- Use Atlas versioned migrations and a pinned Atlas Community CLI release in the one-shot `migrate` container. The Community binary is built from the Apache-2.0-licensed Atlas repository and does not require Atlas Cloud or a commercial account.
- Keep repository-owned, ordered, forward-only SQL migration files under `migrations/`. Atlas executes these files but does not replace SQL review or make declarative schema diffing the source of truth for V1.
- Commit the Atlas migration directory checksum file, `atlas.sum`. CI runs `atlas migrate hash` and fails if regeneration changes the worktree.
- Treat any migration recorded as applied in a supported environment as immutable. Corrections use a new forward migration; changing an applied file and recomputing `atlas.sum` is prohibited.
- Run `atlas migrate validate` before application. CI also validates SQL semantics against the pinned PostgreSQL test version using `--dev-url`. Validation and directory-integrity failure stop deployment before any production migration statement runs.
- Use Atlas's PostgreSQL advisory locking during migration application. The deployment must not disable locking. Lock acquisition is bounded by an explicit deployment timeout and a failure prevents application containers from starting.
- Use one transaction per migration file by default. A migration may opt out only when PostgreSQL requires non-transactional execution or the reviewed operational plan requires it. Such files must be isolated, idempotent or recoverable, and include documented failure and retry steps.
- Keep Atlas's default linear execution order. Normal deployment must not use `linear-skip`, `non-linear`, `--allow-dirty`, `--baseline`, or `--skip-lock`.
- Use Atlas's revision table as the authoritative migration history. It records the migration version and description, integrity hash, execution timestamp and duration, statement progress, and failure metadata needed to diagnose partial application.
- Keep migration state global and outside tenant RLS. Only the migration role and narrowly scoped operational inspection paths may mutate or inspect it.
- The migration container receives only the PostgreSQL settings and mounted password file required for migration. Credential assembly must not print the password or place it in image layers, Compose files, command output, or logs.
- Application startup remains gated on successful migration completion and compatible schema state. Gateway, control-plane, and worker processes never apply migrations themselves.
- Pin the Atlas binary version and release artifact digest in build/deployment files. Upgrades require release-note review, empty/previous-schema migration tests, lock-contention tests, failure-recovery tests, and review of revision-table compatibility.
- Atlas Cloud, remote schema registries, migration linting that requires Atlas Pro, and outbound telemetry are not V1 dependencies. Migration operation remains self-hosted against the configured PostgreSQL database. Destructive-change and lock review remains a repository review and test responsibility.

## Migration Authoring and Recovery

1. Add a new monotonically ordered SQL migration; do not edit an applied migration.
2. Make transactional files safe to roll back completely on statement failure.
3. For an explicitly non-transactional file, document statement-level restart behavior before merge and use Atlas revision progress to determine the safe recovery action.
4. Regenerate `atlas.sum`, validate the directory, and review both the SQL and checksum change.
5. Test application from an empty database and from the previous supported release schema.
6. For destructive, table-rewriting, or long-lock operations, use expand-and-contract or a resumable worker backfill and document lock/recovery behavior.

Normal deployment never uses downgrade execution, baseline, revision deletion, or revision-table mutation to conceal a failed migration. Exceptional repair requires an audited operator runbook and evidence that the database schema and Atlas revision state agree before rollout continues.

## Alternatives

### Custom Go Migrator

Rejected because NexusRelay would need to own SQL parsing, checksums, advisory-lock lifecycle, transaction directives, statement progress, durable failure metadata, and recovery behavior. That surface is larger and more operationally sensitive than the product-specific code it would replace.

### Goose With a NexusRelay History Store

Rejected for V1. Goose provides ordered SQL migrations, transactions, a Go library, a pluggable history store, and PostgreSQL session locks, but meeting the required checksum, execution-duration, immutable-directory, and partial-failure contracts would require a custom store and wrapper. Atlas provides those capabilities as one coherent migration system with less NexusRelay-owned migration machinery.

### `golang-migrate` or Tern

Rejected because their standard migration history does not natively satisfy the complete integrity and execution-metadata contract. Extending them would have the same custom ownership problem without a compensating NexusRelay requirement.

### Declarative Schema as the V1 Source of Truth

Deferred. Declarative diffing can be useful, but reviewed forward SQL files provide explicit lock, data-movement, RLS, and recovery behavior. Adoption would be a separate consequential decision.

## Consequences

- The application image does not contain a custom `nexusrelay-migrate` Go binary; the migration container uses a separately pinned Atlas Community CLI image or build stage.
- Migration correctness relies on both PostgreSQL revision history and the committed `atlas.sum` integrity chain.
- Operators gain standardized locking, statement progress, checksums, timestamps, durations, and failure records without NexusRelay maintaining a custom migration engine.
- Atlas Community becomes a production deployment dependency and must be pinned, scanned, included in release provenance, and tested during upgrades.
- Non-transactional PostgreSQL operations remain possible but require explicit review and stronger recovery documentation.
- Forward-only policy is a NexusRelay release rule enforced by repository review, CI, and deployment permissions; Atlas capabilities that could rewrite or repair history are not part of normal deployment.

## Verification

- `atlas migrate validate` succeeds for the committed migration directory.
- `atlas migrate hash` leaves the worktree clean.
- Applying migrations to an empty PostgreSQL database reaches the expected schema and revision state.
- Applying migrations to the previous supported release database reaches the same expected schema and preserves data.
- A second concurrent migrator cannot apply migrations while the first holds the PostgreSQL advisory lock.
- A transactional migration failure leaves its schema changes unapplied and emits an actionable sanitized command failure without exposing credentials.
- A test-only non-transactional failure proves documented statement-progress recovery and safe rerun behavior.
- Editing an already-applied migration causes integrity validation or revision-hash verification to fail.
- Gateway, control-plane, and worker startup remains blocked after migration validation, lock, or application failure.
- Release image inspection confirms the pinned Atlas version/digest, non-root execution where supported, and absence of migration credentials from layers and metadata.

## References

- `docs/requirements.md`: NFR-007 and the deployment, upgrade, and testing acceptance criteria.
- `docs/design/01-system-topology.md`: one-shot migration container and startup ordering.
- `docs/design/02-persistence-tenancy.md`: forward-only history, locking, metadata, expand-and-contract, and schema-state tests.
- `docs/design/11-operations-security-testing.md`: image, secret, CI, and release requirements.
- Atlas versioned migration application: <https://atlasgo.io/versioned/apply>
- Atlas migration directory integrity: <https://atlasgo.io/concepts/migration-directory-integrity>
- Atlas CLI reference for apply, hash, and validate: <https://atlasgo.io/cli-reference#atlas-migrate>
- Atlas Community Edition: <https://atlasgo.io/community-edition>
- Atlas Go migration package and revision model: <https://pkg.go.dev/ariga.io/atlas/sql/migrate>
- PostgreSQL advisory locks: <https://www.postgresql.org/docs/current/explicit-locking.html#ADVISORY-LOCKS>
