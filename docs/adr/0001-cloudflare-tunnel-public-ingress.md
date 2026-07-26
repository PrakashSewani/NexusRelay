# ADR 0001: Cloudflare Tunnel Reference Profile for Public Inference

- Status: Accepted
- Date: 2026-07-26

## Context

NexusRelay is a generic product with a core local Compose profile. One initial reference deployment runs on a MacBook and serves a remote user over the public internet using the operator-owned domain `prakashsewani.com`. Direct router port forwarding would expose the home network address and require public-IP, NAT, firewall, and certificate maintenance.

The public surface must include only the OpenAI-compatible inference API. The administrative dashboard and control-plane API must remain private.

## Decision

This decision applies to the optional `prakashsewani.com` reference profile, not to mandatory core product behavior.

- Publish `api.prakashsewani.com` through a containerized Cloudflare Tunnel.
- Use a locally managed named tunnel with tunnel UUID, credential JSON, and repository-generated ingress configuration. Do not mix this with remotely managed token-run semantics.
- Do not forward router ports for NexusRelay.
- Tunnel ingress routes only that hostname to the Traefik HTTPS origin and ends with an HTTP 404 catch-all.
- Traefik routes only `/v1/*` for the public hostname; all dashboard/control-plane paths return 404.
- Cloudflare caching and transformations are disabled for inference routes, and SSE remains unbuffered.
- Traefik obtains origin certificates using ACME DNS-01 with a Cloudflare API token scoped to DNS edit for the `prakashsewani.com` zone and zone read.
- NexusRelay remains responsible for API-key authentication, limits, budgets, and audit/usage records. Cloudflare controls are defense in depth only.

## Trust Consequence

Cloudflare terminates public TLS. Traffic is encrypted from the client to Cloudflare and from the tunnel to the origin, but it is not cryptographically opaque to Cloudflare. Gateway API keys, prompts, completions, and tool arguments can technically be inspected at the Cloudflare edge. This limitation is explicitly accepted for the initial deployment.

Provider credentials and administrative session cookies do not traverse the public hostname.

## Alternatives

### Direct Router Forwarding

Rejected for the initial deployment because it increases network exposure and operational complexity. It would provide TLS termination only on the Mac when configured correctly.

### Tailscale-Only Access

Rejected as the sole inference path because every remote API consumer would need tailnet membership. It remains the private dashboard path.

### Cloudflare Access on the API

Rejected because generic OpenAI clients such as OpenCode cannot necessarily perform an interactive Cloudflare identity flow. NexusRelay bearer keys are the application authentication mechanism.

## Consequences

- Cloudflare and its availability become dependencies for public inference access.
- The MacBook, Docker, tunnel, internet connection, and provider connections must remain online.
- There are no inbound router ports for NexusRelay.
- Public hostname/path isolation requires automated negative tests.
- A future move to direct end-to-end TLS requires a new ADR and deployment update.
- Generic application code derives hostnames and URLs from configuration and must not contain `prakashsewani.com` constants.
- Deployments that do not enable this profile do not depend on Cloudflare.
