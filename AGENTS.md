# AGENTS.md

This file defines repository-wide instructions for humans and coding agents working on NexusRelay. More specific `AGENTS.md` files may be added inside subdirectories; the nearest file takes precedence for that subtree.

## Project Mission

NexusRelay is a self-hosted, multi-tenant LLM gateway. It exposes an OpenAI-compatible API while centrally managing providers, model aliases, routing, API keys, permissions, budgets, usage, health, analytics, and audit logs.

Read the applicable project documentation before designing or implementing changes:

1. `docs/requirements.md` defines V1 scope, product behavior, and acceptance criteria.
2. `docs/design/README.md` defines approved architectural decisions and links to subsystem low-level designs.
3. The relevant numbered document under `docs/design/` defines data ownership, state transitions, transaction boundaries, APIs, failure behavior, and verification for that subsystem.
4. `docs/design/13-api-compatibility-matrix.md` defines the supported OpenAI-compatible public API surface.
5. `docs/design/14-provider-verification.md` defines provider readiness gates. A provider is not implementation-ready until its authoritative profile is verified.
6. `docs/design/15-agent-config-export.md` defines the generic coding-agent exporter framework, agent-specific verification gates, and key-separation rules.
7. `docs/design/12-requirement-traceability.md` maps requirements to designs and tests.
8. Accepted ADRs under `docs/adr/` define consequential architecture decisions made after these documents.
9. `TODO.md` is the implementation handoff sequence. It tracks delivery work but does not override requirements or design documents.

Treat `docs/requirements.md` as authoritative for product scope and the applicable LLD/ADR as authoritative for implementation behavior. Keep them synchronized whenever an approved decision changes.

## Confirmed Stack

- Go for gateway and control-plane services.
- Next.js and TypeScript for the administrative web application.
- PostgreSQL as the durable source of truth.
- Redis for distributed rate limits, ephemeral coordination, and short-lived cache.
- Docker Compose as the initial deployment target.
- OpenAI-compatible public inference endpoints.
- Configuration-driven Docker Compose deployment. Traefik is the V1 core reverse proxy; Cloudflare Tunnel, Tailscale, and CoreDNS are optional reference-profile components, not mandatory product dependencies.

Do not introduce a different primary framework, database, queue, or deployment platform without an architecture decision record and explicit approval.

## Priorities

When requirements compete, use this order:

1. Tenant isolation and security.
2. Correct policy enforcement and accounting.
3. Public API compatibility and data integrity.
4. Reliability and operability.
5. Maintainability and testability.
6. Performance.
7. Delivery speed.

## Expected Repository Layout

The repository should converge on this structure without creating unnecessary packages:

```text
apps/
  gateway/         Go public inference service or entrypoint
  control-plane/   Go administrative API service or entrypoint
  web/             Next.js dashboard
internal/          Private Go domain and infrastructure packages
migrations/        Forward PostgreSQL migrations
deploy/            Docker Compose and deployment assets
docs/              Requirements, ADRs, API docs, and runbooks
```

The gateway, control plane, and worker are separate binaries and Docker containers built from shared internal packages.

## Working Rules

- Inspect relevant code, tests, migrations, and documentation before editing.
- At the start of an implementation session, read `TODO.md`, select the earliest unblocked item, and confirm its provider/design prerequisites are complete.
- Keep `TODO.md` current as implementation progresses. Check an item only after code, tests, generated artifacts, and required documentation are complete.
- Cite the applicable requirement IDs and design sections in implementation plans, pull requests, or substantial change summaries.
- Make the smallest complete change that satisfies the requirement.
- Do not implement speculative features from the future roadmap.
- Do not add compatibility shims without a concrete persisted-data or external-client need.
- Preserve unrelated work in a dirty working tree.
- Update documentation when behavior, configuration, APIs, schema, or operational procedures change.
- Add an ADR under `docs/adr/` for consequential or difficult-to-reverse architecture decisions.
- Keep commits and pull requests focused. Do not mix broad formatting or refactoring with behavior changes.

## Clarification Before Implementation

Stop and ask the user for clarification before implementation when any of the following is true:

- The requested behavior conflicts with `docs/requirements.md`, an applicable LLD, an accepted ADR, or a security invariant.
- The documentation is missing, ambiguous, or internally inconsistent about externally visible behavior, tenant scope, authorization, data retention, accounting, failure policy, or API compatibility.
- More than one materially different design is valid and the choice affects persisted data, public APIs, deployment topology, security, provider behavior, or future migration cost.
- A provider endpoint, authentication method, capability, model-discovery behavior, usage field, pricing rule, rate limit, or error contract has not been verified from authoritative documentation.
- Implementation would require changing an approved transaction boundary, durability guarantee, fail-open/fail-closed policy, encryption format, routing rule, or streaming commitment rule.
- The change introduces a new dependency, service, database technology, queue, public endpoint, environment secret, permission, or data-retention category not already approved.

When clarification is required:

1. Explain the exact ambiguity or conflict and reference the relevant document/section.
2. Present the smallest set of concrete options with security, compatibility, and operational consequences.
3. Recommend one option when evidence supports it.
4. Do not write implementation code, migrations, or compatibility shims for the unresolved portion until the user decides.
5. Record the approved decision in the requirements, LLD, compatibility matrix, provider profile, or ADR before or with implementation.

Do not stop for routine implementation details already determined by the documentation, local naming choices, or reversible internal refactoring that does not change documented behavior.

## Architecture Boundaries

### Normalized Gateway Domain

- Normalized inference request, response, stream event, usage, capability, and error types must not import provider SDK types.
- Provider-specific translation belongs in provider adapters.
- Do not expose upstream credentials, internal provider IDs, or raw provider errors through public APIs.
- Preserve information needed for OpenAI compatibility, but do not claim support for fields that cannot be honored.

### Provider Adapters

- Implement provider behavior behind a narrow adapter interface.
- Prefer the shared OpenAI-compatible adapter for OpenRouter, Groq, custom endpoints, and other genuinely compatible services.
- Create dedicated adapters only when authentication, payloads, streaming, usage, errors, or capabilities materially differ.
- Every adapter must support cancellation, deadlines, redacted logging, error normalization, usage extraction, and deterministic mock-server contract tests.
- Provider capabilities must be explicit. Never silently drop unsupported tools, structured output, images, embeddings, or other request features.
- Verify provider details from authoritative documentation. Do not invent endpoint paths, headers, models, token accounting, or authentication behavior, especially for Xiaomi MiMo and CommandCode Provider API.

### Routing

- Routing must be deterministic for the same policy inputs and health snapshot unless weighted distribution is explicitly configured.
- Filter ineligible targets before ranking. Consider tenant, key restrictions, model capability, provider state, health, budget, and rate limits.
- Bound retries and fallback attempts by both count and overall deadline.
- Never retry a non-idempotent or streaming request after response content has been committed unless correctness is guaranteed.
- Record candidates, exclusion reasons, selected target, and every upstream attempt without recording prompt or completion bodies.

### Persistence

- PostgreSQL is the durable source of truth. Redis data must be disposable or reconstructable.
- Every tenant-owned row must carry an organization ID unless a reviewed design proves otherwise.
- Repository methods for tenant-owned data must require organization scope; avoid unscoped `GetByID` methods.
- Use database constraints for invariants, uniqueness, and referential integrity in addition to application validation.
- Use transactions for changes that must commit atomically, including configuration plus audit/outbox records.
- Avoid dual writes to independent systems without a transactional outbox or another documented recovery strategy.
- Store timestamps in UTC and make budget timezone semantics explicit.
- Use the documented integer nanos ledger and fixed-precision representations. Minor units are input/display values only. Never use binary floating point for money.

### Migrations

- Migrations are forward-only during normal deployment.
- Never edit a migration that may have been applied; add a new migration.
- Prefer expand-and-contract changes when compatibility across rolling deployments matters.
- Add indexes based on actual access patterns, including organization scope and time-range queries.
- Review destructive or table-rewriting migrations for lock duration and recovery strategy.
- Test migrations from an empty database and from the previous supported schema state.

### Control-Plane API

- Version administrative endpoints explicitly.
- Authenticate first, resolve active organization on the server, then authorize the atomic permission before reading or mutating data.
- The web UI is not a security boundary. Never rely on hidden buttons or client-side route guards for authorization.
- Use stable machine-readable errors and request IDs.
- Paginate unbounded collections. Prefer cursor pagination for append-heavy request and audit tables.
- Validate all inputs at the transport boundary and enforce invariants again in the domain/service layer.

### Web Application

- Use TypeScript strict mode. Do not use `any` to bypass contract design.
- Keep server-only credentials and administrative data out of client bundles.
- Follow the established component and design patterns once they exist; do not introduce parallel UI systems.
- Build accessible forms and interactions with labels, keyboard support, focus handling, clear validation, and explicit loading/empty/error states.
- Ensure critical administration flows work on desktop and mobile.
- Treat generated API types as generated artifacts; update their source schema rather than hand-editing output.

## Security Invariants

These rules are release-blocking:

- Never log or persist plaintext passwords, provider credentials, gateway API keys, browser cookies, authorization headers, prompts, completions, or tool arguments by default.
- Display a gateway API key only once. Store a cryptographic hash and lookup prefix, never reversible plaintext.
- Encrypt provider credentials with authenticated encryption using a master key supplied outside PostgreSQL.
- Hash passwords with Argon2id and reviewed parameters.
- Use secure, HTTP-only, same-site session cookies and CSRF protection for state-changing browser requests.
- Use cryptographically secure randomness for keys, tokens, session IDs, and nonces.
- Use parameterized database queries.
- Treat custom provider URLs as SSRF-sensitive. Validate scheme, redirects, DNS/IP resolution, and private network policy while preserving explicitly configured Ollama/private upstream use.
- Keep CORS deny-by-default.
- Return sanitized errors; never return stack traces or raw upstream bodies to clients.
- Redact secrets at structured logging boundaries, not through ad hoc call-site discipline alone.
- Add negative tests for cross-organization and missing-permission access on every new resource type.

If a requested change conflicts with these invariants, stop and surface the conflict rather than weakening the invariant silently.

## Authentication and Authorization

- Roles are collections of permissions. Never authorize by comparing a role name such as `admin` or `owner` except for narrowly documented system invariants.
- Keep authentication identity separate from organization membership and role assignment.
- A user may belong to multiple organizations.
- Prevent removal or demotion of the final owner.
- Session revocation must take effect server-side.
- Design local authentication behind an identity-provider boundary so OIDC can be added later without replacing RBAC.

## API Key and Policy Enforcement

- Authenticate the gateway key before accepting inference work.
- Enforce expiration, status, model restrictions, provider restrictions, rate limits, and every applicable budget before routing.
- Revocation and provider disablement must affect new requests immediately.
- Distributed counters must be atomic.
- Budget enforcement under concurrency must use reservations and reconciliation or an equivalently safe documented mechanism.
- Internally distinguish gateway policy failures from upstream provider quota and rate-limit failures.

## Usage and Cost Accounting

- Create a request identity early and propagate it through logs, traces, attempts, usage, errors, and response headers.
- Record failed and rejected requests when the organization/key identity is known.
- Provider-reported token usage is authoritative when present.
- Mark estimated usage and cost explicitly; do not represent estimates as actual values.
- Price using a versioned record effective at request time.
- Persist request/usage data durably before relying on asynchronous aggregation.
- Never place user IDs, API key IDs, or request IDs in Prometheus labels.

## Streaming

- Stream incrementally; do not buffer complete upstream responses.
- Use bounded buffers and honor backpressure.
- Propagate client cancellation upstream where possible.
- Separate connection timeout, first-byte timeout, idle-stream timeout, and total deadline.
- Once bytes are committed, do not switch providers in a way that can duplicate or corrupt output.
- Tests must cover disconnects, cancellation, malformed events, partial streams, timeout boundaries, and usage emitted at stream completion.

## Go Conventions

- Follow standard Go formatting and idioms. Run `gofmt` or `goimports` on changed Go files.
- Keep packages cohesive and named for their domain responsibility, not generic layers such as `utils`, `common`, or `helpers`.
- Accept `context.Context` as the first argument for request-scoped and I/O operations. Do not store contexts in structs.
- Wrap errors with useful operation context while preserving errors needed for `errors.Is` and `errors.As`.
- Avoid panics for expected runtime failures.
- Keep interfaces small and define them near the code that consumes them.
- Prefer explicit dependency construction over global mutable state or service locators.
- Do not launch unbounded goroutines. Every background worker needs ownership, cancellation, error handling, and shutdown behavior.
- Close response bodies and other resources deterministically.
- Use typed configuration with startup validation. Never read environment variables throughout business logic.

## TypeScript and Next.js Conventions

- Enable strict TypeScript and linting from the initial scaffold.
- Prefer server components by default and client components only when browser interactivity requires them.
- Keep authorization decisions on the Go backend even if Next.js performs route-level UX checks.
- Avoid duplicating backend domain logic in the browser.
- Use schema-based validation at form and API boundaries.
- Prefer generated API clients/types from the control-plane contract once available.
- Do not add state-management libraries until shared client state justifies one.

## Testing Expectations

Every behavior change must include the smallest test set that proves it and prevents regression.

- Unit test routing, permissions, budget arithmetic, rate-limit decisions, translations, and error mappings.
- Integration test persistence with real PostgreSQL and Redis, not only mocks.
- Contract test every provider adapter against a deterministic local mock server.
- Add opt-in live provider smoke tests; never require secrets for the default test suite.
- Use an official OpenAI SDK against the gateway in compatibility tests.
- End-to-end test critical owner, provider, model, key, inference, usage, revocation, and budget flows.
- Add tenant-isolation tests that create at least two organizations and prove cross-tenant denial.
- Test both streaming and non-streaming behavior.
- Avoid snapshot tests for business-critical behavior when direct assertions are clearer.

Before declaring a change complete, run the relevant subset of:

```text
go test ./...
go vet ./...
golangci-lint run
pnpm lint
pnpm typecheck
pnpm test
pnpm build
docker compose config
```

These commands are target conventions until project scaffolding defines exact scripts. Do not claim a command passed if the command or its configuration does not yet exist.

## Observability

- Production logs must be structured and redacted.
- Emit metrics for rates, latency, active streams, provider outcomes, routing, policy denials, and worker lag.
- Support OpenTelemetry without recording prompt or completion content by default.
- Keep liveness independent from downstream availability; readiness may reflect required dependencies.
- Background jobs need success, failure, duration, and lag instrumentation.

## Configuration and Deployment

- Keep example environment files free of real secrets.
- NexusRelay is a generic product. Never hardcode personal domains, organization names, user identities, provider credentials, private IPs, or machine-specific paths in application behavior.
- All deployment-specific URLs, hostnames, ports, trusted proxies, ingress modes, feature toggles, and secret-file paths must come from typed startup configuration.
- Keep a complete root `.env.example` synchronized with supported Compose/startup configuration. Use safe placeholders and document required versus optional values.
- Treat `.env.example` as the canonical inventory of deployment settings, not as a source of defaults inside business logic. Typed configuration owns defaults and validation.
- Prefer mounted secret files through `*_FILE` settings for production secrets. Direct secret environment values may be supported only when explicitly documented for development and must never appear in examples with real values.
- The core Compose profile must run locally without Cloudflare, Tailscale, or a specific DNS provider. Optional profiles may add public/private ingress integrations.
- Generated client configuration must derive its base URL and key environment-variable name from validated deployment configuration, never a repository constant.
- Validate all required configuration at startup and fail with actionable, non-secret messages.
- Containers should run as non-root with minimal runtime contents.
- Pin production image versions; avoid unbounded `latest` tags.
- Include health checks and graceful shutdown behavior in Compose services.
- Do not make outbound telemetry or hosted ingress mandatory. Core gateway operation must remain self-hosted; optional deployment profiles may depend on third-party ingress/control-plane services and must document that dependency.

## Documentation

Update the applicable documentation when adding or changing:

- Public or control-plane API behavior.
- Environment variables and secrets.
- Database migrations or data retention.
- Provider capabilities and limitations.
- Deployment, backup, restore, or upgrade procedures.
- Permission names or policy behavior.
- Operational alerts and failure recovery.

Use architecture decision records for major choices. An ADR should state context, decision, alternatives considered, consequences, and status.

## Completion Checklist

Before marking work complete, verify:

- The implementation matches `docs/requirements.md` or the requirements were intentionally updated.
- Tenant scope and atomic permission checks are enforced server-side.
- Secrets and model content are absent from logs, traces, errors, fixtures, and persisted metadata.
- Failure, timeout, cancellation, and concurrency behavior have been considered.
- Schema changes include safe migrations and relevant indexes/constraints.
- Tests cover success and important denial/failure paths.
- Documentation and example configuration are current.
- Relevant formatting, linting, tests, builds, and Compose validation pass.
- No unrelated files or user changes were reverted.
