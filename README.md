# NexusRelay

NexusRelay is a self-hosted, multi-tenant LLM gateway designed to give applications one stable OpenAI-compatible API while centralizing provider access, model routing, policy enforcement, usage accounting, budgets, health, analytics, and audit history.

> **Project status:** Phase 1 repository foundation is complete. The repository contains compileable Go service entrypoints, a strict Next.js shell, typed configuration, the versioned control-plane OpenAPI skeleton, pinned non-root images, migration tooling, and CI/security gates. The services currently expose operational scaffolding only, and the checked-in Compose fragment is non-runnable Phase 2 groundwork; PostgreSQL, Redis, Traefik, complete secret mounts, and functional product APIs are not implemented yet. Provider adapters and coding-agent exporters are enabled only after their external contracts are verified from authoritative sources.

Phase 1 accomplishments, preserved foundation decisions, verification evidence, and the Phase 2 boundary are recorded in [`docs/phases/phase-1/README.md`](docs/phases/phase-1/README.md).

## Problem Statement

Applications that use multiple LLM providers usually inherit provider-specific operational complexity:

- Every provider has different credentials, endpoints, models, capabilities, streaming behavior, usage fields, limits, and errors.
- Client applications become coupled to upstream model identifiers and provider SDKs.
- Credentials are distributed across applications and environments, increasing rotation and disclosure risk.
- Routing, fallback, rate limits, and budgets are often implemented independently by each application.
- Usage and cost are difficult to attribute consistently across users, API keys, models, providers, and fallback attempts.
- Provider outages and capability differences are discovered at request time instead of through explicit policy and health state.
- Administrative actions, routing decisions, and policy denials lack a centralized audit trail.
- Coding agents and other clients require tool-specific configuration even when they ultimately consume an OpenAI-compatible API.

NexusRelay addresses this by placing a policy-aware gateway between clients and upstream model providers. Clients use a NexusRelay-issued API key and a stable gateway model name. NexusRelay authenticates the key, applies organization policy, selects an eligible route, calls the upstream provider, normalizes the protocol, and records durable accounting and operational facts.

## Planned V1 Capabilities

### OpenAI-Compatible Gateway

- `GET /v1/models`
- `POST /v1/chat/completions`, including incremental SSE streaming
- A documented subset of `POST /v1/responses`
- A documented subset of `POST /v1/embeddings`
- Stable request IDs and normalized OpenAI-style errors
- Compatibility tests using pinned official OpenAI SDK versions

Compatibility is explicit and capability-gated. NexusRelay does not claim support for fields or behaviors it cannot preserve across an eligible route.

### Provider Connections

- Organization-scoped provider connections with encrypted credentials
- OpenAI, Anthropic, Gemini, OpenRouter, Ollama, Groq, and custom OpenAI-compatible targets after verification
- Explicit capability declarations for operations, streaming, tools, structured output, modalities, usage, and limits
- Safe connection testing and model discovery through durable worker jobs
- SSRF-aware custom URL validation, controlled DNS resolution, redirect restrictions, and named private-network policies
- Sanitized provider errors with no credential or raw upstream-body disclosure

Provider research status is tracked in [`docs/providers/`](docs/providers/). OpenAI, Anthropic, Ollama, and the bounded custom OpenAI-compatible adapter contract are verified for implementation. Gemini, OpenRouter, and Groq remain `profile_drafted` pending exact finish, Responses lifecycle, or final stream-usage contracts. Custom endpoint facts remain operator-asserted, and no provider adapter exists yet.

Xiaomi MiMo and CommandCode Provider API remain blocked. Before V1 scope freeze, each must be verified, explicitly redefined as a bounded profile through requirements/design, or removed/deferred from the V1 baseline; insufficient evidence cannot be filled by inference.

### Model Registry And Routing

- Stable organization-scoped gateway model aliases
- Multiple ordered provider/model targets per gateway model
- Preferred-target fallback
- Lowest estimated cost, lowest observed latency, and highest availability strategies
- Capability, key restriction, budget, rate-limit, provider status, and health eligibility checks
- Deterministic routing for the same policy inputs and health snapshot
- Bounded attempts, deadlines, exponential backoff, and no fallback after streamed output is committed
- Durable routing explanations and attempt history without storing prompt or completion content

### API Keys And Policy Enforcement

- Cryptographically random NexusRelay API keys displayed once
- Non-reversible keyed hashes and lookup prefixes in PostgreSQL
- Expiration, disablement, revocation, model restrictions, and provider restrictions
- Request-per-minute and token-per-minute limits enforced atomically through Redis
- Immediate fail-closed authority reduction across replicas
- Explicit owner-scoped and organization-wide permissions
- Organization, membership, role, provider, model, session, and key invalidation through shared critical-state versioning

### Usage, Pricing, And Budgets

- Durable request and pre-dispatch attempt records
- Per-attempt token, latency, outcome, provider, model, and cost attribution
- Independent provenance for input, output, total, cached, and reasoning token dimensions
- Provider-reported usage when available and clearly labeled estimates otherwise
- Versioned fixed-precision pricing captured at request time
- Organization, membership, and API-key budgets
- Daily and monthly periods with explicit IANA timezones
- Atomic PostgreSQL reservations and reconciliation for hard limits
- Conservative accounting for fallback, cancellation, partial streams, and potentially billable attempts with unavailable usage

### Administration And Governance

- Local email/password authentication behind an identity-provider boundary
- Server-side sessions, secure cookies, CSRF protection, and session revocation
- Multi-organization membership with server-resolved active organization scope
- Permission-based RBAC with Owner, Administrator, Developer, and Viewer defaults
- Custom organization roles and final-owner protection
- Provider, model, routing, API-key, budget, pricing, usage, analytics, health, and audit administration
- Immutable redacted audit events for security-sensitive and administrative actions
- Responsive and accessible Next.js dashboard

### Coding-Agent Configuration Export

NexusRelay uses a generic, versioned exporter framework for external coding agents. Exporters generate only the NexusRelay connection/provider entry and selected gateway models; agent defaults, tools, permissions, workflows, and unrelated settings remain user-managed.

- OpenCode is the first V1 exporter contract and is verified only for the protected run-scoped invocation documented in [`docs/agents/opencode.md`](docs/agents/opencode.md).
- Kilo and CommandCode remain blocked; other exporters can be added without changing the gateway contract after their profiles are verified.
- Every exporter requires authoritative schema verification, pinned fixtures or schemas, deterministic validation, and documented merge behavior.
- Generated artifacts reference an environment variable and never embed a plaintext NexusRelay API key.

## Capability Architecture

```text
                              Administrative path
                     +--------------------------------+
                     |                                v
+----------------+   |   +-----------+       +-----------------+
| Owners/Admins  |------>| Next.js   |------>| Control Plane   |
+----------------+       | Dashboard |       | Go API          |
                         +-----------+       +--------+--------+
                                                     |
                                                     | configuration,
                                                     | identity, audit,
                                                     | outbox, jobs
                                                     v
+----------------+       +-----------+       +-----------------+
| Apps / Agents  |------>| Traefik  |------>| Gateway         |
| OpenAI clients |       | Proxy     |       | Go API          |
+----------------+       +-----------+       +--------+--------+
                                                     |
                                                     | normalized request,
                                                     | routing and adapters
                                                     v
                                            +-------------------+
                                            | LLM Providers     |
                                            | and local models  |
                                            +-------------------+

               +----------------+         +----------------+
               | PostgreSQL     |<------->| Worker         |
               | durable truth  |         | jobs/rollups   |
               +-------+--------+         +-------+--------+
                       ^                          |
                       |                          |
                       +------------+-------------+
                                    |
                              +-----v------+
                              | Redis      |
                              | ephemeral  |
                              | policy and |
                              | counters   |
                              +------------+
```

### Runtime Components

| Component | Responsibility |
| --- | --- |
| `gateway` | Public OpenAI-compatible inference API, authentication, policy admission, routing, provider adapters, streaming, and request finalization |
| `control-plane` | Administrative API, identity, RBAC, organization configuration, provider/key secret creation, audit, and dashboard contracts |
| `worker` | Outbox delivery, provider tests and health probes, analytics rollups, reconciliation, retention, and maintenance jobs |
| `web` | Next.js administrative dashboard; never the authorization boundary |
| PostgreSQL | Durable source of truth for identity, configuration, requests, attempts, usage, budgets, audit, health summaries, and outbox records |
| Redis | Disposable distributed rate-limit state, critical deny markers and versions, short-lived caches, and coordination |
| Traefik | Core Docker Compose reverse proxy, host/path isolation, TLS termination, body limits, and streaming-safe forwarding |

The gateway, control plane, and worker are separate Go binaries and containers built from shared private packages. They remain stateless across replicas except for PostgreSQL and reconstructable Redis coordination state.

## Request Lifecycle

1. Traefik routes a `/v1` request to the gateway without buffering a stream.
2. The gateway creates a request ID and authenticates the NexusRelay API key.
3. Basic request validation consumes request-rate capacity before expensive routing work.
4. The protocol layer normalizes the request and derives required capabilities.
5. The router filters and ranks eligible targets from a captured configuration, health, and pricing snapshot.
6. Redis token-rate capacity and PostgreSQL hard-budget capacity are reserved against immutable attempt slots.
7. PostgreSQL records the request and routing snapshot before any upstream call.
8. Every provider dispatch receives a durable pre-dispatch attempt marker.
9. The selected adapter translates the normalized request and streams or returns provider output.
10. Attempt usage, request totals, cost, and budget reconciliation are persisted before a successful terminal response event.
11. The worker consumes transactional outbox deliveries for analytics, health, invalidation, warnings, and maintenance.

No PostgreSQL transaction remains open while waiting for a provider or streaming to a client.

## Security Architecture

Security and tenant isolation take precedence over availability and delivery speed.

- Every tenant-owned resource is organization-scoped and protected by application checks plus forced PostgreSQL row-level security.
- Tenant SQL uses transaction-local organization context; missing or invalid context fails closed.
- Provider credentials use AES-256-GCM envelopes with keys mounted outside PostgreSQL.
- Gateway API keys use high-entropy generation and keyed hashing with a separately mounted pepper ring.
- Control plane, gateway, and relevant workers form an explicit cryptographic trust boundary with least-privilege code paths.
- Authority-reducing mutations establish Redis deny markers before committing database state.
- Redis loss fails closed for finite rate limits and security-critical authorization or dispatch checks.
- PostgreSQL loss prevents untracked provider dispatch.
- Provider URLs and headers are validated against SSRF, DNS rebinding, redirect, request-smuggling, and routing-header risks.
- Prompts, completions, tool arguments, credentials, API keys, cookies, authorization headers, and raw provider bodies are excluded from default logs, traces, audit events, and usage records.
- Public inference and administrative hosts can be isolated so the public host exposes only documented `/v1` endpoints.

## Data And Consistency Model

- PostgreSQL is authoritative; no durable business record exists only in Redis.
- Administrative mutations commit resource state, audit history, and outbox events atomically.
- Outbox consumers are at-least-once and idempotent per logical projection or side effect.
- Request attempts are recorded before provider dispatch so potentially billable work is never treated as unstarted.
- Money uses integer nanos of ISO 4217 currency units; binary floating point is prohibited.
- Prices and routing snapshots are versioned so historical accounting remains reproducible.
- Analytics and health summaries are derived and rebuildable from durable source facts.
- Raw model content is not persisted by default.

## Deployment Architecture

The initial deployment target is Docker Compose with:

- Traefik
- Web dashboard
- Gateway
- Control plane
- Worker
- One-shot migration and bootstrap commands
- PostgreSQL
- Redis

The core profile is designed to run locally or privately without Cloudflare, Tailscale, provider credentials, or hosted telemetry. Optional profiles may add Cloudflare Tunnel for public API ingress and, after platform verification, Tailscale/CoreDNS for private administration.

## Implementation Readiness

The documented **Phase 0 research and decision gates** and **Phase 1 repository foundation** are complete. Phase 2 is the next implementation stage and must turn the existing non-runnable Compose groundwork into the generic localhost core with PostgreSQL, Redis, Traefik, migrations, protected secrets, and real readiness initialization. Exhaustive gateway compatibility remains incremental: Models and Chat are gated in Phase 6, and Responses and Embeddings extend the same harness in Phase 10. The following artifacts and contracts continue to gate later provider and public endpoint work:

- Authoritative provider profiles and specified deterministic contract fixtures
- A verified OpenCode exporter profile and pinned golden artifacts
- Pinned official OpenAI SDK versions and representative protocol golden fixtures in [`docs/testing/openai-sdk-compatibility.md`](docs/testing/openai-sdk-compatibility.md)
- Control-plane OpenAPI tooling selected by ADR 0006; the versioned Phase 1 source-contract skeleton is checked in and expands with each administrative subsystem
- Migration implementation using the Atlas Community tooling selected by ADR 0007
- Redis limiter implementation using the fixed-window Redis Function design selected by ADR 0008

These gates prevent provider behavior, public compatibility, persisted-data decisions, and security failure policies from being invented during implementation.

## Design Principles

- Tenant isolation and security before availability.
- Correct policy enforcement and accounting before throughput.
- Stable public contracts with explicit capability limits.
- Provider-specific behavior contained within adapters.
- Durable facts before asynchronous aggregation.
- Fail closed when security-critical state cannot be verified.
- Small, versioned, observable state transitions.
- Self-hosted operation without mandatory external telemetry or ingress services.
