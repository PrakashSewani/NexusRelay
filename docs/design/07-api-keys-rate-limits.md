# API Keys and Rate Limits Design

## Scope

This design defines gateway API key format, creation, hashing, lookup, ownership, restrictions, status, expiration, revocation, cache invalidation, usage metadata, and distributed request/token rate limits.

## Key Format

A key has a public prefix and random secret:

```text
nrk_<environment>_<lookup-prefix>_<secret>
```

Exact separators and alphabet are implementation details but must be unambiguous and copy-safe. The secret contains at least 256 bits of cryptographically secure entropy. The environment marker reduces accidental cross-environment use but is not authorization.

The lookup prefix is random/non-secret and indexes a small candidate set. It must not encode organization, user, or sequential IDs.

## API Key Data

```text
api_keys
  id                    uuid primary key
  organization_id       uuid not null
  name                  text not null
  lookup_prefix         text not null unique
  key_hash              bytea not null
  hash_algorithm_version smallint not null
  pepper_key_id         text not null
  owner_user_id         uuid not null
  created_by            uuid not null
  status                text not null  -- active, disabled, revoked
  expires_at            timestamptz null
  revoked_at            timestamptz null
  revoked_by            uuid null
  last_used_at          timestamptz null
  last_used_ip          inet null  -- optional/configurable privacy field
  version               bigint not null default 1
  created_at            timestamptz not null
  updated_at            timestamptz not null
```

The owner is attribution/policy metadata, distinct from the creator, and must be an active organization member at creation. Service principals are not a V1 entity; automation keys remain owned by an accountable human member.

Restrictions use normalized join tables:

```text
api_key_model_rules
  organization_id, api_key_id, gateway_model_id

api_key_provider_rules
  organization_id, api_key_id, provider_connection_id
```

V1 semantics are allowlists. No rows means unrestricted within organization policy. Explicit deny rules are deferred to avoid ambiguous precedence.

Rate configuration:

```text
api_key_rate_limits
  organization_id
  api_key_id unique
  requests_per_minute null
  tokens_per_minute null
  token_estimation_mode
  version
```

## Key Hashing

Because API keys are high-entropy random secrets, use a keyed cryptographic hash or HMAC with a server-side pepper, rather than a deliberately slow password hash. This provides fast request authentication and protects hashes if the database alone is compromised.

- Pepper is mounted outside PostgreSQL and versioned.
- Hash input includes the full canonical key.
- Comparison is constant-time.
- `hash_algorithm_version` selects the canonical HMAC input/output contract and `pepper_key_id` selects the retained ring entry used for verification.
- Prefix lookup returns a bounded candidate set; prefix uniqueness is preferred.
- Plaintext key is held only during creation response and authentication parsing.

## Creation Flow

1. Require `api_keys.create_own` for a self-owned key or `api_keys.create_all` for another active membership.
2. Validate name, owner membership, expiry, restrictions, rate limits, and optional budget linkage.
3. Generate random prefix and secret.
4. Hash canonical key with active pepper.
5. Insert key with the active pepper key ID plus restrictions, rate settings, audit event, and invalidation outbox event in one transaction.
6. Return plaintext once in a response explicitly marked non-repeatable.
7. Prevent response caching with appropriate HTTP headers.

The control plane performs key hashing synchronously and receives the API-key pepper ring as defined by ADR 0004. Plaintext never crosses an asynchronous job or internal service boundary.

No endpoint can retrieve or regenerate the same key. Rotation creates a new key and allows explicit revocation of the old key.

## Authentication Flow

1. Parse exactly one bearer credential and enforce maximum length/format before expensive work.
2. Extract lookup prefix without logging the credential.
3. Resolve a cached key descriptor or query by prefix.
4. Verify the full key hash in constant time.
5. Check organization, key status, expiration, and organization status.
6. Check the Redis deny marker and compare cached key version with Redis critical version state before accepting new work.
7. Return an immutable authentication context containing IDs, restrictions, rates, budget references, and config versions.

Public responses generally use the same 401 error for malformed, unknown, disabled, expired, and revoked credentials to avoid credential enumeration. Internal request records distinguish causes when identity could be safely established.

## Immediate Authority Reduction

Revocation, disablement, expiry shortening, model/provider restriction narrowing, ownership invalidation, and any other authority-reducing update use this protocol:

1. Require `api_keys.revoke_own` with ownership enforcement or `api_keys.revoke_all`, then lock the scoped key row.
2. Create a unique mutation token and synchronously establish a temporary Redis deny marker for the key before database commit, following the shared state machine in `02-persistence-tenancy.md`.
3. Apply the denied or narrowed authoritative state, record timestamp/actor where applicable, and increment version.
4. Write audit and critical invalidation outbox events containing the mutation token.
5. Commit. If a rollback is known, remove only the matching temporary marker; an uncertain cleanup leaves the key denied until the lock-aware reconciler resolves it.
6. The outbox consumer promotes/repairs the marker, increments version counters, publishes invalidation, and retains denial until the gateway reloads and can enforce the new descriptor. For a narrowed-but-active key, the marker is removed through safe reactivation only after the new authoritative version is visible.

Gateway security behavior:

- Every new request checks the key deny marker; every fallback dispatch rechecks key/provider/model deny markers.
- Redis counter mismatch forces cache eviction and authoritative reload.
- If Redis is unavailable, new inference authentication and new provider dispatches fail closed because immediate revocation/disablement cannot be established.
- Already committed streams may finish according to their captured authority; they do not start a new fallback attempt while marker state is unavailable.
- Authority reduction does not wait for cache TTL and takes effect for new work once the database mutation commits because the deny marker necessarily predates that commit.

Provider and model critical invalidation use the same pattern.

## Expiration and Status

- Expiration is checked against one captured UTC time per request.
- `expires_at <= now` is expired.
- Disabled keys can be re-enabled through the shared fail-closed reactivation protocol; revoked keys are terminal and cannot be re-enabled.
- Authority-expanding changes commit first and become usable only after version propagation. Authority-reducing restriction/rate changes use the deny-marker protocol above.
- Key names are mutable but do not affect auth cache validity unless included in usage display metadata.

## Ownership and Visibility

Organization-wide key operations require the corresponding `api_keys.*_all` permission. Owner-scoped operations require `api_keys.*_own` and enforce `owner_user_id = actor_user_id`. Creating a key for another active member requires `api_keys.create_all`; `members.manage` alone is insufficient. Viewer has no key-secret lifecycle permission. These resource policies are enforced in control-plane services. List responses include prefix, status, restrictions, owner, creator, expiry, last-used metadata, and timestamps, never hashes.

API-key ownership references the organization membership, not only the global user ID. Membership suspension/removal immediately disables owned keys through the critical marker protocol unless ownership is transferred to another active membership in the same transaction. Historical request facts retain copied user attribution.

## Last-Used Updates

Authentication emits a coalesced activity event rather than writing PostgreSQL per request:

- Redis stores maximum observed use time per key with TTL, or the gateway emits bounded outbox/activity updates.
- Worker periodically flushes monotonic `last_used_at = greatest(existing, observed)` updates.
- Loss of last-used freshness is operationally acceptable and never affects authorization.
- Request/usage facts remain the authoritative detailed usage history.

If storing last-used IP is enabled, retention and UI disclosure are explicit. It is not required for key correctness.

## Request Rate Limiting

### Algorithm

Use an atomic Redis Lua script or Redis function implementing a token bucket or generalized cell rate algorithm. The algorithm returns:

```text
allowed
remaining
retry_after
reset_at
```

Keys are scoped by API key ID and limiter version, not plaintext key. Request allowance is consumed after successful authentication and basic request-shape validation but before expensive routing/provider work. Every authenticated denial after this point creates a terminal request fact asynchronously or in the admission transaction; RPM denial itself is written through a bounded denial-record transaction. Policy-rejected authenticated inference attempts count toward RPM to prevent abuse.

### Headers

Responses include safe rate-limit metadata using documented headers where compatible. HTTP 429 errors distinguish `gateway_rate_limited` from provider rate limits.

## Token Rate Limiting

TPM must account for requests whose final output is unknown at admission.

V1 uses reservation and reconciliation:

1. Generate the request entity ID before admission and use it as the Redis reservation ID.
2. Estimate input tokens and reserve input plus a bounded output allowance derived from the admitted route plan.
3. Atomically reject if minute-window capacity cannot cover the reservation; duplicate reservation of the same request ID is idempotent.
4. After PostgreSQL admission commits, atomically mark the Redis reservation `admitted`. If PostgreSQL admission fails, run an idempotent release for that request ID before returning.
5. On completion, atomically transition `admitted` to `reconciled` using provider-reported actual or explicit estimate. Release and reconciliation leave bounded tombstones so duplicate or late operations cannot recreate allowance.
6. For streaming cancellation/unknown usage, retain a conservative charge according to policy.

Redis reservation states are `reserved`, `admitted`, `reconciled`, and `released`. TTL is greater than the request total deadline plus shutdown and reconciliation grace. A crash in `reserved` before durable admission may temporarily consume capacity but cannot authorize excess use; reconciliation releases it after the conservative grace. Late operations against an expired/tombstoned reservation fail closed and emit an operational error rather than granting fresh capacity.

The exact Redis implementation and oversize-request behavior require load/concurrency tests. It must never represent an estimate as actual usage in PostgreSQL.

## Redis Failure Policy

Recommended V1 default:

- RPM/TPM enforcement fails closed when a configured finite limit exists and Redis cannot perform the atomic decision.
- Keys without rate limits do not require Redis solely for rate decisions, but still follow revocation version policy.
- V1 exposes no fail-open mode. Any future unsafe mode requires a requirements change and ADR.

Rate-limit state is disposable. Redis stores a limiter epoch and bucket/window start. After restart, finite-limit keys fail closed until the next complete configured RPM/TPM window boundary measured from authoritative UTC time, unless a documented conservative reconstruction completes earlier. Responses expose the bounded retry time; keys never receive unconditional fresh allowance.

## Restriction Enforcement

- Model restrictions are checked before returning `/v1/models` and before request persistence/routing.
- Provider restrictions filter route targets, not client-facing model aliases.
- If a permitted model has no permitted targets, return a disclosure-safe model/provider availability error.
- Restriction references require same-organization foreign keys and cannot point to disabled/deleted resources in create/update validation.
- Empty allowlists mean organization policy applies; this semantic is explicit in API responses.

## Audit Events

```text
api_key.created
api_key.updated
api_key.disabled
api_key.enabled
api_key.revoked
api_key.restrictions_changed
api_key.rate_limits_changed
```

Events store key ID, name/prefix metadata, owner/creator IDs, and redacted change fields. Plaintext and hash never appear.

## Verification

- Key entropy/format and one-time plaintext response tests.
- Database/log/audit/Redis scan proving plaintext and hashes are not exposed incorrectly.
- Constant-time hash comparison path review and algorithm-version tests.
- Invalid, expired, disabled, revoked, suspended-organization, and cross-tenant tests.
- Two-gateway immediate revocation and provider/model invalidation tests.
- Atomic RPM tests under high concurrency across replicas.
- TPM reservation/reconciliation, cancellation, expiry, and Redis restart tests.
- Crash-boundary tests after RPM consumption, TPM reservation, PostgreSQL admission, admitted transition, release, and reconciliation.
- Restriction tests for `/v1/models` and route filtering.

## Requirement Coverage

This design satisfies FR-KEY-001 through FR-KEY-012, FR-RATE-001 through FR-RATE-004, API-key-related FR-MODEL/ROUTE requirements, SEC-003, SEC-014, NFR-009, and key lifecycle release acceptance criteria.
