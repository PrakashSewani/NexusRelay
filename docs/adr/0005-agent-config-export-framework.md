# ADR 0005: Agent Configuration Export Framework

- Status: Accepted
- Date: 2026-07-26
- Supersedes: ADR 0003

## Context

NexusRelay users need configuration artifacts for coding agents such as OpenCode, Kilo, and CommandCode. These tools have independent schemas, key-reference syntax, merge behavior, and release cadence. Treating one agent's schema as the product-wide contract would couple NexusRelay to that tool and encourage guessed configuration for other agents.

## Decision

- Implement a registry of named, versioned agent exporters behind one provider-neutral internal contract.
- Every exporter accepts validated public base URL, environment-variable name, selected key metadata, and selected gateway-model metadata. It returns a structured preview, serialized artifact, media type, filename, profile version, and merge guidance.
- Exporters generate only the NexusRelay connection/provider entry and selected models. They do not own agent defaults, tools, workflows, permissions, MCP servers, or unrelated settings.
- Generated artifacts never embed plaintext gateway keys. The shared default environment-variable name is `NEXUSRELAY_API_KEY`; each exporter renders the reference using verified agent-specific syntax.
- Every exporter has an authoritative profile, repository-pinned schema or golden fixtures, deterministic validation, and explicit merge/collision behavior before reaching `contract_verified`.
- OpenCode is the first release-required exporter. Kilo, CommandCode, and other exporters remain unavailable until their profiles are verified.
- The OpenCode exporter uses `@ai-sdk/openai-compatible`, derives its base URL from `PUBLIC_API_BASE_URL`, renders `{env:<name>}`, and does not emit `enabled_providers`.

## Alternatives

### One Generic Manifest Only

Rejected as the only V1 output because users need artifacts accepted by actual agents, and agent key-reference and model schemas differ. A neutral internal representation still feeds every exporter.

### Hardcode Every Named Agent

Rejected because it would mix independent contracts without a common verification lifecycle and encourage support claims based on assumptions.

### Generate Full Agent Configuration

Rejected because it would own preferences and security settings outside NexusRelay's connection/model responsibility and create destructive merge behavior.

## Consequences

- Adding an agent does not change gateway or key-management contracts.
- Each exporter evolves only through its versioned profile and compatibility tests.
- OpenCode remains the sole release-blocking exporter until requirements intentionally add another.
- Generated fragments are merge-safe and connection/model-only by default.
