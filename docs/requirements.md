# NexusRelay Requirements

## 1. Document Purpose

This document is the source of truth for the first production release of NexusRelay, a self-hosted LLM gateway. It defines product scope, functional behavior, system constraints, and release acceptance criteria.

Implementation details may evolve, but behavior that conflicts with this document requires an explicit requirements update.

## 2. Product Summary

NexusRelay provides one OpenAI-compatible API endpoint for applications that use multiple LLM providers. It centralizes provider credentials, model aliases, routing, API keys, access control, budgets, usage, health, analytics, and audit history.

Clients authenticate with a NexusRelay API key and request a gateway model name. NexusRelay selects an eligible provider and upstream model, forwards the request, normalizes the response, records the routing decision and usage, and enforces applicable policy.

## 3. Goals

Version 1 must:

- Provide a single base URL and API key format for client applications.
- Expose an OpenAI-compatible API for supported operations.
- Keep upstream provider credentials hidden from clients.
- Support multiple isolated organizations and users.
- Normalize models through a gateway-owned model registry.
- Route requests using explicit, observable policies and deterministic fallbacks.
- Enforce role permissions, API key restrictions, and hard budget limits.
- Record usage, cost, latency, errors, provider health, and administrative actions.
- Be self-hostable with Docker Compose.
- Allow additional providers, auth methods, routing strategies, and storage backends to be added without changing the public gateway contract.

## 4. Non-Goals for Version 1

Version 1 will not provide:

- AI agents or autonomous task execution.
- Workflow orchestration.
- Prompt libraries or prompt engineering tools.
- Vector databases, retrieval-augmented generation, or memory systems.
- MCP server hosting.
- A plugin marketplace.
- Customer invoicing, payment collection, or a full billing system.
- Prompt optimization.
- AI-generated routing decisions.
- Cross-region active-active deployment.
- Kubernetes or Helm deployment artifacts.
- Enterprise SSO, SCIM, or external identity providers. The auth design must permit these later.

## 5. Confirmed Technical Baseline

- Gateway and control-plane backend: Go.
- Administrative web application: Next.js with TypeScript.
- Primary database: PostgreSQL.
- Ephemeral coordination, rate-limit counters, and short-lived cache: Redis.
- Client protocol: OpenAI-compatible HTTP API.
- Initial authentication: local email and password with secure browser sessions.
- Initial deployment: Docker Compose.
- Deployment hostnames, public URLs, trusted proxies, TLS, and optional ingress integrations must be configuration-driven rather than compiled into the application.
- The core Compose profile must support local/private operation without Cloudflare or Tailscale.
- The core Compose profile uses Traefik as its configuration-driven reverse proxy. Optional reference ingress profiles may add Cloudflare Tunnel, Tailscale, and CoreDNS. The initial `prakashsewani.com` profile is documented by ADRs 0001 and 0002 but is not a product-wide requirement.
- Initial provider targets:
  - OpenAI
  - Anthropic
  - Google Gemini
  - OpenRouter
  - Ollama
  - Groq
  - Xiaomi MiMo
  - CommandCode Provider API
  - Custom OpenAI-compatible upstreams

Provider support is considered complete only after the provider-specific capabilities and authentication requirements are verified against its authoritative API documentation. CommandCode Provider API and Xiaomi MiMo integration details must not be guessed; implementation begins with a provider capability investigation.

## 6. Terminology

- **Organization**: The top-level tenant and security boundary.
- **User**: A human identity that can sign in to the dashboard.
- **Membership**: A user's role assignment within an organization.
- **Role**: A named collection of permissions.
- **Permission**: An atomic authorization capability.
- **Gateway API key**: A NexusRelay-issued credential used by a client application.
- **Provider**: An upstream LLM service type, such as OpenAI or Anthropic.
- **Provider connection**: An organization-specific configured instance of a provider, including encrypted credentials and endpoint settings.
- **Upstream model**: A model identifier offered by a provider connection.
- **Gateway model**: A stable client-facing model alias mapped to one or more upstream model targets.
- **Route target**: A provider connection and upstream model pair eligible to serve a gateway model.
- **Routing policy**: Rules used to order or exclude route targets.
- **Request attempt**: One call from NexusRelay to an upstream provider. A gateway request can have multiple attempts when fallback occurs.
- **Usage record**: Normalized token, request, cost, latency, and outcome data for a gateway request.
- **Budget**: A usage or monetary limit scoped to an organization, user, or API key for a time period.

## 7. Primary Users

### 7.1 Owner

Controls organization-wide settings, members, roles, provider connections, policies, budgets, and API keys. An organization must always retain at least one owner.

### 7.2 Administrator

Operates providers, models, routing, users, API keys, budgets, and observability according to granted permissions.

### 7.3 Developer

Creates and manages permitted API keys, views permitted model information, and inspects usage relevant to their work.

### 7.4 Viewer

Has read-only access to explicitly permitted administrative information.

The names above are default roles. Authorization must depend on permissions, not hardcoded role names.

## 8. Functional Requirements

### 8.1 Organizations and Tenant Isolation

- FR-ORG-001: Every provider connection, model, route, API key, budget, usage record, and tenant administrative audit event must belong to exactly one organization. Pre-tenant authentication security events may be global and must use a separate restricted event path.
- FR-ORG-002: A user may belong to one or more organizations through memberships.
- FR-ORG-003: Data access must always be scoped by the active organization on the server; client-supplied identifiers alone must never establish tenant scope.
- FR-ORG-004: An organization owner may update the organization's display name and operational settings.
- FR-ORG-005: Deleting an organization is not required in V1. Disabling or scheduling deletion may be added later.
- FR-ORG-006: After initial bootstrap, creation of another top-level organization is a deployment-operator action in V1. Tenant roles and custom tenant permissions must not grant deployment-global organization creation.

### 8.2 Authentication and Sessions

- FR-AUTH-001: Users must authenticate with email and password before accessing the dashboard.
- FR-AUTH-002: Passwords must be stored using Argon2id with configurable, secure parameters.
- FR-AUTH-003: Browser authentication must use secure, HTTP-only, same-site cookies and server-managed sessions.
- FR-AUTH-004: Login responses must not disclose whether an email address exists.
- FR-AUTH-005: Login attempts must be rate limited by IP and account identifier.
- FR-AUTH-006: Users must be able to sign out and invalidate their current session.
- FR-AUTH-007: Users must be able to list and revoke their own active sessions. Deployment-operator identity administration may revoke another user's global sessions for a documented security action; tenant administrators use membership suspension for organization-scoped access removal.
- FR-AUTH-008: Password reset and email delivery are deferred unless configured for the first release. The initial owner may be bootstrapped through a one-time setup flow or deployment command.
- FR-AUTH-009: Authentication code must expose an internal identity-provider boundary so OIDC can be added without replacing membership or authorization logic.
- FR-AUTH-010: Tenant administrators may suspend or revoke a user's access to the active organization, but may not terminate that user's sessions or access in unrelated organizations. Global session revocation is limited to the user, password/security lifecycle, or a deployment-operator identity action.
- FR-AUTH-011: Browser CSRF tokens must be derived from the current server-side session identity and version using a domain-separated HMAC and a rotatable secret ring. The database must not store a CSRF token or digest; session replacement/version change invalidates previously derived tokens.

### 8.3 Authorization

- FR-RBAC-001: Authorization must use atomic permissions evaluated by the backend.
- FR-RBAC-002: Default roles must include Owner, Administrator, Developer, and Viewer.
- FR-RBAC-003: Owners must be able to create custom roles from supported permissions.
- FR-RBAC-004: A membership may have one role in V1. Multi-role membership may be added later.
- FR-RBAC-005: The system must prevent removal or demotion of the final organization owner.
- FR-RBAC-006: Dashboard visibility must reflect permissions, but backend authorization remains authoritative.
- FR-RBAC-007: At minimum, the permission catalog must represent:
  - Organization settings management
  - User and membership management
  - Role and permission management
  - Provider viewing and management
  - Model registry viewing and management
  - Routing viewing and management
  - API key viewing, creation, update, and revocation
  - Budget viewing and management
  - Usage and analytics viewing
  - Audit log viewing
- FR-RBAC-008: When a resource supports both owner-scoped and organization-wide access, those scopes must be represented by distinct atomic permissions. Default role names must not determine resource scope.

### 8.4 Provider Connections

- FR-PROV-001: Authorized users must be able to create, view, edit, enable, disable, and test provider connections.
- FR-PROV-002: Provider credentials must be encrypted at rest with an application master key supplied outside the database.
- FR-PROV-003: Secret values must never be returned after creation. The UI may only show metadata and a masked indicator.
- FR-PROV-004: Updating non-secret settings must not require resubmitting an existing secret.
- FR-PROV-005: A provider connection may define a custom base URL where the provider type permits it.
- FR-PROV-006: The custom OpenAI-compatible provider must support configurable base URL, API key, optional headers, timeout, and explicitly configured model IDs.
- FR-PROV-007: Disabling a provider connection must immediately make it ineligible for new routing decisions.
- FR-PROV-008: A connection test must report success, normalized failure category, and measured latency without leaking credentials.
- FR-PROV-009: Provider adapters must declare capabilities such as chat completions, responses, embeddings, streaming, tool calls, JSON output, and usage reporting.
- FR-PROV-010: Unsupported request features must fail before upstream dispatch with a clear gateway error unless another eligible route supports them.
- FR-PROV-011: The control-plane service is part of the trusted cryptographic boundary for provider credential creation and rotation and must receive the provider master key ring through a protected mounted secret file. Decryption is permitted only in explicitly reviewed provider-secret workflows and must never be exposed through administrative read APIs.
- FR-PROV-012: Provider master-key activation must be fenced by durable global cryptographic state identifying the active key and epoch. A process whose configured ring disagrees with that state must fail readiness and reject provider-secret writes until the deployment is consistent.

### 8.5 Model Registry

- FR-MODEL-001: Authorized users must be able to create a gateway model with a unique organization-scoped identifier.
- FR-MODEL-002: A gateway model must map to one or more ordered route targets.
- FR-MODEL-003: Each route target must reference a provider connection and upstream model ID.
- FR-MODEL-004: Route targets may define capability metadata, context limits, output limits, and pricing metadata.
- FR-MODEL-005: Gateway model identifiers must remain stable when route targets change.
- FR-MODEL-006: Disabled gateway models must not appear in client model listings and must reject new inference requests.
- FR-MODEL-007: `GET /v1/models` must return only models available to the requesting API key.
- FR-MODEL-008: Automatic provider model discovery may be offered where supported, but manually configured upstream models must remain supported.
- FR-MODEL-009: Model changes must take effect without client configuration changes or service restart.
- FR-MODEL-010: Disabling a route target must immediately make that target ineligible for new attempts across gateway replicas without requiring its gateway model or provider connection to be disabled.

### 8.6 Gateway API Compatibility

The V1 compatibility target is behavioral compatibility for the documented supported subset, not a claim that every OpenAI field or endpoint is implemented.

- FR-API-001: All public inference endpoints must be available under `/v1`.
- FR-API-002: Gateway API keys must be accepted as `Authorization: Bearer <key>`.
- FR-API-003: V1 must support `GET /v1/models`.
- FR-API-004: V1 must support `POST /v1/chat/completions`, including streaming with server-sent events.
- FR-API-005: V1 must support `POST /v1/responses`, including streaming, for the subset that can be normalized across eligible adapters.
- FR-API-006: V1 should support `POST /v1/embeddings` for gateway models whose targets declare embedding capability. It may be released after chat and responses but remains part of V1.
- FR-API-007: Chat and response APIs must support text input/output, system instructions, temperature and token limits where accepted upstream, tool definitions/tool calls, and structured output where target capabilities permit them. Chat Completions additionally supports capability-gated `stop`; the V1 Responses subset rejects `stop` because no normalized Responses stop contract is defined.
- FR-API-008: Streaming responses must preserve the client connection while forwarding normalized upstream events and must stop upstream work when client cancellation is detected where technically possible.
- FR-API-009: The gateway must generate a unique request ID and return it in a response header.
- FR-API-010: Responses should include normalized usage when available. Final usage may be unavailable until a stream completes.
- FR-API-011: Provider-specific fields may be accepted through a clearly namespaced extension mechanism later; unrecognized core fields must not be silently reinterpreted.
- FR-API-012: Error responses must use a stable OpenAI-compatible JSON envelope with a gateway error code and request ID.
- FR-API-013: Request bodies must have a configurable maximum size.
- FR-API-014: The gateway must enforce configurable upstream connection, first-byte, idle-stream, and total timeouts.

### 8.7 API Key Management

- FR-KEY-001: Authorized users must be able to create multiple gateway API keys.
- FR-KEY-002: A plaintext key must be displayed exactly once at creation.
- FR-KEY-003: The database must store a non-reversible key hash, a lookup prefix, and metadata, not the plaintext key.
- FR-KEY-004: A key must have a name, owner, creator, organization, status, and creation time.
- FR-KEY-005: A key may have an expiration time.
- FR-KEY-006: A key may restrict allowed gateway models and provider connections. Empty restrictions mean organization policy applies.
- FR-KEY-007: A key may define request and token rate limits.
- FR-KEY-008: A key may have a scoped budget.
- FR-KEY-009: Authorized users must be able to revoke a key immediately.
- FR-KEY-010: Key use must update last-used metadata without blocking the request path on synchronous reporting work.
- FR-KEY-011: API key authentication must distinguish invalid, expired, disabled, and policy-denied states internally while avoiding unnecessary credential information leakage to clients.
- FR-KEY-012: The control-plane service must receive the API-key pepper ring through a protected mounted secret file so key creation can hash plaintext before its one-time response. Plaintext keys must not cross an asynchronous job or internal service boundary.
- FR-KEY-013: Every API key must reference its owning organization membership. Request and usage facts must immutably copy both the owner user ID and owner membership ID captured at authentication so later membership or ownership changes do not rewrite history.

### 8.8 Routing

- FR-ROUTE-001: Routing must consider gateway model mapping, provider/model capability, connection status, API key restrictions, budget eligibility, provider health, and routing policy.
- FR-ROUTE-002: V1 must support ordered preferred targets with fallback.
- FR-ROUTE-003: V1 must support lowest estimated cost among eligible healthy targets.
- FR-ROUTE-004: V1 must support lowest observed latency among eligible healthy targets.
- FR-ROUTE-005: V1 must support highest availability among eligible targets.
- FR-ROUTE-006: Policy evaluation must be deterministic for the same inputs and health snapshot, except where an explicitly configured tie-break strategy permits weighted distribution.
- FR-ROUTE-007: Fallback may occur for connection failures, timeouts before response commitment, provider rate limits, and configured retryable upstream errors.
- FR-ROUTE-008: The system must not retry or fall back after output has been committed to a client stream unless the protocol can guarantee no duplicated or corrupted output.
- FR-ROUTE-009: Attempts must be bounded by configurable maximum attempts and an overall request deadline.
- FR-ROUTE-010: The routing decision, candidates considered, exclusion reasons, selected target, and fallback attempts must be observable to authorized users.
- FR-ROUTE-011: Raw prompts and model outputs must not be persisted by default.
- FR-ROUTE-012: Custom routing rules in V1 are declarative combinations of supported predicates and strategies, not user-executed code.
- FR-ROUTE-013: For default routing eligibility, `unavailable` is the unhealthy state that excludes a target. `degraded` and `unknown` remain eligible with deterministic ranking penalties, including when stale health is converted to `unknown`; a routing policy may explicitly narrow those defaults but must not silently reinterpret the state names.

### 8.9 Provider Health

- FR-HEALTH-001: The system must perform periodic provider connection health checks.
- FR-HEALTH-002: Passive request outcomes must also contribute to health state.
- FR-HEALTH-003: Health states must include at least unknown, healthy, degraded, and unavailable.
- FR-HEALTH-004: Health evaluation must consider recent success rate and latency over bounded windows.
- FR-HEALTH-005: Health checks must avoid expensive model generation when a lightweight provider operation is available.
- FR-HEALTH-006: Administrators must be able to inspect current health, last check, recent latency, and recent failure rate.
- FR-HEALTH-007: A provider manually disabled by an administrator must remain ineligible regardless of measured health.

### 8.10 Usage and Cost

- FR-USAGE-001: Every accepted gateway request must produce a request record, including failed and rejected requests where identity is known.
- FR-USAGE-002: Usage must be attributable to organization, API key, key owner where present, gateway model, provider connection, upstream model, and request ID.
- FR-USAGE-003: The system must record input tokens, output tokens, total tokens, latency, time to first token when streaming, status, and normalized error category when available.
- FR-USAGE-004: Token counts reported by the upstream provider are authoritative when present.
- FR-USAGE-005: If the provider does not report usage, the system may store an explicitly labeled estimate. It must not present estimated values as provider-reported values.
- FR-USAGE-006: Cost must be calculated from versioned pricing records effective at request time.
- FR-USAGE-007: Actual provider-reported cost, when available, must be distinguishable from gateway-estimated cost.
- FR-USAGE-008: The request path must persist enough data durably to prevent silent usage loss. Derived aggregates may be computed asynchronously.
- FR-USAGE-009: Usage queries must support organization, user, API key, provider, model, status, and time-range filters.
- FR-USAGE-010: Raw prompt and completion content must not be included in usage records.
- FR-USAGE-011: Token provenance must be recorded per usage dimension. Billable token categories must be non-overlapping according to the verified provider profile, and potentially billable dispatched attempts with unavailable usage must never be reconciled as implicit zero.

### 8.11 Budgets

- FR-BUDGET-001: V1 must support budgets scoped to organization, user, and API key. Team budgets are deferred until a team domain is introduced.
- FR-BUDGET-002: A budget must define a monetary or token limit, a calendar period, timezone, and enabled state.
- FR-BUDGET-003: Supported calendar periods must include daily and monthly.
- FR-BUDGET-004: Warning thresholds must be configurable as percentages of the limit.
- FR-BUDGET-005: Hard-limit budgets must reject requests that begin after the tracked limit is exhausted.
- FR-BUDGET-006: All applicable hierarchical budgets must permit a request before it can be routed.
- FR-BUDGET-007: Concurrent requests must reserve estimated usage or spend atomically to limit overshoot. Reservations must be reconciled to actual usage after completion.
- FR-BUDGET-008: Administrators must be able to inspect limit, consumed amount, reserved amount, remaining amount, and period boundaries.
- FR-BUDGET-009: The system must record threshold crossings and hard-limit denials.
- FR-BUDGET-010: V1 warning delivery may be in-dashboard and audit-based. Email or webhook notifications are optional unless separately specified.

### 8.12 Rate Limiting

- FR-RATE-001: The gateway must support API-key request-per-minute and token-per-minute limits.
- FR-RATE-002: Rate limits must be enforced atomically across gateway replicas using Redis.
- FR-RATE-003: Rejected requests must return HTTP 429 with stable error codes and useful retry metadata where available.
- FR-RATE-004: Provider-side quota responses must be categorized separately from NexusRelay rate-limit responses.

### 8.13 Analytics Dashboard

- FR-ANALYTICS-001: Authorized users must be able to view usage and estimated cost over time.
- FR-ANALYTICS-002: The dashboard must show request count, input tokens, output tokens, estimated cost, average latency, and failure rate.
- FR-ANALYTICS-003: The dashboard must show provider utilization, popular gateway models, top users, budget consumption, request trends, and error trends.
- FR-ANALYTICS-004: Analytics must support a time range and filters appropriate to the user's permission scope.
- FR-ANALYTICS-005: Analytics data may be eventually consistent, with a target delay of no more than five minutes under normal operation.
- FR-ANALYTICS-006: Empty, loading, partial-data, and error states must be explicit in the UI.

### 8.14 Audit Logs

- FR-AUDIT-001: Security-sensitive and administrative changes must produce immutable audit events.
- FR-AUDIT-002: Events must include organization, actor, action, target type, target ID, timestamp, request ID or correlation ID, source IP where applicable, and a redacted change summary.
- FR-AUDIT-003: Audited actions must include login outcomes, session revocation, API key lifecycle, provider connection lifecycle and tests, model changes, route changes, budget changes, membership changes, role changes, and organization configuration changes.
- FR-AUDIT-004: Secrets, passwords, API keys, prompts, and completions must never appear in audit payloads.
- FR-AUDIT-005: Authorized users must be able to filter audit events by actor, action, target, and time range.
- FR-AUDIT-006: Audit records must not be editable through application APIs.

### 8.15 Administration UI

- FR-UI-001: The dashboard must support current desktop browsers and responsive use on mobile screens.
- FR-UI-002: Primary areas must include overview, providers, models, routing, API keys, users, roles, budgets, usage, analytics, audit logs, and organization settings.
- FR-UI-003: Dangerous operations must require explicit confirmation and communicate impact.
- FR-UI-004: Secret forms must prevent accidental disclosure and must clearly explain that saved secrets cannot be retrieved.
- FR-UI-005: Server-side permission failures must be presented clearly rather than hidden as generic errors.
- FR-UI-006: Forms must provide accessible labels, keyboard operation, focus management, and actionable validation errors.

### 8.16 Operational Configuration

- FR-CONFIG-001: Deployment-specific settings must be supplied by environment variables or mounted secrets.
- FR-CONFIG-002: Organization configuration stored in the database must take effect without client changes.
- FR-CONFIG-003: Runtime configuration that can be safely refreshed should not require a process restart.
- FR-CONFIG-004: Invalid required deployment configuration must cause startup to fail with a clear, non-secret error.
- FR-CONFIG-005: Sensitive environment values must not be included in logs or diagnostics.

### 8.17 Agent Configuration Export

- FR-EXPORT-001: Authorized users must be able to generate configuration for a supported external coding agent using a gateway API key and a selected subset of gateway models.
- FR-EXPORT-002: Export is implemented through a registry of named, versioned agent exporters behind one provider-neutral internal contract. An exporter is supported only after its authoritative configuration schema and merge behavior are verified.
- FR-EXPORT-003: Every generated connection must derive its base URL from validated `PUBLIC_API_BASE_URL` and target the public OpenAI-compatible `/v1` base.
- FR-EXPORT-004: Generated configuration must expose only models allowed by the selected API key and explicitly selected by the user.
- FR-EXPORT-005: Generated configuration must reference a validated environment-variable name and must not embed plaintext gateway API keys. V1 defaults the shared variable name to `NEXUSRELAY_API_KEY`; an exporter renders the reference using its verified agent-specific syntax.
- FR-EXPORT-006: A complete preview may be generated during key creation while the plaintext key is visible once, but the key must be presented separately as an environment-variable setup command and never inserted into generated configuration.
- FR-EXPORT-007: Existing keys may regenerate non-secret agent configuration because the server does not require or retrieve the plaintext key.
- FR-EXPORT-008: Exported model entries should include safe names and verified context/output/capability metadata when the target agent schema supports them; missing metadata must be omitted rather than invented.
- FR-EXPORT-009: Each exporter must target a documented agent schema/version/retrieval date and repository-pinned schema or fixtures, with deterministic validation tests.
- FR-EXPORT-010: V1 exporters own only the NexusRelay connection/provider entry and selected models. Agent defaults, workflows, tools, permissions, MCP servers, and unrelated settings remain user-managed.
- FR-EXPORT-011: Generated fragments must be merge-safe and must not disable, replace, or mutate unrelated providers or agent settings.
- FR-EXPORT-012: OpenCode is the first required V1 exporter. Kilo, CommandCode, and other agent exporters use the same framework but are not supported until their authoritative profiles reach `contract_verified`.

## 9. Provider Adapter Contract

Every provider adapter must implement or explicitly reject the following responsibilities:

- Validate connection configuration.
- Test connectivity.
- Report static and discovered capabilities.
- List models where supported.
- Translate normalized chat completion requests.
- Translate normalized Responses API requests.
- Translate embedding requests where supported.
- Stream normalized events without buffering the complete response.
- Normalize provider errors into gateway categories.
- Extract provider request IDs, token usage, and cost when supplied.
- Honor request cancellation and deadlines.
- Avoid logging secrets, prompts, or completions.

OpenAI-compatible providers should share a reusable adapter implementation with configuration-driven endpoint and header differences. A dedicated adapter is warranted only when behavior diverges from the compatible contract.

## 10. Error Categories

The gateway must normalize failures into stable categories, including:

- `authentication_error`
- `permission_denied`
- `invalid_request`
- `model_not_found`
- `model_not_allowed`
- `provider_unavailable`
- `provider_rate_limited`
- `gateway_rate_limited`
- `budget_exceeded`
- `request_timeout`
- `content_filtered`
- `upstream_error`
- `internal_error`

Errors must include an HTTP status, human-readable message, stable code, and request ID. Internal stack traces and upstream secrets must never be returned.

## 11. Security Requirements

- SEC-001: All production HTTP traffic must be served behind TLS. Docker Compose may terminate TLS in an included or external reverse proxy.
- SEC-002: Provider secrets must use authenticated encryption at rest and support future key rotation.
- SEC-003: Gateway API keys must use cryptographically secure random generation with at least 256 bits of entropy.
- SEC-004: Browser state-changing operations must be protected from CSRF.
- SEC-005: Database access must use parameterized queries.
- SEC-006: The application must validate outbound provider base URLs and provide an explicit policy for private network access to reduce SSRF risk. Ollama and intentional private upstreams must remain configurable.
- SEC-007: Redirects from upstream provider requests must be disabled or strictly validated.
- SEC-008: Logs and traces must redact authorization headers, cookies, credentials, API keys, prompts, completions, and tool arguments by default.
- SEC-009: Containers must run as non-root users where practical and use minimal runtime images.
- SEC-010: Dependencies and container images must be scanned in CI for known vulnerabilities.
- SEC-011: Authorization and tenant isolation must have automated negative tests.
- SEC-012: CORS must be deny-by-default and explicitly configurable for administrative web origins.
- SEC-013: Security headers must be set for the dashboard, including a restrictive content security policy.
- SEC-014: Sensitive comparisons must use constant-time operations where applicable.
- SEC-015: No telemetry may be sent to third parties by default.
- SEC-016: When public and administrative hostnames are separated, the public inference hostname must route only documented inference endpoints. Administrative web and control-plane APIs must not be reachable through it.
- SEC-017: Administrative exposure mode must be explicit and configurable as local/private or public. Production administrative traffic must use HTTPS, and private profiles must enforce their configured network boundary.
- SEC-018: Trust of forwarding headers must be restricted to configured trusted proxy CIDRs/hops; direct client-supplied forwarding headers must be stripped regardless of ingress provider.
- SEC-019: Configurable provider headers must reject control characters and transport-owned, framing, hop-by-hop, routing, proxy, forwarding, and host headers. HTTP Host and TLS SNI must derive only from the validated provider URL.
- SEC-020: Pre-tenant database lookups must use reviewed narrow `SECURITY DEFINER` functions with fixed `search_path`, parameterized inputs, bounded descriptor outputs, non-login ownership, and `EXECUTE` granted only to the service roles that require each function.
- SEC-021: Tenant-owned relationships must use composite organization-aware foreign keys at the database layer. Exceptions are limited to explicitly documented references to global tables or the organization root and must not permit a tenant row to reference another tenant's resource.
- SEC-022: Fan-out revocation must use parent resource marker/version checks on every relevant authorization path rather than enumerating and mutating all descendant resources.

## 12. Reliability and Performance Requirements

- NFR-001: The gateway must be stateless across replicas except for PostgreSQL and Redis dependencies.
- NFR-002: A single gateway process must shut down gracefully, stop accepting new requests, and allow in-flight requests a configurable drain period.
- NFR-003: Streaming must use bounded memory and apply backpressure.
- NFR-004: Gateway processing overhead, excluding upstream time and network transit, should have a p95 target below 50 ms under the documented baseline load.
- NFR-005: The initial baseline load target is 100 concurrent streaming requests and 500 non-streaming requests per second on reference hardware to be documented during performance testing.
- NFR-006: No single failed provider health check or analytics aggregation job may make the inference API unavailable.
- NFR-007: Database migrations must be versioned, repeatable in CI, and safe to run before application rollout.
- NFR-008: Retry behavior must use bounded exponential backoff with jitter where retry is safe.
- NFR-009: Redis loss must fail closed for finite distributed rate limits and security-critical authorization/invalidation checks in V1. It must not corrupt persistent configuration or usage records. Any future unsafe fail-open mode requires a requirements update and ADR.
- NFR-010: PostgreSQL is the durable source of truth. A database outage must produce controlled service-unavailable errors rather than untracked inference traffic.

## 13. Observability Requirements

- OBS-001: Services must emit structured JSON logs in production.
- OBS-002: Logs must include timestamp, severity, service, request ID, organization ID when known, route, status, duration, and normalized error category.
- OBS-003: The system must expose Prometheus-compatible metrics.
- OBS-004: Metrics must include request rates, latency histograms, active streams, upstream attempts, provider outcomes, routing selections, rate-limit denials, budget denials, queue/job lag, and database/Redis health.
- OBS-005: OpenTelemetry traces must be supported, with prompt and completion content excluded by default.
- OBS-006: Health endpoints must distinguish process liveness from dependency readiness.
- OBS-007: High-cardinality identifiers such as user ID, API key ID, and request ID must not be Prometheus labels.

## 14. Data Retention and Privacy

- DATA-001: Prompt and completion bodies must not be persisted by default.
- DATA-002: Operational request metadata and audit logs must have configurable retention periods.
- DATA-003: Default retention should be 90 days for request-level metadata and one year for audit events, subject to deployment configuration.
- DATA-004: Aggregated historical usage may outlive request-level records.
- DATA-005: Deletion and retention jobs must be observable and auditable.
- DATA-006: Export of usage and audit data is desirable but not required for initial V1 acceptance.

## 15. High-Level Architecture Boundaries

The initial repository should use a monorepo with independently deployable components:

- `apps/gateway`: Go HTTP service for public inference traffic and provider adapters.
- `apps/control-plane`: Go HTTP service for dashboard APIs, authentication, configuration, analytics, and audit access.
- `apps/worker`: Go process for outbox deliveries, provider health checks, analytics aggregation, reconciliation, and retention.
- `apps/web`: Next.js administrative dashboard.
- `internal`: Private Go domain and infrastructure packages for auth, RBAC, providers, models, routing, budgets, usage, audit, and observability.
- `deploy`: Docker Compose and deployment configuration.
- `migrations`: PostgreSQL schema migrations.
- Optional ingress-profile containers such as `cloudflared`, `tailscale`, and `coredns`; they are not required by the generic core profile.

Key boundaries:

- The normalized request/response domain must not import provider-specific types.
- Provider adapters depend on normalized contracts, not on dashboard or persistence handlers.
- Routing depends on provider capability and health interfaces, not concrete providers.
- Authorization is enforced in application services or handlers before repository operations.
- Usage ingestion is durable before asynchronous aggregation.
- The web application accesses control-plane APIs and never reads the database directly.

## 16. Initial Data Model

The schema is expected to include at least:

- organizations
- users
- memberships
- roles
- permissions
- role_permissions
- sessions
- provider_connections
- provider_credentials or encrypted secret fields
- gateway_models
- route_targets
- routing_policies
- api_keys
- api_key_model_rules
- api_key_provider_rules
- budgets
- budget_reservations
- requests
- request_attempts
- attempt_usage_records
- request_usage_totals or equivalent derived totals
- model_prices
- provider_health_samples or rollups
- audit_events
- outbox_events
- outbox_deliveries
- provider_test_jobs

All tenant-owned tables must include an organization identifier and use composite organization-aware foreign keys, constraints, and indexes appropriate to tenant-scoped access. Only documented global-table and organization-root relationships may use non-composite exceptions; exact relationships are governed by `docs/design/02-persistence-tenancy.md` and migration review.

## 17. API and Schema Change Policy

- Public gateway behavior must be documented with an OpenAPI specification where practical and compatibility tests using official OpenAI clients.
- Control-plane APIs must have an explicit version prefix.
- Database migrations must be forward-only in normal operation.
- Breaking public API changes require a new version or a documented migration path.
- Provider behavior differences must be documented as capability limitations, not hidden through lossy transformations.

## 18. Testing Requirements

- TEST-001: Unit tests must cover routing eligibility/order, budget arithmetic, rate-limit decisions, permission checks, error normalization, and provider translations.
- TEST-002: Integration tests must run against real PostgreSQL and Redis instances in CI.
- TEST-003: Provider adapters must have contract tests using deterministic mock upstream servers.
- TEST-004: At least OpenAI and Anthropic adapters must have opt-in live smoke tests guarded by environment credentials.
- TEST-005: End-to-end tests must cover owner setup, login, provider creation, model mapping, API key creation, inference, usage visibility, revocation, and budget denial.
- TEST-006: OpenAI compatibility tests must exercise supported endpoints with pinned official OpenAI SDKs configured to use NexusRelay's base URL. The Phase 0 baseline pins representative success and pre/post-commit error wire fixtures. Compatibility expands incrementally with the implemented public surface: Phase 6 proves Models and Chat, and Phase 10 adds Responses and Embeddings. For each implemented endpoint, the gateway suite must cover every reachable public error category/status used by the V1 matrix, request capture, representative validation and authentication failures, pre-commit retry metadata, malformed/oversized sanitized upstream failures, and applicable post-commit stream failure behavior; each SDK runner must retain assertions for raw status, headers, and parseable body/event data even when SDK exception classes differ.
- TEST-007: Streaming tests must cover cancellation, malformed upstream events, timeouts, and disconnects.
- TEST-008: Security tests must verify cross-organization denial, privilege denial, secret redaction, API key hashing, and CSRF protection.
- TEST-009: Load tests must validate the documented concurrency and overhead target before V1 release.
- TEST-010: Web UI must pass automated accessibility checks for critical flows.

## 19. Delivery Milestones

### Milestone 0: Foundation

- Monorepo structure, local tooling, CI, Docker Compose, PostgreSQL, Redis.
- Go service skeleton, Next.js shell, migrations, logging, metrics, health endpoints.
- Architecture decision records for service topology, data model, secrets, and usage durability.

### Milestone 1: Identity and Administration

- Initial owner bootstrap, local login, sessions, organizations, permissions, default roles.
- Basic dashboard navigation and organization settings.
- Audit framework.

### Milestone 2: Provider and Model Control Plane

- Encrypted provider connections.
- OpenAI, Anthropic, and reusable OpenAI-compatible adapters.
- Provider tests, model registry, route targets, provider health.
- Provider and model administration UI.

### Milestone 3: Gateway Core

- Gateway API key lifecycle and authentication.
- `/v1/models`, `/v1/chat/completions`, and streaming.
- Preferred target routing, bounded fallback, normalized errors, request attempts.
- Usage capture and basic request inspection.

### Milestone 4: Policy and Observability

- Rate limits, budgets with reservations, pricing, cost estimation.
- Cost, latency, availability routing strategies.
- Analytics dashboard, health views, audit log browser.
- Prometheus metrics and OpenTelemetry support.

### Milestone 5: Provider and Protocol Completion

- Google Gemini, OpenRouter, Ollama, and Groq as their profiles reach `contract_verified`; Xiaomi MiMo and CommandCode Provider API only if they pass the explicit provider release decision gate before scope freeze.
- `/v1/responses` and `/v1/embeddings` supported subsets.
- Capability matrix and provider-specific documentation.
- Generic agent-exporter framework and the verified OpenCode connection/model exporter.

### Milestone 6: Hardening and V1 Release

- Security review, tenant isolation tests, load tests, failure testing, backup/restore documentation.
- Retention jobs, operational runbooks, upgrade documentation, release images.

## 20. V1 Release Acceptance Criteria

V1 is accepted when all of the following are true:

- A fresh Docker Compose deployment can be configured and started from documented steps.
- The configured `PUBLIC_API_BASE_URL` reaches only the documented gateway API surface and is reflected correctly in generated client configuration.
- The configured administrative URL works under the selected exposure mode, and forbidden cross-host routes are denied.
- An initial owner can securely create an organization and sign in.
- The owner can configure each release provider whose profile is `contract_verified`; every provider still listed in the confirmed baseline must either meet that gate or be explicitly removed from V1 scope before release.
- The owner can create gateway models with multiple route targets and routing policies.
- A developer can create a restricted gateway API key and see its plaintext only once.
- A user can select models and generate a schema-valid configuration through the OpenCode V1 exporter, referencing the configured key environment variable and exposing only the connection/provider entry plus selected models.
- An official OpenAI SDK can list models and perform non-streaming and streaming requests through NexusRelay.
- Chat completions and Responses API requests route correctly to at least OpenAI, Anthropic, Gemini, and one custom OpenAI-compatible endpoint.
- Routing excludes disabled, `unavailable`, disallowed, over-budget, and capability-incompatible targets. `degraded` and `unknown` targets remain eligible by default with the documented deterministic penalties unless an explicit policy narrows eligibility.
- Safe failures trigger bounded fallback and record every attempt.
- API key model/provider restrictions, expiration, revocation, rate limits, and budgets are enforced.
- Usage and cost can be filtered by organization, user, API key, provider, and model.
- Dashboard analytics show the required metrics with documented freshness.
- Administrative changes and authentication events appear in immutable, redacted audit logs.
- Prompts, completions, credentials, API keys, cookies, and tool arguments do not appear in default logs, traces, usage records, or audit events.
- Automated tests cover critical flows, authorization boundaries, provider contracts, streaming failure cases, and compatibility.
- Performance and security targets have documented test results with no unresolved release-blocking findings.

## 21. Open Decisions

These remaining decisions require an architecture decision record or provider verification note before the affected milestone:

- External pricing source/import management beyond the manual versioned V1 workflow.
- Any optional unsafe fail-open mode beyond the secure fail-closed V1 Redis policy.
- Additions to the supported field matrix defined in `docs/design/13-api-compatibility-matrix.md`.
- Release disposition for Xiaomi MiMo and CommandCode Provider API: before V1 scope freeze, each must either obtain an authoritative contract and reach `contract_verified`, be explicitly redefined by a requirements/design update as a bounded deployment profile, or be removed/deferred from the V1 provider baseline. A blocked profile cannot be implemented or release-supported by inference.

The approved V1 baseline uses separate gateway, control-plane, and worker processes; PostgreSQL request/attempt facts with transactional outbox deliveries; application-scoped repositories plus PostgreSQL RLS; and the secure Redis deny-marker behavior described in `docs/design/`.

## 22. Requirements Change Process

Changes to committed scope should update this document in the same pull request as implementation. New requirements should include:

- A stable requirement identifier.
- User-visible behavior.
- Security and tenant-isolation impact.
- Data migration impact.
- Observability expectations.
- Acceptance and test criteria.
