# Tailscale Private-Admin Acceptance

## Status

The private-admin profile is not yet supported or enabled. ADR 0002 remains Proposed until the authenticated tailnet checks in this runbook pass. NexusRelay deployment validation continues to reject `ENABLE_TAILSCALE_PRIVATE_ADMIN=true`.

The credential-free feasibility harness proves only that the current Docker runtime can create a kernel TUN interface and can serve an exact fixed-address CoreDNS answer. It does not prove tailnet route approval, grants, restricted DNS, or remote authorization behavior.

## Local Feasibility

Requirements:

- macOS with the deployment Docker runtime running.
- `/dev/net/tun` support in Linux containers.
- Permission for a test container to receive `NET_ADMIN` and `net.ipv4.ip_forward=1`.

Run:

```text
make private-admin-feasibility-test
```

The test uses the candidate `172.30.0.0/24` subnet only in an isolated temporary Docker network. Before an authenticated deployment, verify that this subnet does not overlap any Docker, home, VPN, or tailnet client route. Change `PRIVATE_INGRESS_SUBNET`, `PRIVATE_TRAEFIK_IP`, `PRIVATE_DNS_IP`, and `TAILSCALE_ADVERTISE_ROUTES` together if it overlaps.

## Operator Prerequisites

Create a tagged reusable Tailscale auth key through the Tailscale admin console. Do not paste it into chat, commit it, place it in `.env`, or expose it in command arguments. Store it in a regular file outside the repository with directory mode `0700` and file mode `0600`.

Configure the tailnet before the authenticated test:

- Approve or auto-approve only `172.30.0.10/32` and `172.30.0.53/32`, or the replacement addresses selected after overlap review.
- Restrict DNS for the configured `ADMIN_HOST` to `PRIVATE_DNS_IP`.
- Grant approved administrator identities/devices DNS access to `PRIVATE_DNS_IP:53` over UDP and TCP.
- Grant approved administrator identities/devices HTTPS access to `PRIVATE_TRAEFIK_IP:443`.
- Do not grant these destinations to the identity/device used for the negative test.
- Keep NexusRelay login, session, CSRF, and RBAC controls enabled after network admission.

Provide the protected auth-key file path to the operator running the acceptance test, never the key value.

## Required Acceptance Evidence

ADR 0002 may become Accepted only when one test session records all of the following against the pinned candidate images and selected subnet:

1. The Tailscale container starts with kernel TUN networking, persistent state, `NET_ADMIN`, forwarding sysctls, and only the two documented host routes advertised.
2. The Tailscale admin console shows both routes approved.
3. An authorized remote tailnet client using tailnet DNS resolves exactly `ADMIN_HOST` to `PRIVATE_TRAEFIK_IP`.
4. The authorized client reaches the dashboard over HTTPS with the expected hostname and a valid certificate.
5. An unauthorized tailnet identity/device cannot query the private DNS address and cannot reach private HTTPS.
6. A non-tailnet public client receives no public DNS route/address for `ADMIN_HOST` and cannot reach the service.
7. CoreDNS does not answer unrelated hostnames.
8. Container restart preserves node state and does not require a newly exposed key.

Record the Docker runtime/version, macOS version, Tailscale image/version, CoreDNS image/version, subnet, route approval result, DNS result, HTTPS result, and both denial results in ADR 0002. Do not record auth keys, cookies, authorization headers, or response bodies.

## Failure Policy

If any authenticated check fails, keep ADR 0002 Proposed and leave `ENABLE_TAILSCALE_PRIVATE_ADMIN=true` rejected. Do not publish the dashboard through the Cloudflare public tunnel as a fallback. Approving a host-Tailscale alternative requires an updated ADR before implementation.
