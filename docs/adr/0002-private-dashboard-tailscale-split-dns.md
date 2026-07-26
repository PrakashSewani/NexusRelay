# ADR 0002: Tailscale Split-DNS Reference Profile for Private Administration

- Status: Proposed pending platform verification
- Date: 2026-07-26

## Context

The NexusRelay dashboard controls provider credentials, API keys, budgets, users, and routing. In the optional `prakashsewani.com` reference deployment, local username/password authentication should not be the only barrier protecting the administrative network surface.

The dashboard should be remotely accessible to approved administrators without publishing it through the public Cloudflare Tunnel.

## Decision

This proposed decision applies to an optional private-administration profile. It becomes Accepted only after the documented kernel TUN, route advertisement, split-DNS, and Docker Desktop/macOS tests pass. Other deployments choose local, private, or intentionally public administration through validated configuration and equivalent security controls.

- Use `nexus.prakashsewani.com` as the private administrative hostname.
- Run Tailscale and CoreDNS as Docker Compose containers.
- Use Tailscale subnet/private routing to expose only the Traefik private ingress and CoreDNS addresses needed by the dashboard.
- Assign fixed non-overlapping Docker subnet addresses and run the Tailscale container with kernel TUN networking, `NET_ADMIN`, IP forwarding, and approved advertised routes.
- Configure Tailscale restricted DNS so authorized tailnet clients resolve `nexus.prakashsewani.com` through CoreDNS.
- CoreDNS answers only the exact private hostname and is not publicly reachable.
- Traefik serves HTTPS for the hostname using the DNS-01 certificate obtained through Cloudflare DNS.
- Tailscale grants restrict private DNS and HTTPS access to approved administrator identities/devices.
- NexusRelay login, session, CSRF, and RBAC controls remain mandatory after network admission.

## Alternatives

### Local LAN Only

Rejected because it prevents remote administration when away from home.

### Cloudflare Access

Rejected for the initial dashboard because the user selected a private tailnet path rather than a public hostname with an identity-aware proxy.

### Public DNS Pointing to a Tailscale IP

Rejected because it publishes the private route address and does not meet the selected split-DNS requirement. The hostname itself is not confidential: issuance of its publicly trusted DNS-01 certificate can disclose it through Certificate Transparency logs. Privacy here means no public route/address and no public service reachability, not secrecy of the hostname string.

## Consequences

- Authorized administrator devices must run Tailscale and accept tailnet DNS settings.
- Tailscale control-plane availability is required for new private connectivity, though established connections may continue according to Tailscale behavior.
- Docker Desktop must support kernel TUN and subnet routing for the selected configuration. If it does not, a separate approved design is required rather than weakening dashboard privacy.
- CoreDNS and route configuration become operational components that require health checks and tests.
- The dashboard remains unavailable from ordinary public internet clients.
- Generic application behavior does not depend on Tailscale, CoreDNS, or these hostnames when the profile is disabled.

## Verification Record

The credential-free macOS container feasibility harness is implemented in `deploy/private-admin/test-feasibility.sh` and documented in `docs/runbooks/tailscale-private-admin.md`. It verifies kernel-mode `tailscale0` creation with forwarding enabled and fixed-address, exact-host CoreDNS behavior using the pinned candidate images.

This local evidence is necessary but not sufficient for acceptance. The ADR remains Proposed until one authenticated tailnet session verifies route approval, restricted DNS, administrator grants, authorized DNS/HTTPS access, unauthorized tailnet denial, public-client denial, and persistent-state restart behavior. No private-admin Compose services or enablement are shipped before that record is complete.
