# NexusRelay Migrations

This is the Atlas versioned migration directory selected by ADR 0007.

Phase 1 intentionally contains no SQL migration. PostgreSQL empty-volume initialization creates the fixed login/role graph and bootstrap ownership boundary in Phase 2. Reviewed application schema migrations begin in Phase 3 and must use the pre-provisioned roles; they must not create or alter roles.

Application objects belong in private schema `nexusrelay` and are created after explicit `SET ROLE nexusrelay_schema_owner`. Atlas revision history belongs in private schema `nexusrelay_migration` and is owned directly by `nexusrelay_migration`. Security-definer functions are schema-qualified in `nexusrelay` and transferred to `nexusrelay_security_definer_owner`.

The Atlas connection is scoped to `search_path=nexusrelay` and stores revision history in `nexusrelay_migration`. Do not remove that scope or use `--allow-dirty` to work around unrelated database objects.

Add monotonically ordered, forward-only `.sql` files. Use one transaction per file unless a reviewed PostgreSQL operation requires an explicit `-- atlas:txmode none` directive and documented recovery. Never edit an applied file; add a new migration and regenerate `atlas.sum`.

The checksum represents the empty SQL migration set. This README is intentionally excluded by Atlas's migration-directory hashing.
