# NexusRelay Implementation Checklist

This checklist is the implementation handoff for a new session. Read `AGENTS.md`, `docs/requirements.md`, and the applicable `docs/design/` document before starting each item. Check items only after implementation, tests, and documentation are complete.

## Delivery Goal

A new operator must be able to clone the repository, configure the documented environment and local secret files, bootstrap the initial owner, and start a healthy generic NexusRelay core deployment with Docker Compose. The core path must not require Cloudflare, Tailscale, provider credentials, personal domains, or undocumented machine-specific setup.

Completion requires an automated clean-clone installation test in a fresh environment using only tracked files, `.env.example` as the configuration inventory, documented secret/bootstrap commands, and the public runbook. The test must verify migrations, service health, local dashboard access, owner login, and authenticated access to the gateway API. Optional ingress and provider profiles are tested separately and must not block core startup.

## Phase 0: Decisions and Provider Research

- [ ] Create ADR for Go HTTP/router and control-plane API schema tooling if the choice is consequential.
- [ ] Create ADR for migration tooling and confirm it supports advisory locking and forward-only migrations.
- [ ] Create ADR for Redis limiter algorithm and script/version management.
- [ ] Create `docs/providers/openai.md` from authoritative documentation and mark it `contract_verified`.
- [ ] Create `docs/providers/anthropic.md` from authoritative documentation and mark it `contract_verified`.
- [ ] Create `docs/providers/openai-compatible.md` for the reusable custom adapter contract.
- [ ] Research and create profiles for Gemini, OpenRouter, Ollama, and Groq.
- [ ] Locate authoritative Xiaomi MiMo and CommandCode API documentation or keep those providers blocked.
- [ ] Complete `docs/agents/opencode.md`, pin its schema/fixtures, and mark the OpenCode exporter `contract_verified`.
- [ ] Pin exact official OpenAI SDK versions and commit golden chat, Responses, embeddings, stream, and error fixtures before gateway endpoint implementation.
- [ ] Research Kilo and CommandCode agent configuration profiles; keep their exporters blocked until authoritative schemas are verified.

## Phase 1: Repository Foundation

- [ ] Initialize the Go module and root workspace/tooling files.
- [ ] Create `apps/gateway`, `apps/control-plane`, and `apps/worker` entrypoints.
- [ ] Create migration and bootstrap command entrypoints.
- [ ] Scaffold `apps/web` with strict TypeScript, linting, typecheck, and tests.
- [ ] Add shared Makefile/task scripts without hiding required commands.
- [ ] Add formatting, lint, unit-test, build, secret-scan, and dependency-scan CI.
- [ ] Add pinned multi-stage Dockerfiles running as non-root.
- [ ] Add typed configuration loading with `*_FILE` secret support and validation.
- [ ] Keep `.env.example` synchronized with implemented settings.

## Phase 2: Core Docker Compose and Operations

- [ ] Create the generic core Compose profile: Traefik, web, gateway, control-plane, worker, migrate, PostgreSQL, and Redis.
- [ ] Make the core profile work on localhost without Cloudflare, Tailscale, or provider credentials.
- [ ] Document and automate generation of development secret files with correct formats and permissions without committing secret values.
- [ ] Add a documented first-run flow covering `.env` creation, secret setup, migrations, owner bootstrap, startup, health checks, and shutdown.
- [ ] Add health checks, dependency readiness, restart policies, resource limits, and graceful shutdown.
- [ ] Add protected secret mounts and named PostgreSQL/Traefik state volumes.
- [ ] Add optional Cloudflare Tunnel profile driven by `PUBLIC_API_BASE_URL`.
- [ ] Validate Tailscale/CoreDNS profile feasibility on macOS/Docker Desktop or approve and document a host-Tailscale alternative; update ADR 0002 to Accepted before implementation.
- [ ] Add the accepted optional private-admin profile driven by `ADMIN_BASE_URL` and private subnet settings.
- [ ] Add structured redacted logging, Prometheus metrics, and optional OpenTelemetry setup.
- [ ] Add backup, restore, key-ring recovery, and upgrade runbooks.

## Phase 3: PostgreSQL, RLS, and Outbox

- [ ] Implement UUIDv7 generation and common time/version conventions.
- [ ] Implement initial schema with organization-aware keys, indexes, and constraints.
- [ ] Create application, authentication, maintenance, and migration database roles.
- [ ] Enable and force RLS on every tenant table.
- [ ] Implement transaction-local organization/actor context.
- [ ] Generate typed queries with `sqlc` and use `pgx` pools per process.
- [ ] Implement per-consumer transactional outbox events/deliveries.
- [ ] Implement worker lease/claim/retry/idempotency behavior.
- [ ] Implement the shared fail-closed Redis deny-marker/version infrastructure before identity, provider, model, or API-key security mutations depend on it.
- [ ] Test migrations from empty and previous supported schema states.
- [ ] Add cross-organization and missing-RLS-context integration tests.

## Phase 4: Identity, Organizations, and RBAC

- [ ] Implement one-time owner/organization bootstrap.
- [ ] Implement Argon2id password hashing and rehash-on-login behavior.
- [ ] Implement login throttling with generic failure responses and mandatory failed-login security audit events.
- [ ] Implement server-side sessions, secure cookies, CSRF, expiry, and revocation.
- [ ] Implement organizations, memberships, active organization selection, and deployment-operator organization creation command.
- [ ] Seed the documented permission catalog and default role matrix exactly.
- [ ] Implement custom roles and integrate role/membership mutations with the shared deny-marker/version infrastructure.
- [ ] Implement final-owner protection, including global user-disable concurrency behavior.
- [ ] Add permission and two-organization negative tests for every identity resource.

## Phase 5: Provider Connections and Secrets

- [ ] Implement AES-256-GCM provider credential envelopes and key-ring startup validation.
- [ ] Implement bounded online credential re-encryption/key rotation.
- [ ] Implement provider connection CRUD, enable/disable, secret rotation, audit events, and shared deny-marker integration.
- [ ] Implement SSRF-safe URL validation, DNS resolution, dialing, redirect policy, and private-network policies.
- [ ] Implement asynchronous encrypted provider-test jobs owned by the worker.
- [ ] Implement capability declarations and model discovery snapshots.
- [ ] Implement OpenAI, Anthropic, and shared OpenAI-compatible adapters only after profiles are verified.
- [ ] Add deterministic adapter contract tests for cancellation, streaming, usage, errors, and redaction.

## Phase 6: Models, API Keys, and Gateway Protocol

- [ ] Implement gateway models, route targets, routing policies, pricing references, and optimistic concurrency.
- [ ] Implement API-key generation, prefix lookup, keyed hashing, one-time display, ownership, restrictions, expiry, disablement, and revocation.
- [ ] Integrate API keys and models with the shared fail-closed deny-marker/version infrastructure and add multi-replica invalidation tests.
- [ ] Implement `GET /v1/models` filtered by key and enabled model state.
- [ ] Implement normalized inference domain and strict transport validation.
- [ ] Implement `POST /v1/chat/completions` non-streaming and SSE streaming according to the compatibility matrix.
- [ ] Implement stable errors, request IDs, body limits, timeout stages, cancellation, and bounded backpressure.
- [ ] Add official OpenAI SDK compatibility tests and privacy sentinel tests.

## Phase 7: Routing, Usage, Limits, and Budgets

- [ ] Implement deterministic two-phase route planning and admission.
- [ ] Implement eligibility exclusions, ordered routing, and bounded fallback.
- [ ] Persist request start and mandatory pre-dispatch attempt markers.
- [ ] Persist attempt completion and per-attempt usage/cost facts.
- [ ] Implement source-aware provider-reported/estimated token usage.
- [ ] Implement versioned nanos-based pricing and historical effective selection.
- [ ] Implement Redis RPM and TPM reservation/reconciliation scripts.
- [ ] Implement organization, user, and API-key budget periods and PostgreSQL reservations.
- [ ] Persist final usage and reconcile budgets before successful response/stream termination.
- [ ] Implement stale request/reservation reconciliation.
- [ ] Add concurrency, fallback-cost, cancellation, deadline, and Redis/PostgreSQL outage tests.

## Phase 8: Health, Analytics, Audit, and Retention

- [ ] Implement active provider probes and passive attempt-health observations.
- [ ] Implement health summaries with freshness, thresholds, and hysteresis.
- [ ] Implement idempotent hourly analytics rollups and freshness metadata.
- [ ] Implement immutable redacted audit events and database privileges.
- [ ] Implement request, usage, provider, model, user, key, budget, and audit queries with cursor pagination.
- [ ] Implement bounded retention jobs and deletion observability.
- [ ] Add duplicate-delivery, rebuild, stale-data, retention-boundary, and worker-crash tests.

## Phase 9: Control Plane and Web Dashboard

- [ ] Define/version the control-plane API contract and generate clients/types.
- [ ] Implement endpoint permission/resource-policy matrix.
- [ ] Build login, organization selection, overview, provider, model, routing, API-key, user, role, budget, usage, analytics, audit, and settings pages.
- [ ] Implement explicit loading, empty, partial, stale, permission-denied, validation, and service-error states.
- [ ] Implement accessible secret rotation, one-time API-key, destructive confirmation, and stale-version conflict flows.
- [ ] Verify critical flows on desktop and mobile with accessibility tests.

## Phase 10: Agent Export and Remaining V1 Protocols

- [ ] Implement the generic agent-exporter registry and versioned preview/render control-plane contract.
- [ ] Implement the verified OpenCode exporter using the configured public base URL and shared key environment-variable name.
- [ ] Restrict every export to the selected key/model intersection and connection/model-only settings.
- [ ] Validate OpenCode output against pinned schema/fixtures and run an opt-in OpenCode smoke test.
- [ ] Add Kilo, CommandCode, or other exporters only after their agent profiles reach `contract_verified`.
- [ ] Implement the documented Responses API subset.
- [ ] Implement the documented Embeddings API subset.
- [ ] Complete verified Gemini, OpenRouter, Ollama, Groq, Xiaomi MiMo, and CommandCode adapters as profiles become ready.

## Phase 11: Release Hardening

- [ ] Run the full unit, integration, contract, compatibility, E2E, security, migration, and accessibility suites.
- [ ] Meet the documented non-streaming throughput, streaming concurrency, memory, and p95 overhead targets.
- [ ] Verify no secrets/model content appear in logs, traces, errors, fixtures, audit, usage metadata, or generated configs.
- [ ] Test clean Compose installation, optional ingress profiles, backup/restore, key rotation, dependency outage, and graceful shutdown.
- [ ] Run an automated clean-clone acceptance test from a fresh workspace using only tracked files and public setup documentation; verify secret generation, migration, bootstrap, healthy startup, dashboard login, and authenticated gateway access.
- [ ] Verify the generic core starts without Cloudflare, Tailscale, provider credentials, personal domains, or undocumented host dependencies.
- [ ] Produce SBOMs, vulnerability reports, pinned release images, runbooks, and release notes.
- [ ] Complete the V1 acceptance checklist in `docs/requirements.md` and traceability evidence in `docs/design/12-requirement-traceability.md`.
