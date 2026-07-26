# User Tasks

This file tracks actions that must be completed by the user or deployment operator. Coding agents must follow the user-task gate in `AGENTS.md`: ask the user whether each blocking item is complete, check it only after explicit confirmation, and do not continue with dependent work while an item remains incomplete.

Never place auth keys, passwords, tokens, cookies, authorization headers, or other secret values in this file, chat, `.env`, command arguments, logs, or committed files.

## Phase 2 Tailscale Private-Admin Gate

These tasks block `TODO.md` items 47 and 48 and acceptance of ADR 0002. Detailed requirements and acceptance evidence are in `docs/runbooks/tailscale-private-admin.md`.

- [ ] Confirm that `172.30.0.0/24`, `172.30.0.10/32`, and `172.30.0.53/32` do not overlap home, Docker, VPN, or tailnet-client routes. If they overlap, provide the replacement subnet, Traefik IP, and DNS IP together.
- [ ] Create a tagged, reusable Tailscale auth key for the NexusRelay private-admin router.
- [ ] Store the auth key in a regular file outside the repository. Set its parent directory to mode `0700` and the file to mode `0600`.
- [ ] Provide the agent only the absolute auth-key file path. Do not paste or disclose the key value.
- [ ] Configure route approval or `autoApprovers` for only `172.30.0.10/32` and `172.30.0.53/32`, or the approved replacement addresses.
- [ ] Configure restricted tailnet DNS for the selected `ADMIN_HOST`, using `172.30.0.53` or the approved replacement `PRIVATE_DNS_IP` as its nameserver.
- [ ] Configure tailnet grants allowing approved administrator identities/devices to query the private DNS address on TCP and UDP port 53.
- [ ] Configure tailnet grants allowing approved administrator identities/devices to reach the private Traefik address on TCP port 443.
- [ ] Select an unauthorized tailnet identity/device for the required denial test and ensure it has no DNS or HTTPS grant to the private addresses.
- [ ] Prepare an authorized remote tailnet client that accepts Tailscale DNS settings.
- [ ] Prepare a non-tailnet public client for the public DNS and reachability denial test.
- [ ] Tell the agent that all tasks above are complete and provide only the protected auth-key file path plus the selected authorized and unauthorized test-client descriptions. Do not provide credentials or secret values.

## Agent-Run Acceptance After Confirmation

The agent performs these checks only after the user explicitly confirms every prerequisite above:

- [ ] Start the authenticated candidate with kernel TUN networking, persistent state, forwarding, and only the two approved host routes.
- [ ] Verify both advertised routes are approved in the tailnet.
- [ ] Verify authorized split-DNS resolution and HTTPS access.
- [ ] Verify unauthorized tailnet DNS and HTTPS denial.
- [ ] Verify non-tailnet public DNS and service denial.
- [ ] Verify unrelated hostnames receive no CoreDNS answer.
- [ ] Verify restart preserves Tailscale node state without exposing or replacing the auth key.
- [ ] Record sanitized platform and acceptance evidence in ADR 0002, update it to Accepted only if every check passes, and then implement the optional private-admin profile.

Agent-run items may be checked by an agent after successful verification. User prerequisite items may be checked only after explicit user confirmation.
