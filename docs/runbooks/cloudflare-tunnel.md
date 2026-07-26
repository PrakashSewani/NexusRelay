# Cloudflare Tunnel Public API Runbook

## Scope

This optional profile implements ADR 0001 and `docs/design/11-operations-security-testing.md`. It uses a locally managed named Cloudflare Tunnel to publish exactly `PUBLIC_API_HOST`; cloudflared forwards to Traefik over validated HTTPS, and Traefik exposes only `/v1` for that hostname. The dashboard and control-plane hostname are not tunnel routes.

The core profile remains independent and disabled by default. Cloudflare terminates public TLS and can technically inspect API keys and model traffic; this path is not cryptographically opaque to Cloudflare.

## Prerequisites

- A Cloudflare zone containing `PUBLIC_API_HOST`.
- A locally managed named tunnel and its credential JSON. Do not use a remotely managed tunnel token with this profile.
- A Cloudflare API token scoped to `Zone:DNS:Edit` and `Zone:Zone:Read` for the required zone. Traefik uses it for ACME DNS-01.
- Docker Engine or Docker Desktop with Compose v2.

## Configuration

Set these values in the ignored `.env` file:

```text
PUBLIC_API_BASE_URL=https://api.example.com/v1
PUBLIC_API_HOST=api.example.com
ADMIN_BASE_URL=https://admin.internal.example
ADMIN_HOST=admin.internal.example
ADMIN_ORIGINS=https://admin.internal.example
TLS_MODE=acme
ACME_EMAIL=operator@example.com
ACME_DNS_PROVIDER=cloudflare
ENABLE_CLOUDFLARE_TUNNEL=true
CLOUDFLARE_TUNNEL_ID=11111111-2222-4333-8444-555555555555
```

Set `NEXUSRELAY_ENV=production` only when the complete production deployment is ready; production also requires the other production-grade settings validated by `.env.example`, including encrypted database transport. The public and admin hostnames must differ.

Create the ignored Cloudflare source-secret directory without printing either secret:

```text
mkdir -m 0700 .cloudflare-secrets
install -m 0600 /path/to/<tunnel-id>.json .cloudflare-secrets/cloudflare_tunnel_credentials.json
install -m 0600 /path/to/cloudflare-dns-api-token .cloudflare-secrets/acme_dns_api_token
```

The credential JSON `TunnelID` must equal `CLOUDFLARE_TUNNEL_ID`. Startup validation rejects symlinks, permissive modes, missing/extra files, mismatched tunnel IDs, an HTTP public URL, a shared admin/public hostname, and non-Cloudflare ACME configuration. A networkless publisher copies the tunnel credential and ACME token to separate service-specific named volumes as mode `0400`, owned by UID/GID `65532`; cloudflared and Traefik never receive the host directory.

## Validate And Start

Run deterministic validation without live credentials or a tunnel connection:

```text
make cloudflare-config-test
```

The test generates a representative config, runs the pinned `cloudflared tunnel ingress validate`, checks matching with `cloudflared tunnel ingress rule`, verifies the exact hostname plus mandatory 404 catch-all, and tests protected secret publication.

Start core and the optional profile together:

```text
docker compose --env-file .env -f deploy/compose.yaml --profile core --profile cloudflare up --build --wait
```

The generated tunnel ingress has one hostname rule targeting `https://proxy:8443`, explicitly sets both origin HTTP Host and certificate/SNI name to `PUBLIC_API_HOST`, leaves certificate validation enabled, and ends with `http_status:404`. Traefik obtains a public origin certificate through Cloudflare DNS-01, requires TLS 1.2 or newer, routes only `/v1` to gateway, and returns 404 for every other path on the public hostname.

## Cloudflare Zone Rules

Create a proxied DNS route from `PUBLIC_API_HOST` to the named tunnel using the Cloudflare dashboard or the authenticated `cloudflared tunnel route dns <tunnel-id> <host>` workflow. DNS provisioning is intentionally not performed by Compose because it requires account-level operator authorization.

For the expression `http.host eq "api.example.com" and starts_with(http.request.uri.path, "/v1")`:

- Create a Cache Rule with cache eligibility set to **Bypass cache**.
- Do not create URL rewrites, request/response header transformations, compression changes, or Workers that buffer or transform `/v1` traffic.
- Disable any feature that buffers or rewrites `text/event-stream`; preserve incremental SSE chunks and chunked transfer encoding.
- Cloudflare rate limiting may reject obvious abuse, but NexusRelay API-key authentication, limits, budgets, accounting, and audit remain authoritative.

Use the configured hostname in the expression instead of the example. Verify the effective behavior with Cloudflare Trace and an SSE client before declaring the public route ready.

## Verification

After DNS and the tunnel are active:

```text
docker compose --env-file .env -f deploy/compose.yaml --profile core --profile cloudflare ps
curl --fail --silent --show-error https://api.example.com/v1/models
curl --silent --show-error --output /dev/null --write-out '%{http_code}\n' https://api.example.com/
curl --silent --show-error --output /dev/null --write-out '%{http_code}\n' https://api.example.com/api/control/v1
```

The two negative checks must return `404`. The `/v1/models` application response becomes functional in Phase 6; before then, verify that it reaches the gateway route rather than the dashboard or control plane. Also verify the origin certificate presented for `PUBLIC_API_HOST`, tunnel connector health, and the DNS route in Cloudflare before declaring readiness.

## Rotation And Shutdown

Stop without deleting retained state:

```text
docker compose --env-file .env -f deploy/compose.yaml --profile core --profile cloudflare down --remove-orphans
```

Published secret volumes are immutable through this flow. To rotate either Cloudflare secret, stop the deployment, update both the protected source inventory and Cloudflare account state as needed, remove only the `cloudflared-secrets` and `traefik-cloudflare-secrets` named volumes for this Compose project, then restart and verify DNS, certificate issuance, tunnel health, `/v1`, and the negative routes. Do not remove PostgreSQL or core application secret volumes for a tunnel-secret rotation.
