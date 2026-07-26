# Phase 2 Local Core Runbook

## Scope

This runbook starts the generic Phase 2 NexusRelay core on localhost. It covers configuration creation, development-secret generation, protected publication, PostgreSQL empty-volume initialization, Atlas migrations, service readiness, dashboard verification, and shutdown.

The Phase 2 deployment has no functional product APIs, login, or owner bootstrap. Initial owner and organization bootstrap is implemented in Phase 4. The dashboard therefore reports that administrative features are unavailable even when the infrastructure is healthy.

No Cloudflare, Tailscale, provider credential, personal domain, or hosted telemetry configuration is required.

## Prerequisites

- Docker Engine or Docker Desktop with Docker Compose v2 and BuildKit.
- Go matching `go.mod`, used only by the local setup command to generate and validate secrets without placing their values in process arguments or logs.
- `curl` for the routed dashboard check.
- Local TCP port `8080` available on loopback.

## First Run

Run:

```text
make local-core-up
```

The command performs these steps:

1. Creates ignored `.env` from `.env.example` with mode `0600` if `.env` does not exist. Existing `.env` files are never overwritten.
2. Creates ignored `.local-secrets/` with mode `0700` and exactly eleven cryptographically random development secret files with mode `0600`. A complete existing inventory is validated and reused; incomplete or modified inventories fail closed rather than being overwritten.
3. Validates the full typed deployment inventory and secret formats before dependency startup.
4. Publishes strict per-service secret allowlists to named volumes as mode `0400` with the fixed container UID/GID. The publisher has no network and rejects changed, missing, or extra files.
5. Initializes the PostgreSQL role graph only when the PostgreSQL data volume is empty, verifies the exact PostgreSQL 18 role/edge contract in a short-lived pre-Atlas container, runs Atlas migrations, and starts the Go services, web application, and Traefik.
6. Waits for container health and verifies the dashboard through `http://localhost:8080`.

The local core does not require provider secrets because the generated provider ring is local cryptographic bootstrap material, not an upstream provider credential.

## Health And Status

Run:

```text
make local-core-check
docker compose --env-file .env -f deploy/compose.yaml --profile core ps
```

Gateway, control-plane, and worker readiness continuously probes authenticated PostgreSQL and Redis access. Their liveness remains available during dependency loss while readiness becomes unhealthy. Phase 3 and later add schema compatibility, cryptographic fence, Redis Function, and product initialization checks cumulatively.

Traefik starts only after gateway, control-plane, and web health checks pass. The web readiness endpoint proves that the Next.js process serves responses; it does not claim the control-plane product surface is implemented.

## Logs, Metrics, And Traces

Gateway, control-plane, worker, and Traefik logs are structured JSON by default. Application logging uses centralized allowlisting and redaction; authorization values, cookies, credentials, API keys, prompts, completions, tool data, request/response bodies, and unknown structured values are replaced before serialization. Traefik access logs retain JSON format with request headers dropped.

Each Go service exposes Prometheus-compatible `/metrics` only on its internal listener while `METRICS_ENABLED=true`. The core profile does not publish those ports and Traefik has no `/metrics` route. An operator-provided Prometheus instance must join the Compose backend network or use a separately protected operational route. Metric labels use fixed service, route, status-class, dependency, and outcome vocabularies; request, organization, user, API-key, and arbitrary model identifiers are never labels.

Tracing is disabled by default and the core profile includes no OpenTelemetry collector. To use an operator-managed collector, set `OTEL_ENABLED=true` and set `OTEL_EXPORTER_OTLP_ENDPOINT` to its complete unauthenticated OTLP/HTTP traces URL, such as `http://collector:4318/v1/traces`. The URL may use only HTTP or HTTPS and must not contain userinfo, a query, or a fragment. NexusRelay exports traces only, accepts W3C `traceparent`/`tracestate`, uses fixed parent-based always-on sampling while enabled, and never exports request headers or bodies. Attach a collector through an operator Compose override; do not expose collector credentials through the endpoint URL or ambient OTEL header variables.

## Shutdown And Restart

Stop containers without deleting persistent state:

```text
make local-core-down
```

Restart with `make local-core-up`. PostgreSQL data, Traefik state, shared configuration, and protected service-secret volumes remain intact.

Before an upgrade or destructive recovery action, follow `backup-restore-key-recovery.md` and `upgrade.md`. Those workflows create separately protected database and cryptographic artifacts and never delete retained backups automatically.

To intentionally replace generated development secrets, first stop the deployment, remove its secret and PostgreSQL volumes with `docker compose --env-file .env -f deploy/compose.yaml --profile core down --volumes`, remove `.local-secrets/`, and run `make local-core-up` again. Secret publication deliberately refuses to mutate an existing protected volume because changing database credentials without coordinated database rotation would make persisted state inaccessible.

## Troubleshooting

- If setup reports an invalid `.env`, compare it with `.env.example`; the validator rejects unknown, duplicate, malformed, and inconsistent settings.
- If secret generation reports an incomplete inventory, do not add or edit individual files. Remove the entire ignored `.local-secrets/` directory only when no retained PostgreSQL volume depends on those credentials.
- If startup times out, inspect `docker compose --env-file .env -f deploy/compose.yaml --profile core ps` and service logs. Errors are designed to identify the failed setting or dependency without printing secret values.
- If port `8080` is occupied, update `HTTP_PORT`, `PUBLIC_API_BASE_URL`, `ADMIN_BASE_URL`, and `ADMIN_ORIGINS` consistently in `.env`. The automated routed check reads `HTTP_PORT` and `ADMIN_HOST` from that file.
