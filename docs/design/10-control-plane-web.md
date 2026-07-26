# Control-Plane API and Web Design

## Scope

This design defines the administrative HTTP API, resource contracts, authorization placement, pagination, errors, dashboard information architecture, server/client boundaries, form behavior, accessibility, and UI state handling.

## Control-Plane API Boundary

All administrative endpoints are versioned under:

```text
/api/control/v1
```

Browser authentication uses the session cookie and CSRF protections described in the identity design. The control plane may later support administrative bearer tokens, but they are not part of V1 unless separately specified.

## API Conventions

- JSON request and response bodies use `application/json`.
- Resource IDs are UUID strings; model keys/provider types remain stable textual identifiers.
- Timestamps use RFC 3339 UTC strings.
- Mutable-resource responses include a strong ETag derived from resource ID/version. Mutations require `If-Match`; missing preconditions return 428 and stale ETags return 412. Request-body `version` is not an alternative concurrency mechanism.
- Collection responses use cursor pagination and return `next_cursor`.
- Filter values are allowlisted and validated.
- Every response includes `x-request-id`.
- Mutation responses set `Cache-Control: no-store`; secret creation responses additionally prevent browser/proxy caching.

Error shape:

```json
{
  "error": {
    "code": "version_conflict",
    "message": "The resource changed. Refresh and try again.",
    "field_errors": [],
    "request_id": "req_..."
  }
}
```

Field errors use stable field paths and codes. Internal errors and inaccessible cross-tenant resource details are not exposed.

The authoritative control-plane contract is a versioned OpenAPI document. It defines every request/response schema, nullability rule, endpoint permission and resource scope, filter, cursor, ETag/precondition, status/error set, and cache policy. Generated server/client types derive from that document; this route inventory is explanatory and does not replace it.

## API Resource Groups

### Authentication and Session

```text
POST   /auth/login
POST   /auth/logout
GET    /session
POST   /session/active-organization
GET    /session/sessions
DELETE /session/sessions/{id}
POST   /session/change-password
```

`GET /session` returns current user, active organization, memberships safe for organization switching, and effective permission keys. It never returns password/session token details.

Current-user session endpoints operate on the caller's global sessions. Tenant administrators end organization access through membership suspension; no tenant endpoint lists or revokes another user's global sessions. `POST /session/change-password` supports the forced first-login flow, rotates session/CSRF state, and follows the identity transaction contract.

### Organization, Members, Roles

```text
GET/PATCH       /organization
GET/POST        /members
GET/PATCH       /members/{id}
POST            /members/{id}/suspend
POST            /members/{id}/reactivate
GET/POST        /roles
GET/PATCH/DELETE /roles/{id}
PUT             /roles/{id}/permissions
GET             /permissions
```

### Providers and Models

```text
GET/POST        /providers
GET/PATCH       /providers/{id}
POST            /providers/{id}/enable
POST            /providers/{id}/disable
POST            /providers/{id}/rotate-secret
POST            /providers/{id}/test
POST            /providers/test-config
GET             /provider-test-jobs/{id}
GET             /providers/{id}/models
GET/POST        /models
GET/PATCH       /models/{id}
POST            /models/{id}/enable
POST            /models/{id}/disable
GET/POST        /models/{id}/targets
PATCH/DELETE    /models/{id}/targets/{target_id}
```

Provider tests create durable worker jobs as described in the provider design. Unsaved secrets are temporarily encrypted, automatically deleted after terminal completion/expiry, and never logged. The submission response is asynchronous rather than holding a control-plane connection open to an upstream provider.

### Routing, Keys, Budgets

```text
GET/POST        /routing-policies
GET/PATCH/DELETE /routing-policies/{id}
POST            /routing-policies/{id}/simulate
GET/POST        /api-keys
GET/PATCH       /api-keys/{id}
POST            /api-keys/{id}/enable
POST            /api-keys/{id}/disable
POST            /api-keys/{id}/revoke
GET/POST        /budgets
GET/PATCH       /budgets/{id}
POST            /budgets/{id}/enable
POST            /budgets/{id}/disable
GET/POST        /prices
GET             /prices/{id}
POST            /prices/{id}/supersede
DELETE          /prices/{id}
GET             /agent-exporters
POST            /agent-configs/preview
POST            /agent-configs/render
```

Routing simulation accepts only synthetic metadata such as operation, capabilities, estimated token counts, and selected model. It must not require actual prompt content.

Price supersession creates a new effective record and closes the predecessor as defined in the usage design. DELETE applies only to a future record that has never been effective and is not referenced by a route/request snapshot.

Agent-config preview/render accepts a verified exporter ID, API key ID, selected gateway model IDs, and exporter-specific validated options. The control plane verifies own/all key visibility and the selected key/model intersection, then delegates to the registry in `15-agent-config-export.md`. During key creation, the UI separately shows the one-time plaintext key and environment-variable setup command.

### Usage, Analytics, Health, Audit

```text
GET /requests
GET /requests/{id}
GET /usage
GET /analytics/summary
GET /analytics/timeseries
GET /analytics/providers
GET /analytics/models
GET /analytics/users
GET /provider-health
GET /audit-events
```

Request detail includes attempts, routing explanation, normalized error metadata, usage, and cost, never prompts/completions.

## Endpoint Permission and Resource Policy

| Resource | Read | Mutate | Additional resource policy |
| --- | --- | --- | --- |
| Organization | `organization.read` | `organization.update` | Top-level organization creation is a deployment-operator command, not a tenant API |
| Members | `members.read` | `members.manage` | Final-owner rules always apply |
| Roles | `roles.read` | `roles.manage` | System Owner role is immutable |
| Providers | `providers.read` | `providers.manage` | Test and rotate require their dedicated permissions |
| Models | `models.read` | `models.manage` | Organization-scoped |
| Routing | `routing.read` | `routing.manage` | Organization-scoped |
| API keys | `api_keys.read_own` or `api_keys.read_all` | corresponding operation-specific `*_own` or `*_all` permission | Own scope enforces `owner_user_id = actor`; all scope is organization-wide |
| Budgets | `budgets.read` | `budgets.manage` | Organization-scoped |
| Pricing | `pricing.read` | `pricing.manage` | Organization-scoped, versioned effective records |
| Requests/usage | `usage.read_own` or `usage.read_all` | none | Own scope filters by keys owned by the actor; all scope is organization-wide |
| Analytics | `analytics.read_own` or `analytics.read_all` | none | Same explicit scope as usage |
| Audit | `audit.read` | none | Owner/Admin only by default role matrix |
| Sessions | authenticated user for own sessions | authenticated user for own revocation/password change | Tenant admins suspend membership instead of revoking global sessions |

Endpoint-specific additions:

| Endpoint/operation | Permission and resource policy |
| --- | --- |
| Provider test and unsaved test config | `providers.test`; unsaved secrets use the encrypted worker-job path |
| Provider secret rotation | `providers.rotate_secret` |
| Provider model discovery/job read | `providers.read`; job must belong to active organization and actor-visible provider |
| Routing simulation | `routing.read`; synthetic inputs only |
| Price supersede/delete | `pricing.manage`; immutable-history rules apply |
| Agent exporter discovery | authenticated session; returns only enabled `contract_verified` exporters |
| Agent preview/render | `api_keys.read_own` with ownership or `api_keys.read_all`; selected models must be in the key intersection |

## Handler and Service Sequence

1. Middleware assigns request ID and applies security headers/size limits.
2. Session middleware authenticates identity. It resolves and requires an active organization only for tenant endpoints; identity-only session, organization-selection, and forced-password endpoints proceed without one.
3. Handler parses and schema-validates transport input.
4. Application service requires atomic permission and applies resource policy.
5. Service executes tenant transaction and domain invariants.
6. Transport maps result/error to stable response.

Handlers do not call SQL repositories directly. Permission checks are not inferred from HTTP method or UI route.

## Pagination and Filtering

- Default page size is conservative, for example 50; maximum is bounded.
- Requests/audit sort by `(occurred_at DESC, id DESC)`.
- Mutable resource lists sort by stable name plus ID or creation time plus ID.
- Cursors include sort values and filter fingerprint so they cannot be reused with incompatible filters.
- Search is bounded prefix/full-text behavior; raw SQL patterns are never accepted.
- Date ranges have maximum spans appropriate to detail versus aggregate endpoints.

## Long-Running Operations

Most mutations are synchronous database operations. Provider tests, historical analytics rebuild, and mass secret re-encryption use job resources. The UI displays queued/running/terminal state and bounded expiry. Automated pricing import is not a V1 operation.

## Web Architecture

Next.js uses the App Router with server components by default. Client components are limited to interactive forms, charts, dialogs, filters, and streaming status where browser state is required.

The browser uses same-origin paths through the reverse proxy. The preferred arrangement is:

```text
browser -> proxy -> control-plane for /api/control/v1/*
browser -> proxy -> web for pages/assets
```

This avoids exposing internal service names and simplifies secure cookies/CORS. Next.js may call control-plane privately for server rendering but does not become the authorization boundary.

## Route Structure

```text
/login
/change-password
/select-organization
/overview
/providers
/providers/[id]
/models
/models/[id]
/routing
/api-keys
/users
/roles
/budgets
/usage
/usage/requests/[id]
/analytics
/audit
/settings
/agent-config
```

Navigation is permission-aware for usability, but direct routes still render backend 403/404 outcomes correctly.

## Page Contracts

### Overview

Displays usage/cost/request trends, provider health summary, budget status, recent errors, and analytics freshness. Cards use actual/estimated indicators.

### Providers

Lists type, name, enabled state, secret-present state, current health, last check, and update version. Detail supports safe non-secret edits, explicit secret rotation, model discovery, test, enable, and disable.

### Models and Routing

Model detail displays stable alias, operation/capability summary, targets, price status, health, priorities, and attached policy. Reordering targets is keyboard accessible and has a non-drag alternative.

### API Keys

Creation is a guided form. The successful response opens a one-time secret dialog that prevents accidental dismissal without warning and makes clear the key cannot be retrieved. The same flow can select allowed models and render/download configuration through a verified agent exporter; the plaintext key remains separate. Lists show prefix only.

### Budgets

Forms explicitly show metric, currency, scope, period, timezone, warning thresholds, hard/warn mode, consumed/reserved/remaining, and current period bounds.

### Usage and Audit

Filterable, cursor-paginated tables support narrow mobile presentation and details. They never expose model content or secret values.

## Data Fetching and Caching

- Session/permission data is fetched server-side for initial rendering.
- Every session-authenticated or tenant-scoped control-plane response uses `no-store`/per-request dynamic behavior and is never placed in a shared Next.js cache. Shared caching is permitted only for explicitly public static assets.
- Resource lists use explicit revalidation after successful mutation, not stale optimistic assumptions for security state.
- Charts may use client fetch with abortable requests and deferred filter input.
- Generated control-plane API types are produced from the source contract; generated files are not manually edited.

## Form Validation

- The browser performs immediate schema-based feedback for usability.
- The control plane repeats all validation and domain invariants.
- Errors map to specific fields and a summary region.
- Secret inputs are never prefilled from stored values.
- Missing secret on normal edit means retain current secret.
- Dangerous actions show resource name, impact, and confirmation; revocation is explicitly irreversible.
- Stale version conflicts prompt refresh/merge instead of silently overwriting changes.

## Loading, Empty, Error, and Partial States

Every page defines:

- Initial loading skeleton that preserves layout.
- Empty state with permitted next action.
- Permission-denied state.
- Service error with request ID.
- Partial/stale analytics state with `data_through`.
- Disabled/unhealthy state without conflating the two.

Client transitions disable duplicate submit and preserve form input after recoverable errors. Provider tests and secret rotation show bounded progress and final normalized outcomes.

## Accessibility

- Semantic headings, landmarks, labels, descriptions, and table headers.
- Full keyboard interaction and visible focus.
- Dialog focus trap, initial focus, escape behavior, and restoration.
- Error summary receives focus after failed submission.
- Status is not conveyed by color alone.
- Charts have textual summaries/tables.
- Responsive critical flows work without horizontal page scrolling; data tables may use scoped scroll regions or card transformations.
- Automated checks are supplemented by keyboard and screen-reader testing for login, provider, model, key, and budget flows.

## Security Headers and Browser Policy

The proxy/web set:

- Content Security Policy with nonces/hashes as required by Next.js.
- `frame-ancestors 'none'` unless an approved embedding requirement appears.
- MIME sniffing protection.
- Strict referrer policy.
- HSTS at the TLS edge in production.
- Permissions Policy minimizing browser capabilities.

CORS is unnecessary for same-origin browser traffic and remains deny-by-default. Public inference CORS is separately disabled unless explicitly configured.

## Verification

- Contract tests for status codes, errors, pagination, filters, versions, and CSRF.
- Permission matrix tests for every endpoint, including cross-organization IDs.
- One-time key response and no-store header tests.
- UI integration tests for critical forms and stale version conflicts.
- End-to-end owner/login/provider/model/key/inference/usage/revocation/budget flow.
- Responsive and accessibility tests for primary pages.
- Tests proving browser/client bundles contain no server secrets or administrative data beyond responses.

## Requirement Coverage

This design satisfies FR-UI-001 through FR-UI-006, control-plane portions of organization/auth/RBAC/provider/model/key/budget/analytics/audit requirements, SEC-004, SEC-012, SEC-013, and TEST-005/TEST-010.
