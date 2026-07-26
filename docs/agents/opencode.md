# OpenCode Exporter Profile

- State: `profile_drafted`
- Owner/reviewer: unassigned
- Retrieved: 2026-07-26
- Review due: before V1 exporter implementation

## Authoritative Sources

- Configuration documentation: `https://opencode.ai/docs/config/`
- Provider documentation: `https://opencode.ai/docs/providers/`
- Schema source: `https://opencode.ai/config.json`

The schema must be vendored under a reviewed repository path with retrieval date and SHA-256 content hash before this profile can reach `contract_verified`. Runtime generation must not fetch the mutable URL.

## Draft Contract

- Output media type: `application/json`.
- Suggested filename: `opencode.json`.
- Custom provider package: `@ai-sdk/openai-compatible`.
- `options.baseURL` is validated `PUBLIC_API_BASE_URL` ending in `/v1`.
- `options.apiKey` renders `{env:<AGENT_API_KEY_ENV>}`.
- Models are explicit entries keyed by NexusRelay gateway model key.
- Export does not emit `enabled_providers`, default model, small model, agents, tools, permissions, MCP servers, or unrelated settings.
- Existing provider-ID collisions require explicit merge/replacement guidance.

## Required Verification

- Pin the exact schema artifact and content hash.
- Confirm conservative provider-ID character/length validation.
- Confirm merge precedence and collision behavior from authoritative documentation.
- Validate minimal, capability-rich, and metadata-omission fixtures.
- Verify environment-variable reference behavior without plaintext fallback.
- Run an opt-in smoke test with a pinned OpenCode version against a mock or local NexusRelay gateway.

Until these checks pass, the exporter remains unavailable despite the framework and draft shape.
