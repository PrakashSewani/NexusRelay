# Model Registry and Routing Design

## Scope

This design covers gateway models, upstream targets, effective capabilities, routing policies, eligibility filtering, deterministic ranking, fallback, health snapshots, configuration caching, and routing observability.

## Data Model

### Gateway Models

```text
gateway_models
  id                    uuid primary key
  organization_id       uuid not null
  model_key             text not null
  display_name          text not null
  description           text not null
  status                text not null  -- enabled, disabled
  operation_set         text[] not null
  default_policy_id     uuid not null
  version               bigint not null default 1
  created_at            timestamptz not null
  created_by            uuid not null
  updated_at            timestamptz not null
  updated_by            uuid not null
  unique (organization_id, model_key)
```

`model_key` is the stable client-facing `model` value. Validation permits a conservative URL/JSON-safe character set and bounded length. Changing it is treated as a breaking rename; V1 should create a new alias instead of mutating an actively used key.

### Route Targets

```text
route_targets
  id                    uuid primary key
  organization_id       uuid not null
  gateway_model_id      uuid not null
  provider_connection_id uuid not null
  upstream_model_id     text not null
  status                text not null  -- enabled, disabled
  priority              integer not null
  weight                integer null
  capability_overrides  jsonb not null
  context_limit         bigint null
  max_output_tokens     bigint null
  version               bigint not null default 1
  created_at            timestamptz not null
  updated_at            timestamptz not null
  unique (organization_id, gateway_model_id, provider_connection_id, upstream_model_id)
```

Targets are explicit. Provider model discovery never creates or deletes targets automatically.

### Routing Policies

```text
routing_policies
  id                    uuid primary key
  organization_id       uuid not null
  name                  text not null
  strategy              text not null
  config                jsonb not null
  max_attempts          integer not null
  retryable_categories  text[] not null
  version               bigint not null default 1
  created_at            timestamptz not null
  updated_at            timestamptz not null
  unique (organization_id, name)
```

V1 strategies:

```text
ordered
lowest_cost
lowest_latency
highest_availability
```

Policy JSON uses a versioned schema and contains only declarative predicates/tie-breakers. It cannot contain executable code or expressions outside the supported grammar. Effective prices are selected by `(organization, provider connection/provider type, upstream model, request start)`; targets do not point to one immutable price version.

## Effective Capability

For each target:

```text
effective = adapter capabilities
            intersect provider/model discovered or static capabilities
            intersect administrator narrowing overrides
```

Limits use the most restrictive non-null value. Administrator configuration cannot widen an unsupported adapter operation. Capability snapshots have a source version and observed time so routing decisions can be explained.

## Routing Input Snapshot

The router receives an immutable per-request snapshot:

```text
organization and policy versions
API key identity and restrictions
gateway model and target definitions
required request capabilities
provider connection statuses
health summaries with observation times
effective prices at request time
current time and remaining deadline
```

Routing does not make ad hoc database calls while ranking. The gateway loads a consistent configuration bundle from cache/database and records relevant versions with the request.

Routing is two-phase:

1. Planning filters and ranks candidates without dispatch, includes effective prices, and calculates the maximum chargeable attempt set.
2. Admission reserves rate/token and budget capacity against immutable attempt slots. Each slot carries target, effective price, timeout ceiling, and whether it is a repeated same-target retry. Execution may consume only admitted slots.

If admission changes eligibility, the plan is rebuilt and re-admitted rather than mutated after reservation.

## Eligibility Pipeline

Targets are filtered in this fixed order, recording one or more exclusion reasons:

1. Gateway model exists and is enabled.
2. Target is enabled.
3. Provider connection is enabled and organization is active.
4. API key permits the gateway model.
5. API key provider restrictions permit the provider connection.
6. Target operation and capabilities satisfy the request.
7. Context/output limits can satisfy known request constraints.
8. Provider health is eligible under policy.
9. Effective price exists when a cost-based policy requires it.
10. Provider-specific temporary circuit/limit state permits an attempt.

Budgets and key rate limits are enforced before route execution and therefore appear as request-level policy decisions rather than target exclusions, except provider-specific quotas that may exclude individual targets.

Example exclusion codes:

```text
model_disabled
target_disabled
provider_disabled
key_model_restricted
key_provider_restricted
operation_unsupported
capability_unsupported
context_limit_exceeded
provider_unhealthy
price_unavailable
provider_temporarily_limited
```

## Ranking Strategies

### Ordered

Sort by explicit target priority ascending, then UUID ascending as a stable tie-breaker. This is preferred-provider-with-fallback behavior.

### Lowest Cost

Calculate estimated request cost using the deterministic routing estimate: known/estimated input plus the request's explicit output limit, or the configured routing default capped by target limits. Admission separately uses the conservative maximum reservation estimate. Both values and formulas are persisted. All compared targets must use the routing policy's configured ISO currency. V1 performs no foreign-exchange conversion; targets priced only in another currency are excluded. Sort by estimated cost, then explicit priority, health score, and UUID. Targets without required pricing are excluded unless policy config defines a deterministic fallback bucket.

### Lowest Latency

Sort by recent exponentially weighted or windowed time-to-first-token for streaming and total latency for non-streaming, selected by operation. Require a minimum sample count; targets below it use a documented neutral/default score rather than always winning. Then sort by availability, priority, and UUID.

### Highest Availability

Sort by bounded-window success rate and health state, then latency, priority, and UUID. Administrative disablement always overrides health.

Health metrics are snapshots, so the same inputs produce the same result. Weighted random distribution is not active unless separately configured with a deterministic request-hash selection scheme.

## Determinism

- All sort keys and tie-breakers are explicit.
- Current time is captured once per routing decision.
- Health and price versions are captured once.
- No map iteration order influences results.
- A routing explanation can be replayed from the stored snapshot metadata without prompt content.

## Attempt Execution

The route plan is an ordered target list bounded by `max_attempts` and remaining deadline.

For each target:

1. Confirm critical provider/key/model deny markers and versions remain valid before dispatch.
2. Compute per-attempt timeout from remaining total deadline and target policy.
3. Persist the attempt in `dispatching` state and commit. Staging in memory is not sufficient.
4. Invoke the adapter only after the durable marker commits.
5. Classify result and whether response commitment occurred.
6. On success, stop.
7. On retryable failure before commitment, apply capped exponential backoff with full jitter, honor a bounded provider `Retry-After`, skip delay when insufficient deadline remains, and consume the next admitted attempt slot.
8. On non-retryable failure, client cancellation, or post-commit failure, stop.

The same provider target is not attempted twice unless policy explicitly permits a same-target retry and the adapter error supplies a safe retry indication. V1 defaults to fallback to the next target rather than same-target retries.

## Retry Classification

Potentially retryable before commitment:

- Connection establishment failures.
- First-byte timeout.
- HTTP 429/provider rate limit when another target exists.
- Provider 5xx/unavailable errors.
- Malformed response before any client-visible output.

Not retryable by default:

- Invalid request or provider authentication/configuration failure.
- Content filtering.
- Unsupported feature.
- Client cancellation.
- Total deadline exhaustion.
- Any failure after stream commitment.

Policy can narrow retry categories but cannot make unsafe post-commit retry legal.

## Health Eligibility

Default state behavior:

- `healthy`: eligible.
- `degraded`: eligible but ranking penalty applies.
- `unknown`: eligible with conservative penalty, useful for new connections.
- `unavailable`: excluded unless an explicit emergency policy permits last-resort use; V1 defaults to exclusion.

Health state has a freshness threshold. Stale health becomes `unknown`, not permanently healthy.

## Configuration Caching

The gateway caches organization routing bundles containing models, targets, provider metadata without secrets, policy definitions, and versions. Provider client instances may cache decrypted credentials only in process memory for a short bounded lifetime and are rebuilt when provider/secret version changes.

Critical version counters:

```text
org:{org}:policy_version
key:{key}:version
provider:{provider}:version
model:{model}:version
```

Redis keys use opaque IDs, never plaintext API keys. On Redis failure, non-security configuration may be used within a bounded TTL and then reloaded from PostgreSQL. New key authentication and provider/model dispatch fail closed because critical deny-marker state cannot be verified. Provider disablement and key revocation policy is detailed in the API-key design.

## Routing Decision Persistence

The request stores:

- Gateway model ID and version.
- Routing policy ID and version.
- Required capabilities.
- Candidate target IDs in ranked order.
- Exclusion codes by target.
- Selected target and attempt sequence.
- Health/price snapshot references or compact safe scores.

Do not store prompt-derived text or tool arguments. Candidate snapshots may be normalized into child rows or stored as bounded versioned JSONB; the final schema should favor query requirements and bounded row size.

## Administrative Behavior

- Disabling a gateway model removes it from new `/v1/models` responses and rejects new requests.
- Disabling a target/provider first establishes a synchronous fail-closed Redis deny marker, then commits database state and durable invalidation. The operation fails without committing when Redis cannot establish the marker. Gateways check deny markers before every new dispatch.
- Existing in-flight requests may finish on their already selected provider unless disablement is marked emergency/cancel-active in a future feature. V1 disablement affects new attempts.
- Target reorder and policy change use optimistic versioning and are audited.
- Model/target validation prevents operation sets with no eligible target at save time, while acknowledging health may later remove all targets.

## Failure Responses

- No visible model: `model_not_found` or disclosure-safe equivalent.
- Model visible but key restricted: `model_not_allowed`.
- No capability-compatible target: `invalid_request` with `unsupported_model_capability`.
- All targets unhealthy/unavailable: `provider_unavailable`.
- Attempts exhausted: apply this deterministic precedence, first matching category wins: client cancellation, invalid request/configuration, content filtered, gateway policy denial, total timeout, provider rate limited, provider unavailable, malformed/upstream error, internal persistence error. Preserve every attempt internally and expose only the sanitized mapped error.

## Verification

- Table-driven eligibility tests for every exclusion reason.
- Deterministic ordering tests with identical snapshots and randomized input map order.
- Strategy tests for missing prices, insufficient health samples, ties, and stale health.
- Retry/fallback tests by error category, attempt limit, deadline, and commitment state.
- Permutation tests for exhausted-attempt error precedence and same-target retry slot accounting.
- Immediate disablement/version invalidation tests across two gateway instances.
- Cross-tenant target-reference database tests.
- Routing explanation contains no model content or secrets.

## Requirement Coverage

This design satisfies FR-MODEL-001 through FR-MODEL-009, FR-ROUTE-001 through FR-ROUTE-012, FR-HEALTH-003/004/007 where consumed by routing, NFR-008, and routing-related release acceptance criteria.
