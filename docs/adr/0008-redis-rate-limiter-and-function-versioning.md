# ADR 0008: Redis Rate Limiter and Function Versioning

- Status: Accepted
- Date: 2026-07-26

## Context

NexusRelay must enforce API-key requests-per-minute (RPM) and tokens-per-minute (TPM) limits atomically across gateway replicas. TPM requires a reservation before provider work because final output and fallback usage are unknown at admission. Redis state is disposable, but loss or uncertainty must fail closed for finite limits rather than grant a fresh allowance.

The design also needs a deterministic way to deploy and version the atomic Redis logic. Ephemeral Lua scripts require every client to handle volatile script caches and `NOSCRIPT` recovery. The limiter must remain short-running, idempotent across ambiguous client retries, observable, and independent from plaintext API keys or model content.

The phrase "per minute" is externally visible because fixed, sliding, and continuously refilling algorithms permit different bursts and return different reset times. V1 therefore requires an explicit semantic choice.

## Decision

### Minute Semantics

- RPM and TPM use fixed UTC-minute windows: `[floor(unix_seconds / 60) * 60, window_start + 60)`.
- Redis server time from `TIME`, called inside the function, determines the window. Gateway host clocks do not select or advance limiter windows.
- A request is charged to the window in which its rate admission occurs. Its complete TPM reservation and reconciled request-level token total remain attributed to that admission window even if execution or streaming crosses a minute boundary.
- Fixed windows intentionally permit a burst at a boundary. V1 does not claim sliding-window or continuously refilling token-bucket semantics.
- `reset_at` is the next UTC-minute boundary. `retry_after` is the non-negative ceiling in seconds until that boundary when another minute may change the result.

### Redis Functions

- Require a pinned Redis 7.x release and use Redis Functions invoked with `FCALL`, not application-generated Lua or an `EVAL`/`EVALSHA` script cache.
- Commit each Lua function library as a reviewed source artifact. The application embeds or packages its expected SHA-256 digest and function/library identifiers.
- Give every incompatible library and function set a monotonically versioned name, such as `nexusrelay_limiter_v1`. Function entry-point names also contain the version because Redis function names are globally unique.
- Never use `FUNCTION LOAD REPLACE` for an active version. A new version loads side by side, new application code selects it explicitly, and the prior version remains until no old process or unexpired reservation can call it.
- The worker owns function-library initialization and recovery. It loads an absent expected library, verifies loaded source with `FUNCTION LIST ... WITHCODE`, and fails readiness on a name/digest mismatch rather than replacing unknown code.
- Gateways receive Redis ACL permission for `FCALL` and the required NexusRelay key prefixes. Redis ACLs do not restrict `FCALL` by function name, so every loaded function must enforce its key/argument contract; gateways do not receive `EVAL`, function-load, delete, flush, or other scripting-management permissions.
- Redis readiness for finite-limit traffic requires the exact expected function version and limiter epoch state. A reachable Redis server without those artifacts is not policy-ready.
- Functions use only explicitly supplied keys and bounded integer arguments, perform no network or filesystem work, and finish in constant or bounded time. All numeric limits, estimates, and actual amounts must be non-negative integers within the exact integer range validated for the Redis Lua runtime.

### Key Scope

- Limiter keys use opaque API key IDs, rate-configuration versions, limiter-function versions, and request IDs. They never contain plaintext credentials, user content, provider credentials, or unrestricted client-controlled text.
- RPM and TPM each use a stable per-API-key/configuration bucket hash. UTC window starts are hash fields inside that declared key rather than programmatically generated Redis key names. This lets functions select the authoritative window from Redis `TIME` while complying with Redis's explicit-key rule and retaining older minute fields for late reconciliation.
- Every function receives all accessed keys explicitly, including the deployment limiter-epoch key, stable bucket hash, and request receipt/reservation key. V1's single writable Redis primary permits these atomic multi-key calls.
- The V1 function contract is intentionally not Redis Cluster compatible because a deployment-wide epoch and arbitrary per-key buckets do not share one hash slot. A clustered design requires a separate ADR defining epoch fencing and key placement; it must not silently remove atomic epoch verification.
- A rate-limit configuration change creates a new namespace through its captured configuration version. Authority-reducing changes still use the deny-marker protocol before commit. Existing admitted requests reconcile against their captured old version and window.
- Bucket and receipt/reservation TTLs exceed the minute end plus the maximum request total deadline, shutdown grace, reconciliation grace, and a bounded duplicate-operation tombstone period. Keys are never persistent without a reviewed reason.

### RPM Function

The versioned RPM function accepts the stable bucket-hash key, request receipt key, configured limit, and bounded tombstone TTL. It selects the minute field from Redis server time and returns a versioned tuple containing decision, limit, used, remaining, window start, reset time, and retry delay.

1. If the receipt already exists for the same server-generated request ID and parameters, return its recorded result without consuming again.
2. Reject a mismatched reuse of a request ID as an internal integrity error.
3. If the current used count is below the finite limit, increment it exactly once and record an `allowed` receipt.
4. If capacity is exhausted, do not increment beyond the limit and record a `denied` receipt.
5. Preserve the receipt long enough to make a retry after an ambiguous Redis/client result idempotent.

Each distinct gateway request receives a new request ID and counts independently. Idempotency protects retries of the same internal operation; it does not make separate client retries free. Authenticated policy rejections after RPM admission retain the consumed request allowance.

### TPM Reservation Functions

TPM uses versioned reserve, admit, release, and reconcile functions. The reservation identity is the server-generated request entity ID. Each reservation stores its captured API key, limiter/configuration version, admission window, reserved amount, state, and bounded integrity metadata.

#### Reserve

1. Derive the current UTC minute from Redis `TIME` and validate the supplied finite limit and conservative token estimate.
2. Return the existing result when the same reservation ID and parameters are retried.
3. If `estimate > limit`, return a stable oversize denial without creating a reservation. Waiting for the next identical window cannot make it admissible, so public retry metadata must not imply otherwise.
4. If `used + estimate > limit`, return a capacity denial with the next minute boundary.
5. Otherwise increment the window's reserved/used amount and create the reservation in `reserved` state atomically.

#### Admit and Release

- After the PostgreSQL admission transaction commits, transition `reserved -> admitted` idempotently.
- If PostgreSQL admission does not commit, transition `reserved -> released`, subtract the original reservation from the captured window exactly once, and retain a tombstone.
- Release of an `admitted` reservation is not a normal gateway operation. Stale-request reconciliation decides whether provider work could have become billable before releasing or conservatively reconciling it.

#### Reconcile

- Transition `admitted -> reconciled` exactly once using the final request-level token amount and source category supplied by durable finalization/reconciliation.
- When actual usage is below the reservation, subtract only the difference from the original window; the bucket never becomes negative.
- When actual usage exceeds the reservation, add the overage to the original window and record it as over-limit usage. The completed request is not retroactively failed, and reconciliation never creates allowance.
- `reserved -> reconciled` is allowed only for the worker's documented crash-recovery path after it proves durable admission and determines actual or conservative usage.
- Duplicate calls return the existing terminal result. Invalid state transitions, parameter mismatches, missing expired state, or late operations after the tombstone boundary fail closed and emit an operational error.

Provider-reported token usage remains authoritative when present. The Redis limiter stores only integer quantities needed for enforcement and idempotency; PostgreSQL remains authoritative for usage provenance and durable request facts.

### Limiter Epoch and Redis Loss

- Redis stores a deployment-scoped limiter epoch with a random identifier, initialization time, and `accept_after` boundary.
- Every finite-limit function verifies the active limiter epoch supplied/expected by the caller. Missing, malformed, or mismatched epoch state fails closed.
- After detected Redis data loss or a fresh deployment, the elected worker creates a new limiter epoch whose `accept_after` is the next UTC-minute boundary after initialization. Finite-limit traffic remains denied until Redis server time reaches that boundary and the function library is verified.
- This wait guarantees that no request receives an unconditional replacement allowance in a minute that may already have consumed capacity before state loss. V1 does not attempt to reconstruct ephemeral RPM/TPM buckets from PostgreSQL.
- Keys without finite RPM/TPM limits do not depend on the limiter epoch solely for rate decisions, but all requests still obey the separate critical authorization epoch and deny-marker policy.
- Automatic Redis failover or clustering is not a correctness-neutral deployment change. Any profile that can lose acknowledged writes while retaining apparently valid epoch state, or that cannot atomically access the limiter epoch and bucket, must prove an equivalent conservative detection/fencing policy before it is supported. The V1 core profile uses one writable Redis primary and treats restart/loss as fail-closed epoch recovery.

### Public Results and Observability

- Capacity and RPM denials map to HTTP 429 with `gateway_rate_limited`, `Retry-After` when the next boundary is useful, and bounded documented gateway rate metadata.
- An oversize TPM reservation maps to the same public category with a stable internal/public subcode indicating that the request cannot fit the configured per-minute token limit. It does not fabricate a useful reset time.
- Provider quota failures remain `provider_rate_limited` and never reuse NexusRelay limiter metadata.
- Metrics include function calls, latency, allowed/denied outcomes, reason, function version, epoch readiness, reservation transitions, reconciliation overage/refund, and errors. API key, request, user, and organization IDs are not metric labels.
- Logs may contain opaque IDs under access controls but never limiter key contents that expose unrestricted identifiers, request bodies, or secrets. Function error replies are translated into stable sanitized gateway errors.

## Alternatives

### Continuous Token Bucket or GCRA

Rejected for V1. These algorithms smooth traffic and bound burst capacity, but they make "per minute" a continuously refilling rate, complicate exact TPM refund/reconciliation, and conflict with the already documented complete-window recovery boundary. They may be added later as explicitly named policies rather than silently changing RPM/TPM semantics.

### Sliding 60-Second Window

Rejected because exact trailing-window accounting requires substantially more timestamped state and cleanup work, especially for token reservations and reconciliation. The stricter boundary behavior does not justify the added V1 memory and operational complexity.

### Ephemeral `EVALSHA` Scripts

Rejected because the script cache is volatile across restart, failover, and `SCRIPT FLUSH`; every caller must implement missing-script reload behavior, and SHA-only operations are harder to inspect and version. Redis Functions provide named, persisted, replicated libraries with an explicit initialization contract.

### Redis Modules or Redis Cell

Rejected because they add a non-core server module and do not directly provide NexusRelay's request-ID reservation/admission/reconciliation state machine. The required behavior is small and can be implemented with bounded Redis Functions.

### PostgreSQL-Only Rate Limiting

Rejected because FR-RATE-002 explicitly requires atomic enforcement across gateway replicas using Redis, and per-request database locking would add latency and contention to the inference path. PostgreSQL remains authoritative for durable usage and budgets, not disposable minute counters.

## Consequences

- RPM/TPM behavior has exact, testable fixed-window semantics and useful reset metadata.
- Boundary bursts are possible and must be documented to operators.
- Redis 7.x and a reviewed function-library lifecycle become runtime requirements.
- The worker needs narrowly scoped function-management permission; gateways need only `FCALL` and key-prefix access.
- TPM state is more complex than a counter but supports idempotent crash recovery without representing estimates as actual durable usage.
- Redis loss temporarily denies finite-limit keys until the next complete UTC minute rather than risking over-admission.
- Redis Cluster and automatic HA/failover remain unsupported until a separate durability, key-placement, and fencing review is accepted.

## Verification

- Unit tests cover UTC boundary arithmetic, integer bounds, remaining/reset calculations, oversize estimates, and every valid/invalid reservation transition.
- Real-Redis tests prove atomic RPM and TPM decisions across many gateway clients and replicas.
- Ambiguous-result tests disconnect clients after function execution and prove retrying the same operation does not double-consume, double-release, or double-reconcile.
- Tests prove separate client retries with distinct request IDs consume separately.
- TPM tests cover provider-reported actuals, estimates, refunds, overage, cancellation, fallback totals, streaming completion, stale requests, and tombstone expiry.
- Redis restart/data-loss tests prove finite-limit operations fail closed until the verified function library, new epoch, and next complete UTC-minute boundary are present.
- Function initialization tests cover concurrent workers, absent libraries, exact digest matches, digest mismatches, side-by-side upgrades, and old-version cleanup only after maximum reservation TTL.
- ACL tests prove gateways can use `FCALL` only against permitted NexusRelay key prefixes and cannot use `EVAL` or load, replace, delete, or flush function libraries. Function contract tests prove calling an unintended loaded function cannot escape those key/argument boundaries.
- Topology tests prove V1 rejects unsupported Redis Cluster configuration and that every function declares all keys it accesses.
- Load tests prove functions remain bounded and meet gateway latency targets without blocking Redis for unsafe durations.
- Privacy tests scan function source, arguments, Redis keys/values, errors, logs, and metrics for credentials and model content.

## References

- `docs/requirements.md`: FR-KEY-007, FR-RATE-001 through FR-RATE-004, FR-USAGE-004 through FR-USAGE-011, NFR-001, NFR-004, NFR-005, NFR-009, OBS-004, OBS-007, TEST-001, and TEST-002.
- `docs/design/07-api-keys-rate-limits.md`: API-key rate configuration, admission, failure policy, and TPM lifecycle.
- `docs/design/08-usage-pricing-budgets.md`: durable usage authority and reconciliation provenance.
- `docs/design/09-workers-health-analytics-audit.md`: stale-request reconciliation ownership.
- `docs/design/11-operations-security-testing.md`: Redis operations, readiness, observability, and CI.
- Redis Functions: <https://redis.io/docs/latest/develop/programmability/functions-intro/>
- Redis Lua API: <https://redis.io/docs/latest/develop/programmability/lua-api/>
- Redis `TIME`: <https://redis.io/docs/latest/commands/time/>
- Redis `FCALL`: <https://redis.io/docs/latest/commands/fcall/>
- Redis Cluster key-slot constraints: <https://redis.io/docs/latest/operate/oss_and_stack/management/scaling/>
