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
- The Phase 2 localhost path keeps host source secrets in an ignored mode-`0700`
  directory with mode-`0600` files, validates the exact inventory, and uses a
  networkless allowlisted publisher to copy only each service's required files
  into separate named volumes as mode `0400` with the fixed service UID/GID.
  Existing published volumes are immutable through this flow: changed, missing,
  or extra files fail startup instead of rotating credentials implicitly.

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
nexusrelay-bootstrap
```

The runtime image is minimal, includes CA certificates and timezone data, runs as a fixed non-root UID/GID, and has no shell when operational needs allow it.

The one-shot migration image is separate from the Go runtime image. It pins the Apache-2.0 Atlas Community CLI version and release artifact digest, includes only the repository migration directory and required certificate/runtime files, and follows the same non-root, provenance, scanning, and secret-handling requirements. ADR 0007 defines migration behavior.

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
POSTGRES_DB
DATABASE_HOST
DATABASE_PORT
DATABASE_NAME
POSTGRES_USER
POSTGRES_PASSWORD_FILE
DATABASE_MIGRATION_USER
DATABASE_MIGRATION_PASSWORD_FILE
DATABASE_GATEWAY_USER
DATABASE_GATEWAY_PASSWORD_FILE
DATABASE_CONTROL_PLANE_USER
DATABASE_CONTROL_PLANE_PASSWORD_FILE
DATABASE_WORKER_USER
DATABASE_WORKER_PASSWORD_FILE
DATABASE_SSLMODE
DATABASE_MIN_CONNECTIONS_*
DATABASE_MAX_CONNECTIONS_*
DATABASE_STATEMENT_TIMEOUT
DATABASE_TRANSACTION_TIMEOUT
REDIS_URL_FILE
REDIS_PASSWORD_FILE
MASTER_KEYRING_FILE
API_KEY_PEPPER_RING_FILE
SESSION_SECRET_FILE
CSRF_SECRET_RING_FILE
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

Phase 2 dependency readiness uses typed startup timeout, probe timeout, probe interval, and retry-backoff settings. Gateway, control-plane, and worker first establish authenticated PostgreSQL and Redis connectivity during a bounded startup window, then probe continuously. Dependency loss marks readiness false without changing liveness; recovery restores readiness. Schema compatibility, provider cryptographic fence, Redis Function, and product initialization checks are added cumulatively by their owning phases.

The PostgreSQL principal-name settings are transport/configuration inputs for official images, not operator-configurable role semantics. Validation requires exactly `POSTGRES_USER=nexusrelay_cluster_admin`, `DATABASE_MIGRATION_USER=nexusrelay_migration`, `DATABASE_GATEWAY_USER=nexusrelay_gateway`, `DATABASE_CONTROL_PLANE_USER=nexusrelay_control_plane`, and `DATABASE_WORKER_USER=nexusrelay_worker`. Core validation also requires `POSTGRES_DB == DATABASE_NAME`; a mismatch or renamed principal fails before initialization or process startup.

Forced RLS is not runtime-configurable in production. Test tooling may use a separate privileged database role only for migration/setup assertions; application roles always operate with forced RLS.

### Secret File Contracts

All `*_FILE` values point to regular read-only files. Parsers remove at most one trailing LF or CRLF, reject other leading/trailing whitespace where the format does not permit it, reject empty files, enforce the maximum below, and never log file contents. Direct secret environment values are not supported in V1; development uses mounted local secret files too.

- `POSTGRES_PASSWORD_FILE`: UTF-8 password for `POSTGRES_USER=nexusrelay_cluster_admin` after one trailing newline is removed. Outside trusted PostgreSQL initialization, explicit audited role-graph provisioning, and reviewed recovery, it is not mounted into Atlas or an application container.
- `DATABASE_MIGRATION_PASSWORD_FILE`, `DATABASE_GATEWAY_PASSWORD_FILE`, `DATABASE_CONTROL_PLANE_PASSWORD_FILE`, and `DATABASE_WORKER_PASSWORD_FILE`: UTF-8 passwords after one trailing newline is removed. Every setting names a distinct file and secret value. Trusted empty-volume PostgreSQL initialization is the sole process that receives all five database password files so it can create the fixed logins. Atlas and each runtime process receive only their own file. Secret-generation/init tooling validates distinct paths and values without logging, serializing, or comparing them in observable output.
- `REDIS_URL_FILE`: one absolute `redis://` or `rediss://` URI with credentials percent-encoded as required; maximum 4 KiB.
- `REDIS_PASSWORD_FILE`: dedicated Redis server bootstrap password containing at least 32 base64url characters. Its value must equal the percent-decoded password in `REDIS_URL_FILE`. Redis startup reads it from a protected file into a generated in-memory ACL/configuration; the value is not passed as a process argument or logged.
- `MASTER_KEYRING_FILE`: versioned JSON key ring with `expected_epoch` defined in `04-providers-secrets.md`; maximum 64 KiB; file permissions must prevent group/world read.
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
- `CSRF_SECRET_RING_FILE`: versioned JSON HMAC key ring using the same strict JSON, key-ID, canonical-base64, duplicate-field, and 64 KiB limits as the API-key pepper ring. It has one active 32-byte key and zero or more verify-only keys. The derived token format, bounded overlap, retirement behavior, and absence of per-session CSRF digest storage are defined in `03-identity-access.md`.
- `ACME_DNS_API_TOKEN_FILE`: opaque provider token interpreted only by the configured ACME DNS provider integration; maximum 8 KiB.
- `CLOUDFLARE_TUNNEL_CREDENTIALS_FILE`: Cloudflare locally managed tunnel credential JSON matching `CLOUDFLARE_TUNNEL_ID`; maximum 64 KiB.
- `TAILSCALE_AUTH_KEY_FILE`: one opaque Tailscale auth key after one trailing newline is removed; maximum 4 KiB.
- `BOOTSTRAP_PASSWORD_FILE`: UTF-8 bootstrap password after one trailing newline is removed; consumed only by the explicit bootstrap command and subject to password policy.

Conditional startup requirements:

- Gateway, control-plane, and worker require PostgreSQL, Redis, their own database password file, and the other secret files they consume. Migrate requires only shared non-secret PostgreSQL connection settings plus `DATABASE_MIGRATION_USER` and `DATABASE_MIGRATION_PASSWORD_FILE`; it does not receive cluster-admin, runtime, provider, session, CSRF, or API-key cryptographic secrets.
- Control-plane requires session and CSRF secrets plus the provider master and API-key pepper rings for synchronous provider/key mutations. Gateway requires the API-key pepper ring and provider master ring. Worker requires only rings used by its enabled jobs/reconciliation duties. ADR 0004 defines the trust boundary.
- `TLS_MODE=files` requires readable certificate/key files. `TLS_MODE=acme` requires email, supported DNS provider, and DNS API token file.
- `ENABLE_CLOUDFLARE_TUNNEL=true` requires HTTPS public URL/host consistency, tunnel ID, credential file, and accepted profile configuration.
- `ENABLE_TAILSCALE_PRIVATE_ADMIN=true` is rejected until ADR 0002 is Accepted; afterward it requires private admin exposure, auth key, state directory, non-overlapping addresses, and approved routes.
- Bootstrap identity fields and password file are required only by the functional Phase 4 bootstrap command and are ignored/rejected by long-running services. The Phase 1 help/version CLI scaffold performs no identity mutation and must not read them. The Phase 4 CLI hashes plaintext with validated application Argon2id parameters before database invocation; only the encoded hash enters the bounded transaction/function. Functional bootstrap uses the control-plane database login and receives neither cluster-admin nor migration credentials.

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
- The credential-free feasibility harness and authenticated acceptance procedure are maintained in `deploy/private-admin/test-feasibility.sh` and `docs/runbooks/tailscale-private-admin.md`. Passing the local harness alone does not accept ADR 0002 or permit profile enablement.

## PostgreSQL Operations

- Use a pinned supported PostgreSQL major version.
- Trusted Phase 2 empty-volume init assets use `nexusrelay_cluster_admin` to create the application database, all five fixed `LOGIN` principals, and the closed foundational role graph: `nexusrelay_schema_owner`, `nexusrelay_security_definer_owner`, `nexusrelay_gateway_runtime`, `nexusrelay_control_plane_runtime`, and `nexusrelay_worker_runtime`. They revoke unsafe `PUBLIC` schema creation and establish enough bounded migration connection/temp/schema bootstrap authority for the first Atlas run.
- `nexusrelay_migration` is non-superuser, `NOBYPASSRLS`, `NOCREATEROLE`, and role-level `NOINHERIT`. Its memberships in `nexusrelay_schema_owner` and `nexusrelay_security_definer_owner` each have `ADMIN FALSE, INHERIT FALSE, SET TRUE`, so it must explicitly `SET ROLE`. Each runtime LOGIN has role-level `INHERIT`, and its sole runtime-role membership has `ADMIN FALSE, INHERIT TRUE, SET FALSE`, so runtime privileges are inherited while `SET ROLE` and membership administration are denied.
- All five foundational `NOLOGIN` roles have role-level `NOINHERIT` as a defensive default and are non-superuser, `NOBYPASSRLS`, and `NOCREATEROLE`. In PostgreSQL 18 this role attribute controls the default for memberships where that role is the member; it does not override the explicit per-membership `inherit_option` above. Runtime roles own no SQL objects because inherited privileges without `SET ROLE` must not provide a path to owner identity; only the two owner roles own application objects.
- `nexusrelay_cluster_admin` is the official-image bootstrap superuser and has `rolcanlogin`, `rolsuper`, `rolbypassrls`, `rolcreaterole`, and `rolinherit` true. Migration and runtime logins have only `rolcanlogin` true plus the documented migration `rolinherit=false` or runtime `rolinherit=true`; all five foundational roles have all five queried attributes false. Its superuser power is why cluster-admin mounting and use are restricted to the explicitly documented lifecycle operations.
- Phase 3 migrations explicitly `SET ROLE` to create objects under the correct owner and grant privileges to the pre-existing runtime roles. No standing Atlas role-creation authority exists. A graph change is an exceptional deployment upgrade through an explicit audited cluster-admin provisioning command/runbook before Atlas, never hidden in a normal migration.
- Application pools have configured per-process maximum/minimum sizes and statement/transaction timeouts. Transaction timeout is enforced through PostgreSQL transaction-local settings; statement timeout is the upper bound for ordinary statements, with explicitly reviewed worker operations allowed a narrower dedicated override.
- Long-running worker queries use separate pool limits from gateway.
- Migrations run once through the pinned Atlas Community CLI container after directory validation and under Atlas's PostgreSQL advisory lock. `atlas.sum`, revision-history integrity, lock contention, and failure recovery are release-tested per ADR 0007. Atlas Cloud and Atlas Pro migration linting are not required.
- Automated backups include regular full backups plus WAL/point-in-time strategy where operator infrastructure supports it.
- The minimum reference full backup is a portable logical PostgreSQL artifact containing a custom-format database dump and password-free global role definitions. Backup/recovery may mount cluster-admin into a short-lived PostgreSQL client only; Atlas and long-running services never receive it. Exact role verification runs before Atlas in normal Compose startup and after restore.
- Restore and exceptional role-graph upgrade runbooks verify ownership, every exact `pg_auth_members` edge option, and the documented `pg_roles` attributes before Atlas or runtime startup. The Phase 2 harness proves infrastructure recovery; application startup, RLS, encrypted credential readability, outbox resumption, and aggregate idempotency are added cumulatively by the phases that implement those capabilities.
- PostgreSQL major upgrades remain blocked until the target NexusRelay release publishes a reviewed release-specific plan. Release evidence always covers the NexusRelay release/revision, Atlas version, PostgreSQL exact minor, and role-graph contract.

## Redis Operations

- Use a pinned Redis 7.x release; ADR 0008 selects Redis Functions for the RPM/TPM state machine.
- Use authentication and private networking.
- The Phase 2 single-user Redis bootstrap uses `REDIS_PASSWORD_FILE` for the server and `REDIS_URL_FILE` for clients, with startup validation requiring matching values. Before Redis-backed product behavior exists, that shared identity is limited to authenticated `PING`; Phase 3 and later introduce process-specific least-privilege ACL identities together with the commands and key prefixes they require.
- Configure memory policy so critical limiter/version keys are not unpredictably evicted; `noeviction` is preferred for correctness-sensitive deployments with capacity monitoring.
- Key TTLs are mandatory for rate/reservation/activity structures where appropriate.
- The worker loads versioned function libraries side by side, verifies committed source digests, and never replaces a mismatched active library. Gateways may call allowlisted functions but cannot load, replace, flush, delete, or execute arbitrary scripting code.
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

Phase 2 Go services implement this boundary with the standard-library `log/slog`. Runtime event names and attributes are allowlisted centrally; unknown attributes, error objects, and forbidden secret/content categories are replaced before JSON or text serialization. A minimal redacting JSON logger is installed before typed configuration loads so startup failures do not fall back to unstructured standard-library logging. Traefik retains JSON application/access logs with all request headers dropped and no request-body logging.

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

Each Phase 2 Go service serves `/metrics` on its existing internal operational listener when `METRICS_ENABLED=true`. Compose does not publish those listeners and Traefik does not route `/metrics`; operators that add a scraper attach it to the internal network or provide a separately protected operational route. The Phase 2 metric surface covers Go/process runtime state, service readiness, bounded PostgreSQL/Redis readiness probes, and operational HTTP outcomes/latency. Later phases add the subsystem metrics listed above as those behaviors are implemented.

## Tracing

- OpenTelemetry support is optional and disabled unless configured.
- Trace propagation uses W3C context.
- Spans cover inbound request, policy stages, database operations, route selection, provider attempt, and finalization.
- Span attributes use IDs and safe metadata under sampling/access controls; no model content or secrets.
- Provider HTTP auto-instrumentation is configured not to capture bodies or authorization headers.
- No mandatory external exporter exists; self-hosted collectors are supported.
- Phase 2 supports traces-only OTLP/HTTP export. NexusRelay bundles no collector. Enabling requires one unauthenticated absolute `http` or `https` collector endpoint with no URL userinfo, query, or fragment; exporter headers are fixed empty and are not sourced from ambient OpenTelemetry header environment variables.
- When tracing is enabled, the sampler is fixed to parent-based always-on. When disabled, no exporter or background trace pipeline is constructed and no telemetry connection is attempted.

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
- Cluster-admin, migration, gateway, control-plane, and worker database credentials have distinct paths and values. PostgreSQL initialization is the sole process that receives all five; Atlas and runtime containers receive only their own. Initialization and integration tests query `pg_auth_members.admin_option`, `inherit_option`, and `set_option` for all five required edges and query `pg_roles` for `rolcanlogin`, `rolsuper`, `rolbypassrls`, `rolcreaterole`, and `rolinherit`. Tests reject reused paths/values, incorrect fixed principal names, `POSTGRES_DB`/`DATABASE_NAME` mismatch, missing or extra edges, wrong edge options, wrong role attributes, cross-runtime inheritance, and cluster-admin use during normal migration/application startup.
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
2. Backing up the provider master ring, API-key pepper ring, CSRF ring, and session secret separately from PostgreSQL with stricter, independent access.
3. Restoring the database and matching key versions.
4. Rotating provider encryption keys online in batches.
5. Rotating API key pepper with verify-old/hash-new behavior; existing key hashes may require retained old peppers because plaintext is unavailable.
6. Rotating session secret/token hashing keys and intentionally revoking sessions if compatibility is not retained.

Losing the provider master key means encrypted provider credentials cannot be recovered and must be re-entered. This is documented prominently.

The core Compose profile must provide one reproducible minimum backup workflow: consistent portable logical PostgreSQL backup, separately protected cryptographic backup, documented retention, and an automated restore exercise. Database and cryptographic artifacts are independently checksum-protected and stored under separate protected roots. Database passwords are independent recovery inputs, not part of the role dump. Tooling never automatically deletes retained artifacts. Release documentation records measured recovery point and recovery time for the reference profile; optional operator WAL/PITR infrastructure may improve those values but is not the only documented recovery path.

Exceptional role-graph upgrades use a release-owned request contract and reviewed mutation SQL bound by digest. The short-lived cluster-admin runner records exact before/after graphs and atomically publishes a protected external evidence bundle before Atlas is permitted to run. Phase 2 intentionally includes no real graph mutation SQL.

## Redis Loss Recovery

Redis loss moves critical authorization and finite-limit operations to fail-closed readiness while liveness remains healthy. An elected worker rebuilds the critical epoch under the lock protocol in `02-persistence-tenancy.md`, verifies required Redis Function source digests, and initializes the ADR 0008 limiter epoch with acceptance delayed until the next complete UTC minute. Metrics expose rebuild phase, active epochs, verified resource counts, function version/digest status, failure reason, limiter recovery boundary, and fail-closed duration. The runbook defines operator observation and retry actions. Readiness returns healthy only when the critical epoch and required functions are verified; provider health and analytics lag do not affect gateway liveness.

## Minimum Alert Catalog

Release configuration includes alerts with threshold, hold duration, severity, and runbook link for PostgreSQL unavailable, Redis critical-state unavailable, outbox oldest age above target, terminal outbox delivery, provider fleet outage, budget reservation/reconciliation errors, master-ring key retirement blocked, backup/restore verification failure, and privacy-sentinel failure.

## CI Pipeline

Stages:

1. Formatting and generated-file consistency.
2. Go unit tests, race-enabled targeted tests, vet, and lint.
3. TypeScript lint, typecheck, unit/component tests, and production build.
4. PostgreSQL/Redis integration tests.
5. Atlas migration validation/integrity plus application from empty and previous schema, lock-contention, rollback, and recovery tests.
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

The exact official OpenAI SDK pins and deterministic public wire fixtures are defined in [`docs/testing/openai-sdk-compatibility.md`](../testing/openai-sdk-compatibility.md). The implemented harness points each SDK at the gateway base URL and exercises models, chat non-streaming/streaming, Responses, embeddings, tools, structured output, errors, and cancellation according to the supported matrix.

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
