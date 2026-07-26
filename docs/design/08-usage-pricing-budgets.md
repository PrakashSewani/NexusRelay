# Usage, Pricing, and Budgets Design

## Scope

This design defines durable request and attempt records, normalized usage, pricing versions, cost calculation, hierarchical budget selection, reservation/reconciliation, warning thresholds, and concurrent enforcement.

## Request State Model

```text
received -> rejected
received -> in_progress -> succeeded
received -> in_progress -> failed
received -> in_progress -> cancelled
received -> in_progress -> partial
received -> in_progress -> abandoned  (worker reconciliation)
```

`received` may be represented only within the initial transaction; persisted accepted requests normally begin at `in_progress`. Terminal transitions are one-way. Finalization uses expected state/version to prevent duplicate completion.

## Request Record

```text
requests
  id                    uuid primary key
  organization_id       uuid not null
  request_id            text not null unique
  client_request_id     text null
  api_key_id            uuid not null
  owner_user_id         uuid not null
  operation             text not null
  gateway_model_id      uuid null
  gateway_model_key     text null
  model_version         bigint null
  routing_policy_id     uuid null
  routing_policy_version bigint null
  status                text not null  -- in_progress, rejected, succeeded, failed, cancelled, partial, abandoned
  error_category        text null
  error_code            text null
  http_status           integer null
  stream                boolean not null
  required_capabilities jsonb not null
  candidate_snapshot    jsonb null
  selected_target_id    uuid null
  input_size_bytes      bigint null
  started_at            timestamptz not null
  first_token_at        timestamptz null
  completed_at          timestamptz null
  latency_ms            bigint null
  time_to_first_token_ms bigint null
  finalization_version  bigint not null default 0
```

The record contains no prompt, completion, image, tool argument, or tool result. `candidate_snapshot` is bounded, versioned, and contains only IDs, scores, and exclusion codes.

## Authenticated Rejection Records

Once an API key identity is known, every terminal rejection creates a request fact even when no provider dispatch occurs. Rejection classes include malformed/unsupported protocol fields discovered after authentication, model/provider restrictions, disabled model/organization, RPM/TPM denial, budget denial, no eligible target, and capability mismatch.

- The gateway generates the request entity/public ID before policy enforcement.
- Normal admission writes the request and reservations atomically.
- A denial writes a terminal `rejected` request in a short independent transaction with normalized category/code and safe policy stage.
- If the denial-record transaction fails, no provider dispatch occurs; the gateway returns a controlled service-unavailable error rather than silently losing an attributable rejection.
- Authentication failures for unknown keys cannot be tenant-attributed and are counted only in low-cardinality security metrics.

## Attempt Record

```text
request_attempts
  id                    uuid primary key
  organization_id       uuid not null
  request_id_fk         uuid not null
  sequence              integer not null
  route_target_id       uuid not null
  provider_connection_id uuid not null
  upstream_model_id     text not null
  status                text not null
  reservation_allocation jsonb not null
  attempt_deadline      timestamptz not null
  error_category        text null
  error_code            text null
  upstream_http_status  integer null
  upstream_request_id   text null
  retryable             boolean null
  response_committed    boolean not null default false
  started_at            timestamptz not null
  first_event_at        timestamptz null
  completed_at          timestamptz null
  latency_ms            bigint null
  time_to_first_token_ms bigint null
  unique (organization_id, request_id_fk, sequence)
```

Provider error messages and raw bodies are excluded. Upstream request IDs are stored only when they are documented non-secret identifiers.

## Usage Facts

```text
attempt_usage_records
  id                    uuid primary key
  organization_id       uuid not null
  request_id_fk         uuid not null
  request_attempt_id    uuid not null unique
  api_key_id            uuid not null
  owner_user_id         uuid not null
  gateway_model_id      uuid null
  provider_connection_id uuid not null
  route_target_id       uuid not null
  upstream_model_id     text not null
  input_tokens          bigint null
  output_tokens         bigint null
  total_tokens          bigint null
  cached_input_tokens   bigint null
  reasoning_tokens      bigint null
  input_token_source    text not null
  output_token_source   text not null
  total_token_source    text not null
  cached_token_source   text not null
  reasoning_token_source text not null
  budget_charge_nanos   numeric(30,0) null
  cost_minor_display    bigint null
  currency              char(3) null
  budget_charge_source  text not null  -- provider_reported_compatible, calculated, conservative_estimate, unavailable
  provider_cost_nanos   numeric(30,0) null
  provider_cost_currency char(3) null
  provider_cost_unit    text null
  pricing_version_id    uuid null
  occurred_at           timestamptz not null
```

One attempt usage fact exists for every dispatched attempt, including failed attempts that may be chargeable. Request-level usage and cost are derived as the sum of attempt facts and may be materialized in a separate `request_usage_totals` row for query performance. This makes provider attribution, fallback cost, budget reconciliation, and idempotency explicit without double-counting.

## Usage Source Precedence

1. Provider-reported token dimensions are authoritative for fields supplied.
2. Missing token fields may be estimated using a provider/model-specific tokenizer or documented heuristic.
3. Provider-observed cost is stored separately. It becomes the budget charge only when the provider profile verifies its unit and currency and it matches every applicable monetary budget; otherwise the budget charge is calculated from the captured pricing version.
4. Missing/uncertain values remain null with source `unavailable`; they are not set to zero.

Usage normalization records a source per token dimension because providers may report only part of usage. UI and analytics distinguish reported, estimated, and unavailable values.

## Pricing Data

```text
model_prices
  id                    uuid primary key
  organization_id       uuid not null
  provider_connection_id uuid null
  provider_type         text not null
  upstream_model_id     text not null
  currency              char(3) not null
  input_price_nanos     numeric(30,0) null
  output_price_nanos    numeric(30,0) null
  cached_input_price_nanos numeric(30,0) null
  reasoning_price_nanos   numeric(30,0) null
  per_request_nanos     numeric(30,0) null
  token_unit            bigint not null default 1000000
  effective_from        timestamptz not null
  effective_to          timestamptz null
  source                text not null
  version               bigint not null
  created_at            timestamptz not null
```

Prices use integer nanos of the currency unit per `token_unit` tokens; V1 defaults to and validates one million. Connection-specific prices take precedence over provider-type fallback prices. PostgreSQL exclusion constraints over normalized scope keys enforce non-overlapping effective ranges for both nullable generic scope and connection-specific scope; transaction-only validation is insufficient under concurrency.

The pricing record effective at request start is captured for cost-based routing and final calculation. Later price changes do not alter historical cost. V1 pricing is administered manually through versioned control-plane records protected by `pricing.manage`; automated provider price import is future work.

Effective price records are immutable. A price change creates a successor record and atomically closes the predecessor's `effective_to`; a future record that has never become effective may be deleted/replaced only through a version-checked transaction. Historical rows are never patched in place.

## Cost Calculation

Provider profiles declare whether cached input is a subset of input and whether reasoning is included in output. The gateway converts reported dimensions into canonical non-overlapping billable quantities before pricing. For example, when cached input is a verified subset, uncached input is `max(input_tokens - cached_input_tokens, 0)` and cached input is priced separately. Reasoning is separately priced only when it is not already included in the provider's output charge; otherwise it remains analytical metadata.

For each non-overlapping category:

```text
cost_nanos = ceil(tokens * price_nanos_per_unit / token_unit)
```

`budget_charge_nanos` is retained whenever determinable. Potentially billable attempts that lack authoritative usage receive a conservative estimate rather than null/zero reconciliation. Category costs and per-request fees are summed in nanos without per-request rounding to minor units. `cost_minor_display` is derived using half-up rounding only for display/export; budgets and analytics accumulate nanos and round only at presentation boundaries.

Cross-currency budgets are not supported in V1. A monetary budget's currency must match all charged routes or those routes are ineligible for that budget/policy combination.

## Budget Data

```text
budgets
  id                    uuid primary key
  organization_id       uuid not null
  scope_type            text not null  -- organization, membership, api_key
  metric                text not null  -- monetary, tokens
  currency              char(3) null
  limit_amount          numeric(30,0) not null
  period                text not null  -- daily, monthly
  timezone              text not null
  enforcement           text not null  -- warn, hard
  status                text not null
  version               bigint not null default 1
  created_at            timestamptz not null
  updated_at            timestamptz not null

organization_budget_scopes
  organization_id       uuid not null
  budget_id             uuid primary key

membership_budget_scopes
  organization_id       uuid not null
  budget_id             uuid primary key
  membership_id         uuid not null

api_key_budget_scopes
  organization_id       uuid not null
  budget_id             uuid primary key
  api_key_id            uuid not null
```

`budgets` is the single identity referenced by thresholds, periods, and reservations. Exactly one scope row must exist and match `scope_type`; service transactions and deferred constraint triggers enforce that invariant. Scope tables use composite organization-aware foreign keys to `budgets` and the referenced membership/API key. Membership scope is used instead of a bare global user ID. Active-row uniqueness is one budget per `(scope identity, metric, period)`. For monetary budgets, `limit_amount` is integer currency nanos; API/UI convert configured minor units to nanos. For token budgets, it is integer tokens.

Thresholds:

```text
budget_thresholds
  organization_id
  budget_id
  percentage_basis_points integer
  unique (budget_id, percentage_basis_points)
```

Basis points avoid floating point percentages.

## Period Calculation

- Daily: local midnight to next local midnight in budget timezone.
- Monthly: first local calendar day to first day of next month.
- Boundaries are converted to UTC for storage and queries.
- IANA timezone rules handle DST; a day may not equal 24 hours.
- The period containing the captured request admission time determines the budget window.

## Applicable Budgets

For an authenticated request, load all enabled budgets for:

- Organization.
- API key owner user.
- API key.

Every hard budget must authorize the reservation. Warning-only budgets never reject. Multiple budgets at the same scope/metric are allowed only if explicit semantics are defined; V1 should enforce one active budget per `(scope, metric, period)` to avoid ambiguity.

## Budget Ledger and Reservations

Durable budget correctness lives in PostgreSQL:

```text
budget_periods
  id                    uuid primary key
  organization_id       uuid not null
  budget_id             uuid not null
  period_start          timestamptz not null
  period_end            timestamptz not null
  consumed_amount       numeric(30,0) not null default 0
  reserved_amount       numeric(30,0) not null default 0
  version               bigint not null default 1
  unique (budget_id, period_start)

budget_reservations
  id                    uuid primary key
  organization_id       uuid not null
  request_id_fk         uuid not null
  budget_id             uuid not null
  budget_period_id      uuid not null
  reserved_amount       numeric(30,0) not null
  actual_amount         numeric(30,0) null
  actual_source         text null
  status                text not null  -- active, reconciled, released, expired
  expires_at            timestamptz not null
  created_at            timestamptz not null
  reconciled_at         timestamptz null
  unique (request_id_fk, budget_id)
```

### Planning and Reservation Transaction

Budget admission follows route planning, not dispatch:

1. Build an immutable candidate plan containing only policy/capability/health-eligible targets, effective prices, and explicit maximum chargeable attempt slots, including repeated same-target slots when permitted.
2. Compute request estimates for every applicable budget metric across that plan.
3. Reserve budgets and persist the request/routing snapshot atomically.
4. Execute only targets represented by the admitted plan. If configuration invalidation makes the plan unusable, reject/release and re-plan rather than silently expanding cost exposure.

Within the transaction:

1. Determine period boundaries once.
2. Sort budget IDs to establish deterministic lock order.
3. Insert/get period rows and lock them `FOR UPDATE` in sorted order.
4. For each hard budget, verify `consumed + reserved + estimate <= limit`.
5. Insert reservation rows and increment `reserved_amount`.
6. Create request row, immutable plan snapshot, and outbox event.

If any hard budget fails, no reservations are committed. A rejected request record is written in a separate/adjusted transaction after identity is known, with budget-denial metadata but no model content.

### Estimate

- Token budget: sum the conservative input plus maximum output estimate for every chargeable attempt permitted by the immutable plan; if max output is absent, use the target/model policy default capped by target limits.
- Monetary budget: estimated tokens multiplied by the effective price for every chargeable attempt permitted by the immutable plan. Execution is constrained to that reserved attempt set.
- Multi-attempt potential cost is included conservatively according to policy's maximum chargeable attempts.

This can over-reserve temporarily but prevents concurrency overspend. Reservation size is observable for tuning.

### Reconciliation

Finalization locks period/reservation rows in deterministic order:

1. Determine the authoritative budget charge: compatible provider-reported cost, captured-price calculation, or conservative estimate.
2. Decrease reserved by original reservation.
3. Increase consumed by actual amount.
4. Mark reservation reconciled with source-aware actual.
5. Calculate threshold crossings.
6. Emit warning/aggregation outbox events.

If actual exceeds reservation, consumed may cross the hard limit. The completed request is not retroactively failed; future admissions are rejected. Overshoot metrics and reasons are recorded. An unavailable but potentially billable dispatched attempt is never treated as zero.

## Budget Mutation Rules

- Metric, currency, period, and timezone are immutable after a period or reservation exists; changing them creates a new budget.
- Limit, enforcement, and status updates lock the active period row and use optimistic concurrency.
- Existing reservations remain bound to the captured budget version even when a budget is disabled; they reconcile normally.
- Reducing a limit below `consumed + reserved` is allowed only with an explicit confirmation, immediately blocks new admissions, and emits an audit/operational event.
- Warning budgets do not create reservation rows. They consume actual finalized amounts and show projected values separately from hard-budget reservations.

### Abandonment

Reconciliation worker processes expired active reservations:

- If terminal usage exists, reconcile to it.
- If an attempt may have been billed but usage is unknown, apply a conservative configured estimate and mark source estimated.
- If no upstream attempt started, release fully.
- Mark associated stale request terminal and audit/metric the reconciliation.

Reservation expiry is `request_total_deadline + shutdown_grace + reconciliation_grace`, captured at admission. Gateways do not renew it because no legitimate request may outlive the captured total deadline. The worker does not expire a reservation before this timestamp.

## Redis Role in Budgets

PostgreSQL provides authoritative reservations. Redis may cache period summaries or accelerate warning dashboards, but V1 budget admission does not depend solely on Redis. This prioritizes correctness over maximum throughput and can be optimized later with a reviewed equivalent-safe design.

## Threshold Warnings

- Threshold crossing is detected when consumed moves from below to at/above threshold.
- A unique `(budget_period_id, threshold)` event record prevents duplicates under retry.
- Warnings appear in dashboard and audit/event feeds.
- No email/webhook is required for V1.
- Reservation alone may display projected crossing but does not emit a consumed threshold warning unless product policy explicitly adds it.

## Query and Analytics Behavior

Usage list filters use organization, time range, user, key, provider, gateway model, upstream model, status, and source. Request and audit detail APIs never include model content. Aggregates expose actual and estimated amounts separately or include estimated-share indicators.

## Failure Handling

- Cannot reserve because PostgreSQL unavailable: reject before provider dispatch.
- Finalization unavailable: bounded retry; request remains `in_progress` and reservation active for worker reconciliation.
- Missing pricing for monetary hard budget: target is ineligible or request rejected; never assume zero cost.
- Provider currency mismatch: route target ineligible for that monetary budget.
- Token estimate unavailable: use documented conservative fallback or reject unsupported request.
- Duplicate finalization/outbox delivery: unique constraints and expected-state updates make reconciliation idempotent.

## Verification

- Request state transition and idempotent finalization tests.
- Provider-reported versus estimated usage/source tests.
- Mixed per-dimension provenance, cached/reasoning non-overlap, foreign-currency provider-cost separation, and unavailable-but-billable attempt tests.
- Fixed-precision pricing and rounding boundary tests.
- Effective price interval and historical immutability tests.
- Daily/monthly timezone and DST boundary tests.
- Concurrent hierarchical budget reservation tests with no admission beyond available amount.
- Multi-attempt cost reservation, actual-over-reservation, cancellation, and abandoned request tests.
- Warning threshold exactly-once tests.
- Cross-tenant usage and budget denial tests.

## Requirement Coverage

This design satisfies FR-USAGE-001 through FR-USAGE-011, FR-BUDGET-001 through FR-BUDGET-010, cost-related routing requirements, DATA-001, and usage/budget release acceptance criteria.
