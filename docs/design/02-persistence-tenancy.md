# Persistence and Tenant Isolation

## Storage Responsibilities

PostgreSQL is authoritative for identities, tenant configuration, policy, request facts, usage, budgets, outbox events, health rollups, and audit history. Redis contains only reconstructable cache, distributed counters, version state, leases, and short-lived coordination data.

## Database Access

- Go services use `pgx` connection pools.
- SQL is explicit and type-checked through `sqlc`.
- Handwritten dynamic SQL is limited to cases that cannot be represented cleanly through generated queries and receives the same tenant and parameterization review.
- Domain services own transaction boundaries; repositories do not begin hidden transactions.
- Database errors are mapped into domain categories while preserving wrapped PostgreSQL causes for internal handling.

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

Append-only facts omit update columns and include their own event or occurrence time. Foreign keys involving tenant-owned resources use composite organization-aware constraints where practical to prevent cross-tenant references at the database layer.

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

Separate narrowly privileged database roles exist for:

- Migrations and schema administration.
- Application tenant operations subject to RLS.
- Approved global identity authentication queries.
- Maintenance operations that iterate organizations and establish each tenant context before mutation.
- A narrowly scoped identity-administration function/role for deployment-operator user disablement. It may inspect owner memberships, lock affected organizations in deterministic order, enforce the final-owner invariant, and insert per-organization audit/outbox records, but cannot read arbitrary tenant resources.

Application services never use the migration owner role.

### Global Tables

The following may be global and are not selected through tenant RLS:

- `users` identity records.
- Permission catalog definitions.
- Migration metadata.
- Global provider type definitions if represented in the database.

Memberships bridge global users into organizations and are tenant-protected. Authentication can look up a user by normalized email but cannot infer or return organization data until session and membership resolution.

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

The shared protocol applies to organizations, sessions, API keys, memberships, roles, users, providers, and models. It covers every authority-reducing mutation, including session revocation, disablement, suspension, expiry shortening, restriction narrowing, permission removal, and provider/model ineligibility:

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

- Migrations are forward-only and run through the one-shot `migrate` container.
- The migrator obtains a PostgreSQL advisory lock to prevent concurrent application.
- Migrations record version, checksum, applied time, and execution duration.
- Already-applied migration files are immutable.
- Rolling changes use expand-and-contract: add nullable/new structures, deploy compatible code, backfill, enforce constraints, then remove old structures in a later release.
- Large backfills are resumable worker operations rather than long blocking migration transactions.
- CI tests migration from an empty database and from the previous supported release schema.

## Backup and Recovery

- PostgreSQL backup is the authoritative recovery mechanism.
- Redis is reconstructed after loss; counters may require a documented conservative policy during recovery.
- Restore testing verifies schema, encrypted credential readability with the retained master key, RLS behavior, outbox resumption, and no duplicate aggregate effects.
- Encryption master keys are backed up separately from database backups.

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

This design primarily satisfies FR-ORG-001 through FR-ORG-003, FR-USAGE-008, FR-AUDIT-001, FR-AUDIT-006, FR-CONFIG-002, SEC-005, SEC-011, NFR-001, NFR-007, NFR-009, NFR-010, DATA-002 through DATA-005, and the persistence boundaries in Sections 15 and 16 of the requirements.
