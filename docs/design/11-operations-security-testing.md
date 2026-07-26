# Operations, Security, and Testing Design

## Scope

This design covers Docker Compose deployment, images, runtime configuration, secrets, dependency readiness, proxy behavior, observability, backup/restore, security controls, CI, test layers, load testing, and release verification.

## Compose Deployment

The core production-oriented Compose project contains:

```text
proxy
web
gateway
control-plane
worker
migrate
postgres
redis
```

Optional Compose profiles may add `cloudflared`, `tailscale`, and `coredns`. V1 inference/provider operation remains self-hosted and the core profile has no hosted-ingress dependency. The `prakashsewani.com` reference deployment enables the accepted Cloudflare profile from ADR 0001 and intends to enable the private profile only if ADR 0002 passes platform verification and becomes Accepted.

### Volumes

- PostgreSQL data uses a named persistent volume.
- Redis persistence is optional because its contents are reconstructable; if enabled, it improves operational continuity but does not change correctness assumptions.
- Traefik ACME state uses a protected named volume when automated certificate management is enabled. Certificate resolver/provider selection is configuration-driven.
- Application containers have no durable local state.
- Secret files are mounted read-only.

### Networks

- `public-ingress`: optional hosted tunnel and Traefik only.
- `private-ingress`: optional private-network/DNS containers and Traefik only.
- `edge`: Traefik, web, gateway, and control-plane as required.
- `backend`: private network for application processes, PostgreSQL, and Redis.
- PostgreSQL/Redis are not attached to the externally reachable edge network when Compose topology permits.
- Outbound provider traffic originates only from gateway/worker. Control-plane holds cryptographic rings for synchronous credential/key mutations but submits encrypted provider-test jobs to PostgreSQL; it does not call providers or an ad hoc internal execution API.

## Image Design

### Go Image

A multi-stage build produces static or minimally dynamic binaries:

```text
nexusrelay-gateway
nexusrelay-control-plane
nexusrelay-worker
nexusrelay-migrate
nexusrelay-bootstrap
```

The runtime image is minimal, includes CA certificates and timezone data, runs as a fixed non-root UID/GID, and has no shell when operational needs allow it.

### Web Image

Next.js uses standalone output in a pinned Node runtime image, drops root privileges, and contains only production dependencies/assets.

### Pinning and Provenance

- Base images are pinned to immutable digests for releases.
- Go modules and pnpm lockfiles are committed.
- CI generates SBOMs and vulnerability scan reports.
- Release images carry source revision and semantic version labels.

## Configuration Model

Each Go process loads a typed configuration once at startup. Configuration sources, in precedence order:

1. Safe command-line flags for process mode/path selection.
2. Environment variables for non-secret deployment settings.
3. Mounted files for secret values.

Business configuration such as providers, models, routing, budgets, and roles lives in PostgreSQL, not environment variables.

The root `.env.example` is the complete V1 deployment-setting inventory. Its header defines default, optional, conditional, secret-file, and command-only semantics. The following list is illustrative rather than exhaustive; implementations must define typed fields for every inventory entry and must not accept undocumented settings silently:

```text
DATABASE_HOST
DATABASE_PORT
DATABASE_NAME
DATABASE_USER
DATABASE_PASSWORD_FILE
DATABASE_SSLMODE
DATABASE_MIN_CONNECTIONS_*
DATABASE_MAX_CONNECTIONS_*
DATABASE_STATEMENT_TIMEOUT
DATABASE_TRANSACTION_TIMEOUT
REDIS_URL_FILE
MASTER_KEYRING_FILE
API_KEY_PEPPER_RING_FILE
SESSION_SECRET_FILE
CSRF_SECRET_FILE
PUBLIC_API_BASE_URL
ADMIN_BASE_URL
PUBLIC_API_HOST
ADMIN_HOST
ADMIN_EXPOSURE_MODE
HTTP_BIND_ADDRESS
HTTP_PORT
HTTPS_PORT
ADMIN_ORIGINS
LOG_LEVEL
LOG_FORMAT
OTEL_EXPORTER_OTLP_ENDPOINT (optional)
REQUEST_BODY_MAX_BYTES
UPSTREAM_CONNECT_TIMEOUT
UPSTREAM_FIRST_BYTE_TIMEOUT
UPSTREAM_IDLE_TIMEOUT
UPSTREAM_TOTAL_TIMEOUT
SHUTDOWN_GRACE_PERIOD
WORKER_CONCURRENCY
OUTBOX_LEASE_DURATION
OUTBOX_RETRY_BASE_DELAY
OUTBOX_RETRY_MAX_DELAY
HEALTH_PROBE_INTERVAL
HEALTH_FAST_WINDOW
HEALTH_STABILIZING_WINDOW
HEALTH_MIN_SAMPLES
HEALTH_POLICY_VERSION
RETENTION_* settings
RETENTION_BATCH_SIZE
TLS_MODE
TLS_CERT_FILE
TLS_KEY_FILE
ACME_EMAIL
ACME_DNS_PROVIDER
ACME_DNS_API_TOKEN_FILE
AGENT_API_KEY_ENV
OPENCODE_PROVIDER_ID
OPENCODE_PROVIDER_NAME
ENABLE_CLOUDFLARE_TUNNEL
CLOUDFLARE_TUNNEL_ID
CLOUDFLARE_TUNNEL_CREDENTIALS_FILE
ENABLE_TAILSCALE_PRIVATE_ADMIN
TAILSCALE_AUTH_KEY_FILE
TAILSCALE_STATE_DIR
PRIVATE_DNS_IP
```

Startup validation checks URL schemes, timeout relationships, secret lengths/formats, origin policy, schema compatibility, and required files. Errors name the setting but never print secret values.

Forced RLS is not runtime-configurable in production. Test tooling may use a separate privileged database role only for migration/setup assertions; application roles always operate with forced RLS.

### Secret File Contracts

All `*_FILE` values point to regular read-only files. Parsers remove at most one trailing LF or CRLF, reject other leading/trailing whitespace where the format does not permit it, reject empty files, enforce the maximum below, and never log file contents. Direct secret environment values are not supported in V1; development uses mounted local secret files too.

- `POSTGRES_PASSWORD_FILE` and `DATABASE_PASSWORD_FILE`: UTF-8 password bytes after one trailing newline is removed. The core Compose profile points both settings to the same Docker secret so PostgreSQL initialization and application authentication cannot diverge.
- `REDIS_URL_FILE`: one absolute `redis://` or `rediss://` URI with credentials percent-encoded as required; maximum 4 KiB.
- `MASTER_KEYRING_FILE`: versioned JSON key ring defined in `04-providers-secrets.md`; maximum 64 KiB; file permissions must prevent group/world read.
- `API_KEY_PEPPER_RING_FILE`: UTF-8 JSON with the exact V1 shape below; maximum 64 KiB. `version` must be `1`; `active_key_id` must name exactly one entry; entries not active are verify-only. `key_id` matches `[A-Za-z0-9][A-Za-z0-9._-]{0,63}` and each `key` is canonical padded base64 for exactly 32 decoded bytes. Invalid UTF-8, unknown/duplicate fields, trailing JSON values, and duplicate key IDs are rejected. Existing hashes retain their `pepper_key_id`; old peppers are removed only after no hashes reference them or all affected keys are intentionally revoked.

```json
{
  "version": 1,
  "active_key_id": "pepper-2026-01",
  "keys": [
    {"key_id": "pepper-2026-01", "key": "<base64-32-bytes>"}
  ]
}
```
- `SESSION_SECRET_FILE`: base64-encoded 32-byte random key used only for keyed session-token hashing/binding. Rotation either retains an explicit verify-only key ring in a future documented format or revokes all sessions; V1 single-key replacement revokes all sessions.
- `CSRF_SECRET_FILE`: base64-encoded 32-byte random key used only for CSRF token binding. Rotation invalidates outstanding CSRF tokens but not authenticated sessions.
- `ACME_DNS_API_TOKEN_FILE`: opaque provider token interpreted only by the configured ACME DNS provider integration; maximum 8 KiB.
- `CLOUDFLARE_TUNNEL_CREDENTIALS_FILE`: Cloudflare locally managed tunnel credential JSON matching `CLOUDFLARE_TUNNEL_ID`; maximum 64 KiB.
- `TAILSCALE_AUTH_KEY_FILE`: one opaque Tailscale auth key after one trailing newline is removed; maximum 4 KiB.
- `BOOTSTRAP_PASSWORD_FILE`: UTF-8 bootstrap password after one trailing newline is removed; consumed only by the explicit bootstrap command and subject to password policy.

Conditional startup requirements:

- Gateway, control-plane, and worker require PostgreSQL, Redis, and the secret files they consume; migrate requires PostgreSQL but not provider/session/API-key cryptographic files.
- Control-plane requires session and CSRF secrets plus the provider master and API-key pepper rings for synchronous provider/key mutations. Gateway requires the API-key pepper ring and provider master ring. Worker requires only rings used by its enabled jobs/reconciliation duties. ADR 0004 defines the trust boundary.
- `TLS_MODE=files` requires readable certificate/key files. `TLS_MODE=acme` requires email, supported DNS provider, and DNS API token file.
- `ENABLE_CLOUDFLARE_TUNNEL=true` requires HTTPS public URL/host consistency, tunnel ID, credential file, and accepted profile configuration.
- `ENABLE_TAILSCALE_PRIVATE_ADMIN=true` is rejected until ADR 0002 is Accepted; afterward it requires private admin exposure, auth key, state directory, non-overlapping addresses, and approved routes.
- Bootstrap identity fields and password file are required only by the explicit bootstrap command and are ignored/rejected by long-running services.

## Traefik and TLS

Traefik is the V1 reverse proxy. Required behavior:

- TLS 1.2+ with secure defaults; TLS 1.3 preferred.
- Route the configured public API host `/v1/*` to gateway and return 404 for every other path when the API host is separate.
- Route the configured admin host `/api/control/v1/*` to control-plane and dashboard routes/assets to web.
- Disable response buffering and increase suitable timeouts for SSE routes.
- Preserve client cancellation and request IDs.
- Set trusted forwarding headers and strip untrusted client-supplied equivalents.
- Apply request header/body limits consistent with gateway/control-plane settings.
- Do not log authorization/cookie headers or request bodies.
- Support `TLS_MODE=disabled`, mounted certificate files, or ACME. Production public/admin URLs require HTTPS; disabled TLS is limited to local development or trusted external termination.
- For ACME DNS-01, use the configured DNS provider and a least-privilege token appropriate to the selected zone; never require a global account key.
- Store ACME account/certificate state in a protected volume with restrictive permissions.
- Keep HTTP-to-HTTPS redirect behavior internal/private; no router port forwarding is required.

Trusted proxy count/CIDRs are explicit so source IP cannot be spoofed through forwarding headers.

## Optional Cloudflare Tunnel Profile

- `cloudflared` runs as a pinned, non-root container where supported and makes only outbound connections.
- The profile is enabled only when `ENABLE_CLOUDFLARE_TUNNEL=true` and required secret/config validation passes.
- V1 uses a locally managed named tunnel. `CLOUDFLARE_TUNNEL_ID`, credential JSON, and a generated local ingress configuration are required; token-run remotely managed tunnels are not mixed with this profile.
- Tunnel ingress contains exactly the hostname derived from `PUBLIC_API_BASE_URL`, targets the Traefik HTTPS origin, and ends with a mandatory `http_status:404` catch-all.
- Origin Host/SNI matches the configured public API host and origin certificate validation remains enabled.
- The configured admin hostname must not appear in public tunnel ingress when `ADMIN_EXPOSURE_MODE=private`.
- Cloudflare caching is bypassed for `/v1/*`; SSE buffering/transformation is disabled.
- Cloudflare security controls may rate-limit obvious abuse, but NexusRelay API-key rate limits remain authoritative.
- Tunnel configuration is validated with `cloudflared tunnel ingress validate` and rule tests.
- DNS routing of the configured public hostname to the tunnel is an explicit provisioning/runbook step and is verified before readiness is declared.
- The accepted privacy model acknowledges Cloudflare TLS termination; operators must not describe it as cryptographically opaque to Cloudflare.

## Optional Tailscale and Split-DNS Profile

- This section specifies the candidate profile from proposed ADR 0002. Do not ship or describe it as supported until its platform tests pass and the ADR becomes Accepted.
- The profile is enabled only when `ENABLE_TAILSCALE_PRIVATE_ADMIN=true` and `ADMIN_EXPOSURE_MODE=private`.
- The Tailscale container uses persistent state and a tagged, reusable auth key supplied through a mounted secret file.
- On Docker Desktop/macOS, it uses kernel networking (`/dev/net/tun`, `NET_ADMIN`, required sysctls/IP forwarding) rather than userspace-only mode because it must advertise routes to other containers.
- Compose assigns fixed, non-overlapping private subnets/addresses for Traefik and CoreDNS. Startup validates that the selected subnet does not overlap the tailnet client's local/home networks.
- It advertises only the Compose/private ingress and CoreDNS addresses needed for the dashboard, not the entire home LAN by default.
- Advertised routes must be approved in the Tailscale admin console before remote clients can use them.
- Tailnet grants allow approved administrators to reach private HTTPS and DNS and deny other tailnet identities.
- CoreDNS returns the private Traefik ingress address for the configured admin hostname.
- A private hostname may be publicly discoverable through Certificate Transparency when it uses a publicly trusted certificate. Its DNS route/address and service remain private.
- Tailscale DNS config uses CoreDNS as a restricted nameserver for the configured admin hostname/private suffix. Clients must accept Tailscale DNS settings.
- DNS and HTTPS behavior are tested from an authorized tailnet client and from a non-tailnet public client.
- If Docker Desktop cannot provide kernel TUN/subnet-routing support on the host version, deployment is blocked pending an approved alternative such as running Tailscale on the macOS host; the implementation must not silently publish the dashboard through Cloudflare instead.

## PostgreSQL Operations

- Use a pinned supported PostgreSQL major version.
- Application pools have configured per-process maximum/minimum sizes and statement/transaction timeouts. Transaction timeout is enforced through PostgreSQL transaction-local settings; statement timeout is the upper bound for ordinary statements, with explicitly reviewed worker operations allowed a narrower dedicated override.
- Long-running worker queries use separate pool limits from gateway.
- Migrations run once through the migrate container and advisory lock.
- Automated backups include regular full backups plus WAL/point-in-time strategy where operator infrastructure supports it.
- Restore runbook verifies application startup, RLS, encryption keys, and outbox resumption.

## Redis Operations

- Use authentication and private networking.
- Configure memory policy so critical limiter/version keys are not unpredictably evicted; `noeviction` is preferred for correctness-sensitive deployments with capacity monitoring.
- Key TTLs are mandatory for rate/reservation/activity structures where appropriate.
- Lua scripts/functions are loaded/versioned deterministically.
- Redis readiness does not imply policy operations are healthy; gateway exposes internal metrics for script failures and latency.

## Logging

Production logs are structured JSON with a centralized redaction layer. Allowed common fields:

```text
timestamp
severity
service
version
message/event
request_id
trace_id
organization_id when known
operation/route
status
duration_ms
error_category/code
```

Forbidden fields include authorization, cookies, passwords, provider credentials, gateway keys, secret configuration, prompt/completion content, image/audio content, tool arguments/results, and raw upstream bodies.

Redaction occurs before serialization. Logging full Go structs representing HTTP requests/provider payloads is prohibited. Tests submit sentinel secrets/content and scan captured logs.

## Metrics

Prometheus endpoints are available only on the internal network or protected operational route. Core metrics include:

- Gateway request rate, outcomes, latency, TTFT, active streams, bytes, attempts, and fallback counts.
- Policy denials by low-cardinality reason.
- Provider outcomes and latency by provider type/operation, not unbounded connection/model labels unless carefully bounded.
- PostgreSQL/Redis pool and operation health.
- Worker job duration, failures, lag, and pending work.
- Budget reservation/reconciliation and overshoot.
- Cache hit/miss/invalidation/version mismatch.

No user, key, request, organization, or arbitrary model IDs are labels.

## Tracing

- OpenTelemetry support is optional and disabled unless configured.
- Trace propagation uses W3C context.
- Spans cover inbound request, policy stages, database operations, route selection, provider attempt, and finalization.
- Span attributes use IDs and safe metadata under sampling/access controls; no model content or secrets.
- Provider HTTP auto-instrumentation is configured not to capture bodies or authorization headers.
- No mandatory external exporter exists; self-hosted collectors are supported.

## Security Controls

### Supply Chain

- Dependency vulnerability scanning for Go, Node, and images.
- Secret scanning in CI.
- Lockfile integrity and review of new direct dependencies.
- SBOM generation for release artifacts.

### Runtime

- Non-root, minimal capabilities, read-only filesystem where practical.
- No Docker socket mount.
- Explicit writable temp paths and resource limits.
- Database roles follow least privilege and RLS separation.
- Master key/pepper/session secrets are distinct and independently rotatable.
- Debug endpoints/profilers are disabled or internal/authenticated in production.

### Application

- Parameterized SQL, strict validation, body limits, output encoding.
- CSRF and secure cookies for browser paths.
- SSRF-safe provider dialing.
- CORS deny-by-default.
- Generic authentication failures and sanitized provider errors.
- No content persistence or logging by default.

## Backup and Secret Recovery

Runbooks must cover:

1. Backing up PostgreSQL and verifying restore.
2. Backing up master key ring and API-key pepper ring separately with stricter access.
3. Restoring the database and matching key versions.
4. Rotating provider encryption keys online in batches.
5. Rotating API key pepper with verify-old/hash-new behavior; existing key hashes may require retained old peppers because plaintext is unavailable.
6. Rotating session secret/token hashing keys and intentionally revoking sessions if compatibility is not retained.

Losing the provider master key means encrypted provider credentials cannot be recovered and must be re-entered. This is documented prominently.

The core Compose profile must provide one reproducible minimum backup workflow: consistent PostgreSQL backup, separately protected cryptographic-ring backup, documented retention, and an automated restore exercise. Release documentation records measured recovery point and recovery time for the reference profile; optional operator WAL/PITR infrastructure may improve those values but is not the only documented recovery path.

## Redis Loss Recovery

Redis loss moves critical authorization and finite-limit operations to fail-closed readiness while liveness remains healthy. An elected worker rebuilds the critical epoch under the lock protocol in `02-persistence-tenancy.md`; metrics expose rebuild phase, active epoch, verified resource counts, failure reason, limiter recovery boundary, and fail-closed duration. The runbook defines operator observation and retry actions. Readiness returns healthy only when the critical epoch is verified and required Redis scripts are loaded; provider health and analytics lag do not affect gateway liveness.

## Minimum Alert Catalog

Release configuration includes alerts with threshold, hold duration, severity, and runbook link for PostgreSQL unavailable, Redis critical-state unavailable, outbox oldest age above target, terminal outbox delivery, provider fleet outage, budget reservation/reconciliation errors, master-ring key retirement blocked, backup/restore verification failure, and privacy-sentinel failure.

## CI Pipeline

Stages:

1. Formatting and generated-file consistency.
2. Go unit tests, race-enabled targeted tests, vet, and lint.
3. TypeScript lint, typecheck, unit/component tests, and production build.
4. PostgreSQL/Redis integration tests.
5. Migration from empty and previous schema.
6. Provider mock contract tests.
7. Agent-exporter schema/golden-fixture tests, with OpenCode required for V1 and other agents gated by verified profiles.
8. OpenAI SDK compatibility tests.
9. End-to-end Compose tests.
10. Security scans, secret scan, and SBOM.
11. Image build and Compose config validation.

Live provider smoke tests are opt-in/manual and never required for untrusted pull requests or default CI.

## Test Architecture

### Unit Tests

Pure domain behavior: permission decisions, routing filters/order, error mapping, token/cost arithmetic, budget periods/reservations, capability derivation, and protocol translation.

### Integration Tests

Real PostgreSQL and Redis validate RLS, constraints, transactions, outbox claims, rate-limit atomicity, migration behavior, and cache invalidation.

### Provider Contracts

Deterministic local HTTP servers test auth headers, request translation, streaming chunks/events, usage extraction, malformed responses, timeouts, cancellation, retries, and redaction.

### Compatibility Tests

Official OpenAI SDKs point at the gateway base URL and exercise models, chat non-streaming/streaming, Responses, embeddings, tools, structured output, errors, and cancellation according to the supported matrix.

### End-to-End Tests

Compose-level tests cover bootstrap, login, organization/membership permission boundaries, provider setup against mocks, model/routing setup, key creation, inference, usage, revocation, rate limiting, budget denial, analytics, and audit.

### Security Tests

- Two-organization denial for every resource class.
- Missing permission denial.
- RLS missing/wrong context.
- CSRF/session/cookie behavior.
- SSRF and redirect/DNS edge cases.
- Sentinel secret/content scanning across logs, traces, API responses, audit, fixtures, and persisted metadata.
- Fuzzing parsers, SSE normalization, key format, and cursor decoders.

### Failure Tests

- PostgreSQL/Redis unavailable at each inference stage.
- Gateway termination during stream/finalization.
- Worker termination during outbox side effect.
- Slow clients, provider stalls, malformed events, partial streams, and cancellation.
- Missed invalidation and cache TTL recovery.

## Performance Tests

Reference hardware and exact workload are versioned before results are accepted. The profile records CPU/memory, operating system/container runtime, gateway replica count, PostgreSQL/Redis sizing, TLS mode, request body and token distributions, stream duration/event rate, warm-up, test duration, and the method used to subtract upstream latency. Tests cover:

- 500 non-streaming requests/second using deterministic local providers.
- 100 concurrent streaming requests with bounded event rates.
- p50/p95/p99 gateway overhead excluding upstream latency.
- API-key auth and Redis limiter latency.
- Budget reservation contention.
- Database pool saturation and outbox lag.
- Memory per active stream and slow-client backpressure.

The p95 gateway overhead target is below 50 ms under the documented baseline. Performance optimizations cannot weaken policy correctness or durability without an approved design change.

## Release Checklist

- Compose starts from a clean host using documented configuration.
- Migrations and bootstrap complete safely.
- All default tests pass without provider secrets.
- Live smoke tests pass for release-target providers where credentials are available.
- Backup/restore and key recovery have been exercised.
- Security scan has no unresolved release-blocking findings.
- Load targets and resource recommendations are documented.
- Logs/traces/persistence pass sentinel privacy scans.
- Operational alerts and runbooks cover dependency outage, worker lag, provider outage, budget reconciliation, and key rotation.
- Agent-exporter support claims match `contract_verified` profiles; OpenCode is validated against its pinned artifacts.

## Requirement Coverage

This design satisfies SEC-001 through SEC-018, NFR-001 through NFR-010, OBS-001 through OBS-007, FR-CONFIG-001 through FR-CONFIG-005, TEST-001 through TEST-010, and operational V1 acceptance criteria. SEC-019 is owned by the provider design and verified in the same security suite.
