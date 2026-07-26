# NexusRelay Migrations

This is the Atlas versioned migration directory selected by ADR 0007.

Phase 1 intentionally contains no SQL migration. PostgreSQL empty-volume initialization creates the fixed login/role graph and bootstrap ownership boundary in Phase 2. Reviewed application schema migrations begin in Phase 3 and must use the pre-provisioned roles; they must not create or alter roles.

Add monotonically ordered, forward-only `.sql` files. Use one transaction per file unless a reviewed PostgreSQL operation requires an explicit `-- atlas:txmode none` directive and documented recovery. Never edit an applied file; add a new migration and regenerate `atlas.sum`.

The checksum represents the empty SQL migration set. This README is intentionally excluded by Atlas's migration-directory hashing.
