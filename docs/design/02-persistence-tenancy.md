# Persistence and Tenant Isolation

## Storage Responsibilities

PostgreSQL is authoritative for identities, tenant configuration, policy, request facts, usage, budgets, outbox events, health rollups, and audit history. Redis contains only reconstructable cache, distributed counters, version state, leases, and short-lived coordination data.

## Database Access

- Go services use `pgx` connection pools.
- SQL is explicit and type-checked through `sqlc`.
- Handwritten dynamic SQL is limited to cases that cannot be represented cleanly through generated queries and receives the same tenant and parameterization review.
- Domain services own transaction boundaries; repositories do not begin hidden transactions.
- Database errors are mapped into domain categories while preserving wrapped PostgreSQL causes for internal handling.
- Gateway, control-plane, and worker use the fixed `nexusrelay_gateway`, `nexusrelay_control_plane`, and `nexusrelay_worker` PostgreSQL `LOGIN` principals with distinct password files. Atlas uses the fixed `nexusrelay_migration` login. `nexusrelay_cluster_admin` is initialization, exceptional audited provisioning, and recovery only; it is never supplied to Atlas or a long-running application process.

## Identifier Strategy

- Application processes generate UUIDv7 IDs before persistence.
- IDs are never reused.
- UUID order is not treated as authoritative event time; `created_at` remains explicit.
- External request IDs use a distinct opaque format with at least 128 bits of randomness and are not database primary keys unless explicitly mapped.

## Common Columns

Mutable tenant resources generally include:

```text
id                 uuid primary key
organization_id    uuid not null
status             text not null
version            bigint not null default 1
created_at         timestamptz not null
created_by          uuid null
updated_at         timestamptz not null
updated_by          uuid null
```

Append-only facts omit update columns and include their own event or occurrence time. Foreign keys involving tenant-owned resources use composite organization-aware constraints; this is mandatory, not a best-effort convention.

## Tenant Context and RLS

### Transaction Protocol

Every tenant operation follows this sequence:

1. Acquire a pooled connection and begin a transaction.
2. Set `SET LOCAL` application context for organization ID and actor where relevant.
3. Execute organization-scoped generated queries.
4. Commit or roll back.

The organization setting is transaction-local, preventing tenant context from leaking when pooled connections are reused. Tenant repository functions require organization ID even though RLS also enforces it.

### RLS Policy Shape

Tenant tables enable and force row-level security for application roles. Policies compare `organization_id` with a validated transaction-local setting. Missing or invalid context denies access rather than falling back to global access.

`organizations` is the tenant root and does not duplicate its ID in an `organization_id` column. Its forced RLS policy compares `organizations.id` with the transaction-local organization setting. Listing organizations for a user is performed through restricted membership resolution, never an unscoped organization-table query.

The V1 database role graph is closed and separates credential-bearing principals from capability-bearing roles:

| Principal or role | `rolcanlogin` | `rolsuper` | `rolbypassrls` | `rolcreaterole` | `rolinherit` | V1 outcome |
| --- | --- | --- | --- | --- | --- | --- |
| `nexusrelay_cluster_admin` | `true` | `true` | `true` | `true` | `true` | Official-image bootstrap superuser; trusted empty-volume initialization, explicit audited role-graph upgrade, and reviewed recovery only |
| `nexusrelay_migration` | `true` | `false` | `false` | `false` | `false` | Owns the Atlas revision table directly |
| `nexusrelay_gateway` | `true` | `false` | `false` | `false` | `true` | Member only of `nexusrelay_gateway_runtime` through the exact edge below |
| `nexusrelay_control_plane` | `true` | `false` | `false` | `false` | `true` | Member only of `nexusrelay_control_plane_runtime` through the exact edge below |
| `nexusrelay_worker` | `true` | `false` | `false` | `false` | `true` | Member only of `nexusrelay_worker_runtime` through the exact edge below |
| `nexusrelay_schema_owner` | `false` | `false` | `false` | `false` | `false` | Owns application schemas, tables, types, and policies |
| `nexusrelay_security_definer_owner` | `false` | `false` | `false` | `false` | `false` | Owns all `SECURITY DEFINER` functions and has no runtime membership |
| `nexusrelay_gateway_runtime` | `false` | `false` | `false` | `false` | `false` | Receives only gateway object/function privileges and owns no SQL objects |
| `nexusrelay_control_plane_runtime` | `false` | `false` | `false` | `false` | `false` | Receives only control-plane object/function privileges and owns no SQL objects |
| `nexusrelay_worker_runtime` | `false` | `false` | `false` | `false` | `false` | Receives only worker object/function privileges and owns no SQL objects |

PostgreSQL 18 stores authorization behavior on each membership edge. Initialization creates exactly these edges:

| Member LOGIN -> granted role | `admin_option` | `inherit_option` | `set_option` | Effect |
| --- | --- | --- | --- | --- |
| `nexusrelay_migration -> nexusrelay_schema_owner` | `false` | `false` | `true` | Migration must explicitly `SET ROLE`; it cannot administer the owner role |
| `nexusrelay_migration -> nexusrelay_security_definer_owner` | `false` | `false` | `true` | Migration must explicitly `SET ROLE`; it cannot administer the owner role |
| `nexusrelay_gateway -> nexusrelay_gateway_runtime` | `false` | `true` | `false` | Gateway inherits privileges but cannot `SET ROLE` or administer membership |
| `nexusrelay_control_plane -> nexusrelay_control_plane_runtime` | `false` | `true` | `false` | Control plane inherits privileges but cannot `SET ROLE` or administer membership |
| `nexusrelay_worker -> nexusrelay_worker_runtime` | `false` | `true` | `false` | Worker inherits privileges but cannot `SET ROLE` or administer membership |

Trusted empty-volume initialization creates all five logins and all five foundational roles before Atlas. It is the sole process allowed to receive all database password files, and secret-generation/init tooling validates that their paths and values are distinct without logging values. Initialization creates the application database, revokes unsafe `PUBLIC` schema creation, grants the migration login only the connection, temporary-object, and schema-bootstrap authority needed for Atlas, and creates or transfers the application schema as needed for first migration. Password literals never appear in migrations or committed SQL.

V1 fixes two private schema names. `nexusrelay` contains application objects and is owned by `nexusrelay_schema_owner`. `nexusrelay_migration` contains Atlas revision history and is owned by `nexusrelay_migration`, which directly owns the revision table. `PUBLIC` has no privileges on either schema. Security-definer functions remain schema-qualified inside `nexusrelay` but are transferred to `nexusrelay_security_definer_owner` by reviewed migrations.

Reviewed migrations explicitly `SET ROLE` to the appropriate owner role. They create schema objects and grant object/function privileges to the three existing runtime roles; they do not create or alter roles. Exact PostgreSQL 18 grant syntax remains subject to implementation tests, but the ownership and edge-option outcomes above are exact. Role-level `INHERIT`/`NOINHERIT` only supplies the default `inherit_option` when a new membership omits it; it does not control an already explicit incoming edge. The foundational `NOLOGIN` roles are `NOINHERIT` as a defensive default for the closed graph, not as the mechanism by which runtime inheritance is denied or allowed. Application services never use either owner role, cannot `SET ROLE` into their runtime group identity, cannot create roles, cannot apply migrations, and cannot inherit another service's runtime role.

No arbitrary future role creation is required for V1. A role-graph change is an exceptional deployment upgrade executed before Atlas by an explicit audited cluster-admin provisioning command/runbook. Normal migrations must fail rather than hide such a graph change. Narrow global authentication, maintenance, queue-claim, identity-administration, and similar capabilities are expressed through object privileges and bounded functions granted to the applicable existing runtime role, not through additional roles.

### Pre-Tenant Lookup Functions

The application roles have no direct unscoped `SELECT` privilege on `users`, `sessions`, `memberships`, `api_keys`, or tenant tables. The closed inventory of pre-tenant read functions is:

| Purpose | Input | Maximum output |
| --- | --- | --- |
| Login identity lookup | normalized email | zero or one authentication descriptor containing user ID, status, password hash, auth version, and password-change flag |
| Session lookup | fixed-length session token HMAC | zero or one session/user descriptor needed for identity validation; no organization permissions or unrelated memberships |
| API-key lookup | validated lookup prefix | a small schema-enforced maximum candidate set containing key hash/algorithm metadata and the IDs/versions needed to establish organization scope |
| Organization selection/list | authenticated user/session ID | a bounded page of that user's membership and organization descriptors only |
| Own-session list | authenticated current session/user ID plus validated cursor/page size | a bounded page of that user's global session metadata only; no token hashes, other users, or tenant permissions |

Each function is owned by `nexusrelay_security_definer_owner`, declared `SECURITY DEFINER`, uses a fixed `search_path` containing only `pg_catalog` plus the private application schema, schema-qualifies referenced objects, and accepts typed parameters rather than dynamic SQL. Creation migrations revoke `ALL` from `PUBLIC` and runtime roles, then grant only `EXECUTE` on each function to the specific existing runtime role that needs it. Functions return named composite types or table columns with a reviewed bounded shape, never arbitrary rows, secrets unrelated to the operation, tenant configuration, or model content. The API-key candidate bound is enforced by uniqueness or a hard function limit and treated as corruption if exceeded.

The closed inventory of mixed-scope mutation functions is separate from the read inventory:

| Purpose | Caller | Input and bounded effect |
| --- | --- | --- |
| Tenant member identity resolve/create | tenant control-plane role after `members.manage` and organization context | normalized email plus optional validated new-user fields; returns one user ID/status descriptor, may insert exactly one global user, and cannot read unrelated memberships or mutate an existing identity |
| Deployment-operator global user disable | deployment identity-administration role | user ID, expected auth version, actor/correlation metadata; locks affected organizations in deterministic order, enforces final-owner safety, disables exactly one global user, and inserts the required per-organization audit/outbox records atomically |
| Global failed-login audit append | authentication role | bounded keyed identifier hash, normalized network metadata, outcome code, and correlation ID; inserts one restricted global security event and returns no identity or tenant data |
| Own-session revoke/logout | authenticated identity role | current user ID, target session ID, expected session version, reason, and correlation metadata; revokes exactly one session owned by that user, writes its global security audit event, and cannot affect another user's session |

Mixed-scope mutation functions are also owned by `nexusrelay_security_definer_owner` and use the same fixed-`search_path`, schema qualification, typed-input, `PUBLIC` revocation, and per-function runtime-role `EXECUTE` rules as read functions. Their transaction and disclosure behavior is not broadened through a generic global-user or audit function. Function ownership cannot be held by a service login, the migration login is not used at runtime, and callers cannot alter the effective `search_path`. Migration and integration tests inspect `pg_proc`, ownership, ACLs, volatility/security flags, and result shapes/effects; prove direct table access is denied; exercise malicious inputs; and prove no function crosses user or organization scope. Any new pre-tenant or mixed-scope purpose requires design review and an explicit function, grant, bound, and negative test rather than broadening an existing function or adding a role.

### Global Tables

The following may be global and are not selected through tenant RLS:

- `users` identity records.
- Permission catalog definitions.
- Migration metadata.
- Global provider type definitions if represented in the database.

Memberships bridge global users into organizations and are tenant-protected. Authentication can look up a user by normalized email but cannot infer or return organization data until session and membership resolution.

### Relationship Matrix

Every tenant parent exposes a candidate key or unique constraint on `(organization_id, id)`. Every tenant child stores `organization_id` and references tenant parents with a composite foreign key `(organization_id, parent_id) -> parent(organization_id, id)`. A redundant tenant ID is intentional defense in depth with RLS.

| Child relationship | Required database constraint |
| --- | --- |
| Membership to organization and global user | `organization_id -> organizations.id`; `user_id -> users.id` is the explicit global-table exception |
| Membership to role | `(organization_id, role_id) -> roles(organization_id, id)` |
| Role permission to role and global permission | composite role FK; `permission_key -> permissions.key` is the explicit global-catalog exception |
| Provider secret/job/health row to provider connection | composite provider FK |
| Gateway model, target, and routing row relationships | composite FKs for every tenant-owned model, provider, target, and policy reference |
| API key owner and creator attribution | `(organization_id, owner_membership_id) -> memberships(organization_id, id)`; creator membership references are composite when stored, while immutable global actor user IDs may additionally reference `users` |
| API-key model/provider/rate/budget rows | composite FKs to the key and every referenced tenant model/provider/budget |
| Request, attempt, usage, price, budget, reservation, analytics, and audit tenant references | composite FKs for mutable/authoritative tenant resources; immutable copied attribution may reference global `users` in addition to copied membership ID |
| Outbox event/delivery | delivery references the global event ID; tenant event producers must set `organization_id`, and payload loading occurs under that tenant context |

Permitted non-composite references are only: tenant root `organization_id -> organizations.id`, references to explicit global tables (`users`, `permissions`, provider-type catalog, migration/global cryptographic state), and immutable external/public identifiers that are not tenant resource foreign keys. A nullable tenant parent still uses the composite FK when present. A migration introducing or changing a tenant relationship must update this matrix or document why an existing row applies, and tests must attempt a cross-organization insert/update and expect a database constraint failure independent of RLS.

## Transaction Boundaries

### Administrative Mutation

One transaction performs:

1. Tenant context and permission-related invariant recheck.
2. Resource mutation with optimistic version condition when applicable.
3. Immutable audit event insertion.
4. Outbox event insertion for cache invalidation or downstream work.
5. Commit.

Permissions are checked before opening the mutation transaction and critical invariants are rechecked inside it to prevent time-of-check/time-of-use races.

### Inference Start

A short transaction creates:

- Request row in `in_progress` or policy-rejected terminal state.
- Route decision snapshot or rejection metadata.
- Budget reservation references where applicable.
- Initial outbox event.

It commits before any upstream provider call.

### Attempt Dispatch and Completion

Before every upstream dispatch, a short transaction inserts the attempt in `dispatching` state with its sequence, target, reservation allocation, start time, and deadline. The provider call begins only after commit. This marker is mandatory and gives reconciliation durable evidence that provider billing may have started.

After an attempt completes or fails, another short transaction updates its terminal state and inserts any provider-reported or estimated attempt usage. A later successful fallback does not erase cost or usage from failed chargeable attempts.

### Request Finalization

The finalization transaction atomically:

- Verifies every attempt has a terminal or explicitly reconcilable state.
- Computes request-level totals from attempt usage facts.
- Updates the request to exactly one terminal state.
- Reconciles budget reservations.
- Inserts outbox events for aggregation, passive health, and warnings.

Finalization is idempotent using request state/version and unique constraints on attempt sequence and usage fact identity.

## Transactional Outbox

Suggested columns:

```text
outbox_events
  id                  uuid primary key
  organization_id     uuid null
  topic               text not null
  aggregate_type      text not null
  aggregate_id        uuid not null
  aggregate_version   bigint null
  payload              jsonb not null
  created_at          timestamptz not null

outbox_deliveries
  event_id             uuid not null
  consumer             text not null
  available_at         timestamptz not null
  attempt_count        integer not null default 0
  claimed_by           text null
  claim_expires_at     timestamptz null
  processed_at         timestamptz null
  last_error_code      text null
  primary key (event_id, consumer)
```

Payload schemas are versioned per topic. Payloads contain IDs, versions, timestamps, and redacted metadata only. The mutation transaction inserts one delivery for each required consumer. A partial index on unprocessed deliveries ordered by `available_at` supports claims.

Workers use a narrow queue-claim role that may read delivery identity, topic, and organization ID globally but not arbitrary tenant payloads or tenant tables. After claiming, the worker starts a tenant transaction, sets organization context, and loads/processes the event payload. Global security events use a separate restricted path and never enter tenant-facing audit queries. An event is fully processed only after all required deliveries complete. Failures increment attempts and schedule bounded exponential backoff with jitter. Permanently failed deliveries remain queryable, alertable, and manually replayable through an audited operator command; they are not silently discarded.

Outbox database privileges are separated by contract. Tenant producer roles may insert an event and its registry-derived delivery rows only inside the same tenant transaction as the source mutation; they cannot claim or complete deliveries. The queue-claim role can select/update only delivery lease fields and the event header `(id, topic, organization_id, created_at)`, never `payload` or tenant rows. After claim, a tenant consumer role sets transaction-local organization context and loads the exact event by `(organization_id, event_id)` under forced RLS. Delivery completion/failure uses a narrow parameterized command keyed by event ID, consumer, claimant, and lease token; it cannot change topic, payload, organization, or another consumer's delivery. Global events use separately granted global producer/consumer functions and an allowlisted topic registry. Tests prove each role cannot perform the other roles' operations, cannot claim payload data, cannot complete an unowned/expired lease, and cannot load a tenant event under a different organization.

## Configuration Invalidation

Configuration mutation commits an outbox event such as:

```text
configuration.provider.changed
configuration.model.changed
configuration.routing.changed
configuration.api_key.changed
configuration.organization_policy.changed
```

The worker increments a Redis version counter and publishes an invalidation notification. Gateway cache entries record observed versions. On lookup:

1. Compare local observed version with the Redis counter, using a short local memoization window.
2. Evict and reload from PostgreSQL when versions differ.
3. Apply a bounded absolute TTL even when no notification is missed.

Security-critical revocation/disablement uses the fail-closed deny-marker protocol in the API-key design. The Redis deny marker is established before database commit; if it cannot be established, the mutation does not commit. The outbox remains the durable fan-out and repair path.

### Critical Deny-Marker State Machine

The shared protocol applies to organizations, sessions, API keys, memberships, roles, users, provider connections, gateway models, and route targets. It covers every authority-reducing mutation, including session revocation, disablement, suspension, expiry shortening, restriction narrowing, permission removal, and provider/model/target ineligibility:

1. The writer acquires the deployment critical-state advisory lock in shared mode, reads the active epoch, then locks the authoritative PostgreSQL resource row. All critical writers, marker reconcilers, and outbox promoters use this order. The epoch rebuilder acquires the same lock in exclusive mode.
2. It generates a unique mutation token and uses an epoch-aware Redis script to write a marker containing resource type/ID, `state=temporary`, mutation token, creation time, and intended denied version. The script rejects a stale epoch. The temporary marker has no automatic expiry; an abandoned false denial is safer than automatic re-enablement.
3. The transaction commits denied database state plus audit and outbox records carrying the token. On a known rollback, the writer deletes only a still-temporary marker with the matching token.
4. The outbox consumer compares the token and promotes the marker to `state=committed`, updates critical version state, and publishes invalidation. Promotion is idempotent.
5. A reconciler examines aged temporary markers. It obtains the same PostgreSQL row/advisory lock used by mutation, so it waits for an in-flight transaction. If authoritative state is denied, it promotes/repairs the marker. If authoritative state is active and no committed mutation with that token exists, it conditionally deletes only the matching temporary marker.

Reactivation is intentionally asymmetric and fail closed:

1. Lock the resource, commit active state plus audit/outbox event, and leave the committed denial marker in place.
2. The outbox consumer acquires the shared critical-state advisory lock, verifies the authoritative active version, advances the critical version, and conditionally removes only the marker for the prior denied version/token.
3. Until that consumer succeeds, new work remains denied. Duplicate delivery and stale reactivation events cannot remove a newer denial.

Redis contains a deployment-scoped critical-state epoch initialized by controlled startup/rebuild. A successfully read absent resource deny marker means "not denied" only when that epoch and the resource's required version state are present and valid. Redis unavailability, a missing/unknown epoch, malformed marker data, or missing required critical version state is unverifiable and fails closed for new authentication/dispatch. Thus ordinary marker absence is not itself a denial, while inability to establish the integrity of the marker namespace is.

After Redis loss, one elected rebuilder acquires the critical-state advisory lock in exclusive mode, creates a new random epoch, and writes all reconstructed deny markers and critical resource versions under an epoch-specific Redis key prefix. The exclusive lock prevents critical commits and promotions during the repeatable-read snapshot and publication window, so no post-snapshot mutation can be omitted. Gateways continue to reject new authentication/dispatch because the published active epoch remains absent or invalid. After count/checksum verification, the rebuilder atomically switches `critical:active_epoch` with a Lua script. Only the pointer switch makes the namespace readable. Old epoch keys expire only after all replicas have observed the new epoch and the maximum cache/request window has elapsed. If rebuild or verification fails, no pointer is published and new security-sensitive work remains denied.

## Optimistic Concurrency

Mutable resources expose a `version`. Control-plane reads return a strong ETag derived from resource ID/version, and update/status-change operations require `If-Match`. A missing precondition returns HTTP 428; a stale ETag returns HTTP 412 after a disclosure-safe scoped check. The version remains visible in administrative representations for diagnostics but is not a second mutation-precondition mechanism.

## Schema and Index Rules

- Foreign keys are explicit and use deliberate delete behavior; default cascade deletion is avoided for audit and usage facts.
- Organization-scoped lookup indexes begin with `organization_id`.
- Time-range indexes use `(organization_id, occurred_at DESC, id DESC)` for cursor queries.
- Partial indexes serve active, unprocessed, or non-revoked subsets when query patterns justify them.
- Check constraints enforce status and numeric ranges in addition to domain validation.
- Secrets and API key hashes are never included in general-purpose JSON columns.
- JSONB is reserved for versioned event payloads, provider-specific non-secret configuration, and snapshots whose shape legitimately varies.

## Migrations

- ADR 0007 selects Atlas versioned migrations through a pinned Atlas Community CLI in the one-shot `migrate` container. Repository-owned ordered SQL files remain the V1 source of truth.
- The committed `atlas.sum` protects migration-directory integrity. CI validates it and fails on generated-file drift.
- Atlas obtains a PostgreSQL advisory lock to prevent concurrent application; lock or validation failure blocks application startup.
- The Atlas revision table records version/description, integrity hash, execution timestamp and duration, statement progress, and failure metadata.
- The Atlas revision table is owned directly by `nexusrelay_migration`; application schemas, tables, types, and policies are owned by `nexusrelay_schema_owner`, and all `SECURITY DEFINER` functions are owned by `nexusrelay_security_definer_owner`.
- Already-applied migration files are immutable. Repairs use new forward migrations; normal deployment never rewrites revision history or runs down migrations.
- Migrations use one transaction per file by default. PostgreSQL-required non-transactional operations need explicit review plus documented partial-failure and retry behavior.
- Normal deployment preserves linear execution and never uses Atlas dirty-database, baseline, non-linear, or skip-lock overrides.
- Rolling changes use expand-and-contract: add nullable/new structures, deploy compatible code, backfill, enforce constraints, then remove old structures in a later release.
- Large backfills are resumable worker operations rather than long blocking migration transactions.
- CI tests migration from an empty database and from the previous supported release schema, checksum tampering, lock contention, transactional rollback, and reviewed non-transactional recovery.
- Atlas connects only as `nexusrelay_migration`, a non-superuser, `NOBYPASSRLS`, `NOCREATEROLE` login whose password file and value are distinct from cluster-admin and runtime secrets. Empty-volume principal/role provisioning precedes Atlas and is Phase 2 deployment initialization, not a schema migration.
- Phase 3 migrations use the pre-provisioned closed graph and grant privileges to existing runtime roles. Initialization and integration tests query `pg_auth_members` and assert exact `admin_option`, `inherit_option`, and `set_option` values for all five edges. They query `pg_roles` and assert the documented `rolcanlogin`, `rolsuper`, `rolbypassrls`, `rolcreaterole`, and `rolinherit` attributes, explicit owner-role use, and denial of cross-process capabilities.

## Backup and Recovery

- PostgreSQL backup is the authoritative recovery mechanism. The Phase 2 reference profile uses a portable logical artifact: a custom-format database dump plus password-free global role definitions. The short-lived cluster-admin credential is permitted only for backup/recovery role export, restore, exact graph verification, trusted empty-volume initialization, and the separately audited exceptional graph-upgrade runner; it is never supplied to Atlas or a long-running service.
- Redis is reconstructed after loss; counters may require a documented conservative policy during recovery.
- Database and cryptographic recovery artifacts are independently checksum-protected and use separate protected storage/security domains. The cryptographic scope is the provider master ring, API-key pepper ring, CSRF ring, and session secret. Database login passwords remain separate protected recovery inputs and are not included in logical role SQL.
- Backup metadata records the NexusRelay release/revision, Atlas version, exact PostgreSQL minor, and role-graph contract. PostgreSQL major upgrades are blocked until the target release provides a reviewed release-specific plan and evidence.
- No backup script performs automatic retention deletion. Operators retain every database/cryptographic pair needed by recovery and key-reference policy and retire artifacts only through an explicit reviewed process.
- The Phase 2 isolated restore harness verifies artifact integrity, logical restore into an empty PostgreSQL 18 cluster, exact closed login/role graph, all edge options and role attributes, schema ownership, protected login-password recovery, and cryptographic artifact recovery. Later phases extend the same harness with encrypted credential readability, RLS, outbox resumption, aggregate idempotency, and complete product startup assertions when those capabilities exist.
- An exceptional role-graph upgrade runs before Atlas from a reviewed SQL artifact whose digest is bound to an approved request. It executes in one transaction and atomically publishes a checksum-protected external evidence bundle containing request, reviewed SQL, before/after exact graph results, and sanitized execution output. Phase 2 supplies the contract and runner but no mutation SQL.

## Failure Handling

- PostgreSQL unavailable before request persistence: reject without upstream dispatch.
- PostgreSQL unavailable after an upstream response: do not retry generation. Persist attempt outcome/final usage with retries bounded by the remaining total deadline and a configured finalization cap. If exhausted, non-streaming returns a sanitized 503 without provider retry. A committed stream closes without a success terminal marker; it may emit only a protocol-defined error event when the endpoint contract permits it. Reconciliation uses the durable `dispatching` marker and treats uncertain streaming commitment as potentially partial and billable.
- Worker crash after side effect but before marking an outbox row processed: handler repeats safely through idempotency.
- Stale `in_progress` rows: worker marks terminal `abandoned` or `failed`, releases reservations, and records reconciliation metadata.
- RLS context missing: database denies tenant access and the service records a security-relevant internal error without tenant data.

## Verification

- Cross-organization repository tests against real PostgreSQL.
- Tests proving missing RLS context denies reads and writes.
- Transaction rollback tests for mutation plus audit/outbox atomicity.
- Outbox duplicate-delivery and worker-crash tests.
- Migration tests from empty and previous schemas.
- Query plan checks for high-volume request/audit paths.
- Organization-root RLS, mixed-tenant outbox claim, global-audit isolation, critical epoch rebuild races, safe reactivation, and post-response finalization-failure tests.

## Requirement Coverage

This design primarily satisfies FR-ORG-001 through FR-ORG-003, FR-USAGE-008, FR-AUDIT-001, FR-AUDIT-006, FR-CONFIG-002, SEC-005, SEC-011, SEC-020 through SEC-022, NFR-001, NFR-007, NFR-009, NFR-010, DATA-002 through DATA-005, and the persistence boundaries in Sections 15 and 16 of the requirements.
