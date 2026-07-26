# NexusRelay Upgrade Runbook

## Release Matrix

Every release upgrade must record and test all four dimensions together:

- NexusRelay source release/revision and target release/revision.
- Atlas Community version and revision-table compatibility.
- PostgreSQL exact source and target minor within the supported major.
- PostgreSQL role-graph contract and any approved graph transition.

The Phase 2 pins are Atlas `1.2.3-community`, PostgreSQL `18.4`, and role graph `postgresql-18-v1`. PostgreSQL major upgrades are blocked. A later release must supply a release-specific plan covering logical/physical transition choice, extension/collation compatibility, downtime or replication behavior, validation, rollback boundaries, and recovery evidence before changing major version.

## Normal Upgrade

1. Read target release notes and migration recovery notes.
2. Create and verify separate database and cryptographic artifacts using `backup-restore-key-recovery.md`.
3. Exercise restore with the target release tooling.
4. Stop application services while retaining PostgreSQL.
5. Run `postgres-role-verify` before Atlas.
6. Run Atlas only as `nexusrelay_migration` after the verifier passes.
7. Start the target application release and run the assertions implemented by that release.
8. Retain pre-upgrade artifacts until rollback/recovery policy allows manual retirement. No NexusRelay script deletes them.

## Exceptional Role-Graph Upgrade

A role-graph change is not a migration. It requires a reviewed SQL artifact and an approved request matching `deploy/operations/graph-upgrade-contract.example.env`. The request records change ID, release transition, PostgreSQL major/minor, Atlas version, expected target role-graph contract, reviewed SQL SHA-256, approver, and UTC approval time.

Phase 2 ships the contract and runner but no mutation SQL. The current exact verifier accepts only `postgresql-18-v1`, so a release that intentionally changes the graph must update the authoritative design, verifier, request contract value, tests, and target release artifacts together.

With applications and Atlas stopped, use a protected mode-`0700` evidence destination:

```text
GRAPH_UPGRADE_REQUEST=/protected/change-123.env \
GRAPH_UPGRADE_MUTATION_SQL=/protected/change-123.sql \
GRAPH_UPGRADE_EVIDENCE_ROOT=/protected/graph-upgrade-evidence \
DATABASE_HOST=postgres \
DATABASE_PORT=5432 \
DATABASE_NAME=nexusrelay \
POSTGRES_USER=nexusrelay_cluster_admin \
POSTGRES_PASSWORD_FILE=/protected/postgres_cluster_admin_password \
NEXUSRELAY_POSTGRES_DOCKER_NETWORK=<compose-backend-network> \
CONFIRM_SERVICES_STOPPED=yes \
deploy/operations/graph-upgrade.sh
```

The runner verifies the request and SQL digest, records the exact before graph, executes reviewed SQL in one transaction without retaining arbitrary SQL output, verifies the exact after graph, and atomically publishes a checksum-protected external evidence directory. It records the transaction result and that Atlas did not execute. A failure leaves no completed evidence directory and Atlas remains blocked.

Cluster-admin use is limited to this short-lived audited runner. Do not mount it into the migration container, application services, or an interactive general-purpose administration session. Preserve the evidence bundle outside the database and alongside the release/change record.
