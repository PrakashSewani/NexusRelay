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
| `migrate` | Pinned Atlas Community CLI | One-shot validated forward migration execution | None |
| `postgres` | PostgreSQL | Durable system of record | None |
| `redis` | Redis | Distributed counters, versions, coordination, short-lived cache | None |

The Go application image contains distinct gateway, control-plane, and worker binaries built from the same source revision. The migration container uses the separately pinned Atlas Community CLI and repository-owned SQL migration directory from that revision, as defined by ADR 0007.

PostgreSQL credentials follow process boundaries. The five V1 `LOGIN` principal names are fixed product identifiers, not operator-selected database semantics: `nexusrelay_cluster_admin`, `nexusrelay_migration`, `nexusrelay_gateway`, `nexusrelay_control_plane`, and `nexusrelay_worker`. Environment variables carry these names because official images and processes consume them, but startup validation requires the exact values and requires `POSTGRES_DB == DATABASE_NAME`. The cluster-admin login is used only by trusted empty-volume initialization, explicit audited role-graph upgrades, and reviewed recovery. Atlas and each runtime process otherwise receive only their own password file.

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

1. PostgreSQL starts. On an empty data volume only, trusted Phase 2 initialization uses `nexusrelay_cluster_admin` to create the application database, all five fixed `LOGIN` principals, and the closed foundational `NOLOGIN` role graph: `nexusrelay_schema_owner`, `nexusrelay_security_definer_owner`, `nexusrelay_gateway_runtime`, `nexusrelay_control_plane_runtime`, and `nexusrelay_worker_runtime`. PostgreSQL initialization is the sole container allowed to receive all five database password files; secret-generation/init tooling validates distinct paths and distinct values without logging values. Passwords are not embedded in migrations or committed SQL. Redis starts and both dependencies report readiness.
2. `migrate` connects only as `nexusrelay_migration`, validates the committed Atlas migration directory and checksum, obtains Atlas's PostgreSQL advisory migration lock, and applies pending forward migrations.
3. `gateway`, `control-plane`, and `worker` start with their distinct runtime logins only after migration success.
4. `web` starts independently but displays a service-unavailable state until control-plane readiness succeeds.
5. Traefik obtains/loads certificates and routes only to healthy application containers.
6. Enabled optional profile containers establish their configured routes after origin readiness.

Compose dependency declarations are not considered sufficient readiness guarantees; each process retries dependency connection during a bounded startup window and then fails with a sanitized actionable error.

Initialization establishes exact ownership and PostgreSQL 18 membership outcomes before Atlas runs. Both `nexusrelay_migration -> nexusrelay_schema_owner` and `nexusrelay_migration -> nexusrelay_security_definer_owner` have `ADMIN FALSE, INHERIT FALSE, SET TRUE`. Migration therefore does not automatically inherit owner privileges and reviewed migration SQL must explicitly use `SET ROLE`. The schema-owner role owns application schemas, tables, types, and policies; the security-definer-owner role owns every `SECURITY DEFINER` function and cannot log in. Each runtime edge (`nexusrelay_gateway`, `nexusrelay_control_plane`, or `nexusrelay_worker` to its corresponding runtime role) has `ADMIN FALSE, INHERIT TRUE, SET FALSE`. Runtime processes receive granted object/function privileges through inheritance but cannot `SET ROLE` into the group identity or administer membership.

Initialization also revokes unsafe `PUBLIC` schema creation, grants the migration login the bounded database connection, temporary-object, and schema-bootstrap authority required by Atlas, and creates or transfers the application schema as needed for the first migration. Exact PostgreSQL 18 grant syntax is implementation-tested, but the resulting ownership and per-membership `admin_option`, `inherit_option`, and `set_option` values are mandatory. The role-level `INHERIT` attribute is only the default used when a role is later made a member; it does not override an explicit membership option. Migration and all foundational `NOLOGIN` roles use role-level `NOINHERIT` as a defensive default, while runtime LOGIN roles use `INHERIT`; effective access still follows the explicit edges above. No future arbitrary role creation is required for V1. A role-graph change is an exceptional deployment upgrade performed by an explicit audited cluster-admin provisioning command/runbook before Atlas, never by a normal migration.

The fixed V1 schema layout uses private schema `nexusrelay` for application objects and private schema `nexusrelay_migration` for Atlas revision history. `nexusrelay_schema_owner` owns `nexusrelay`; `nexusrelay_migration` owns `nexusrelay_migration` and Atlas's revision table directly. `PUBLIC` receives no access to either private schema. Reviewed application migrations explicitly `SET ROLE nexusrelay_schema_owner` before creating application objects.

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
