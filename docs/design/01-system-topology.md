# System Topology

## Responsibilities

NexusRelay separates latency-sensitive inference, administrative traffic, and asynchronous work into independently managed containers. All components are stateless across replicas except PostgreSQL and Redis.

## Container Model

| Container | Process | Responsibility | Host ports |
| --- | --- | --- | --- |
| `proxy` | Traefik | TLS termination, hostname/path routing, headers, request limits, streaming pass-through | Configurable local/public bind |
| `cloudflared` | Cloudflare Tunnel client | Optional outbound public tunnel for the configured API hostname | None |
| `tailscale` | Tailscale container | Optional tailnet node/subnet route for private proxy and DNS addresses | None |
| `coredns` | CoreDNS | Optional split-DNS answer for the configured private admin hostname | None |
| `web` | Next.js server | Administrative UI and server-side rendering | None |
| `gateway` | Go gateway binary | Public OpenAI-compatible inference API | None |
| `control-plane` | Go control-plane binary | Authentication and versioned administrative APIs | None |
| `worker` | Go worker binary | Outbox, health checks, analytics, reconciliation, retention | None |
| `migrate` | Go migration command | One-shot forward migration execution | None |
| `postgres` | PostgreSQL | Durable system of record | None |
| `redis` | Redis | Distributed counters, versions, coordination, short-lived cache | None |

The Go application image contains distinct gateway, control-plane, worker, and migration binaries built from the same source revision. Containers select the relevant binary through their image command.

## Network Routes

```text
Core local profile:
  client -> Traefik configured bind/host
  |-- public API host + /v1/*       -> gateway
  |-- admin host + /api/control/*   -> control-plane
  `-- admin host + /*               -> web

Optional public tunnel profile:
  public client -> hosted tunnel edge -> tunnel container -> Traefik -> gateway

Optional private admin profile:
  admin client -> private overlay network -> split DNS -> Traefik -> web/control-plane

Internal network:
  gateway       -> postgres, redis, configured providers
  control-plane -> postgres, redis
  worker        -> postgres, redis, configured providers
  web           -> control-plane through same-origin/private route
  migrate       -> postgres
```

PostgreSQL and Redis never publish host ports in the production Compose profile. The core local profile binds Traefik to configurable host ports, defaulting to loopback-safe development values. Optional ingress profiles may reach Traefik through tunnel/private-network containers without router port forwarding.

## Hostname Isolation

- `PUBLIC_API_BASE_URL` defines the externally advertised inference base URL and must end in `/v1`.
- `ADMIN_BASE_URL` defines the browser/control-plane origin.
- `PUBLIC_API_HOST` and `ADMIN_HOST` are derived or configured consistently for Traefik routing.
- When hosts differ, the public router accepts only `/v1/*`; dashboard assets, `/api/control/*`, and unknown paths return 404.
- A single-host local profile may route by path, but production startup warns or rejects unsafe public-admin combinations according to `ADMIN_EXPOSURE_MODE`.
- Optional private DNS is authoritative only for the configured private admin hostname and is never generally exposed.
- The application does not contain a default personal domain, private IP, or hosted-ingress account identifier.

## Process Boundaries

### Gateway

- Owns `/v1/models`, `/v1/chat/completions`, `/v1/responses`, and `/v1/embeddings`.
- Performs API key authentication, policy enforcement, route selection, provider calls, streaming, and request finalization.
- Does not serve dashboard sessions or run scheduled jobs.
- Uses bounded local caches for key, model, provider, capability, and routing configuration.

### Control Plane

- Owns local authentication, sessions, organization selection, and `/api/control/v1` resources.
- Performs permission checks and configuration mutations.
- Writes audit and outbox records in the same transaction as administrative mutations.
- Does not proxy inference requests.

### Worker

- Claims and processes transactional outbox rows.
- Runs provider health probes, analytics rollups, request/reservation reconciliation, retention, and maintenance tasks.
- Uses PostgreSQL advisory locks or lease rows to ensure singleton ownership where required.
- Is horizontally scalable for partitionable jobs; singleton schedules still use distributed ownership.

### Web

- Renders the dashboard and handles browser interaction.
- Contains no database credentials, provider credentials, master encryption key, or gateway signing material.
- Treats control-plane responses as authoritative for permissions and validation.

## Startup Sequence

1. PostgreSQL and Redis start and report readiness.
2. `migrate` obtains a PostgreSQL advisory migration lock and applies pending forward migrations.
3. `gateway`, `control-plane`, and `worker` start only after migration success.
4. `web` starts independently but displays a service-unavailable state until control-plane readiness succeeds.
5. Traefik obtains/loads certificates and routes only to healthy application containers.
6. Enabled optional profile containers establish their configured routes after origin readiness.

Compose dependency declarations are not considered sufficient readiness guarantees; each process retries dependency connection during a bounded startup window and then fails with a sanitized actionable error.

## Health Endpoints

Each Go process exposes internal endpoints:

- `/health/live`: process event loop is responsive; does not query dependencies.
- `/health/ready`: required dependencies and startup initialization are ready.

Gateway readiness requires PostgreSQL and Redis under the selected failure policy, loaded encryption/configuration prerequisites, and no incompatible schema version. Provider availability does not affect gateway readiness.

Worker readiness confirms database access and worker initialization, not successful health of every provider. Web readiness confirms the Next.js process can serve assets.

## Graceful Shutdown

### Gateway

1. Mark readiness false.
2. Stop accepting new connections.
3. Cancel requests that have not begun upstream work if they cannot complete in the drain window.
4. Allow non-streaming and streaming requests to drain up to the configured deadline.
5. Propagate cancellation upstream after the deadline.
6. Attempt final request and reservation reconciliation before exit.

### Control Plane

Stop accepting requests, complete bounded active transactions, close sessions to dependencies, and exit.

### Worker

Stop claiming new jobs, finish or release claimed leases, persist retry state, cancel provider probes, and exit.

## Scaling

- Gateway replicas scale independently based on request rate and active streams.
- Control-plane replicas scale independently based on dashboard traffic.
- Worker replicas scale based on outbox lag and scheduled workload.
- Web replicas are stateless; browser sessions are held by the control plane, not local web memory.
- All replicas use the same application version during normal deployment; expand-and-contract schema changes support rolling updates where needed.

## Failure Isolation

- Worker failure cannot block inference except when an unprocessed critical invalidation requires a conservative database reload.
- Web failure does not affect inference.
- Control-plane failure prevents administration but does not invalidate already loaded, still-valid gateway configuration.
- Redis failure follows explicit subsystem policies; PostgreSQL remains authoritative.
- PostgreSQL failure causes controlled rejection of new inference when request durability or policy correctness cannot be guaranteed.
- Individual provider failures influence health and routing but not service liveness.

## Image and Runtime Constraints

- Images use pinned base versions and multi-stage builds.
- Runtime containers run as non-root with read-only root filesystems where practical.
- Writable paths are explicit temporary volumes.
- Secrets are mounted through Docker secrets or equivalent protected files; environment variables may reference secret file paths.
- Containers emit logs to stdout/stderr and never write local durable application logs.
- Resource limits and stop grace periods are declared in Compose. Stop grace uses typed deployment configuration; CPU/memory limits use reviewed Compose defaults and may be changed through a standard operator Compose override file rather than application environment variables.

## Hosted Tunnel Trust Boundary

When an optional hosted tunnel terminates client TLS at its edge, that provider can technically inspect inference content and gateway API keys. The profile documentation must state this explicitly. Provider credentials and administrative cookies must not traverse a public API-only hostname. Operators requiring cryptographic opacity from intermediaries must use direct TLS or an approved private-network profile.

The `prakashsewani.com` deployment is one reference configuration: its Cloudflare profile is accepted by ADR 0001, while its Tailscale/CoreDNS profile remains proposed in ADR 0002 pending platform verification. These hostnames and providers are not core defaults.

## Requirement Coverage

This design primarily satisfies FR-API-001, FR-CONFIG-001 through FR-CONFIG-005, NFR-001, NFR-002, NFR-006, NFR-009, NFR-010, OBS-006, SEC-001, SEC-009, and the Docker Compose acceptance criteria.
