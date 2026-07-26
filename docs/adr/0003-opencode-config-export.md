# ADR 0003: OpenCode JSON Configuration Export

- Status: Superseded by ADR 0005
- Date: 2026-07-26

This ADR is historical. ADR 0005 and `docs/design/15-agent-config-export.md` govern implementation. In particular, `enabled_providers` is not emitted, and static installation into a lower-precedence OpenCode config is not support-claimed.

## Context

NexusRelay users need a configuration artifact that can be used directly by OpenCode for a selected subset of gateway models. Existing NexusRelay API keys cannot be retrieved after creation, and generated files must not encourage committing plaintext keys.

OpenCode supports custom providers using its JSON schema, `@ai-sdk/openai-compatible`, a custom base URL, an API-key option, and an explicit models map.

NexusRelay deployments use different domains and may prefer different environment-variable names, so export values must be deployment configuration rather than repository constants.

## Decision

- Generate an OpenCode JSON provider using `OPENCODE_PROVIDER_ID`, defaulting to `nexusrelay`.
- Use `npm: "@ai-sdk/openai-compatible"`.
- Use `options.baseURL` from validated `PUBLIC_API_BASE_URL`.
- Use `options.apiKey` as an OpenCode environment-variable reference. The superseding shared exporter setting is `AGENT_API_KEY_ENV`, defaulting to `NEXUSRELAY_API_KEY`; never embed the plaintext gateway key.
- Include only user-selected gateway models that the selected key is allowed to access.
- Emit model references as `<configured-provider-id>/<gateway-model-key>`.
- Include the configured provider ID in `enabled_providers` in the standalone generated config.
- Do not set default model, small model, agents, tools, permissions, or MCP configuration in V1.
- Validate generated JSON against `https://opencode.ai/config.json` in CI and record the schema verification date.

## Supersession Security Note

Subsequent verification found that OpenCode project configuration can override lower-precedence global configuration. A project could therefore replace the NexusRelay `baseURL` while retaining a global `{env:NEXUSRELAY_API_KEY}` reference. Implementing this ADR's standalone static-file guidance would not fail closed.

The superseding contract uses a generated run-scoped fragment through `OPENCODE_CONFIG_CONTENT`, the highest user/project-controlled merge in the pinned source, and launches with `OPENCODE_DISABLE_PROJECT_CONFIG=1`. The fragment binds the package, base URL, environment reference, and explicit models together. Later organization and operating-system managed settings are trusted administrative tiers and must be checked through resolved-configuration smoke tests. See the OpenCode profile for the pinned version, hashes, fixtures, and supported invocation.

## Alternatives

### Embed the Key in JSON

Rejected because the file could be committed, shared, or retained in downloads. The API key is displayed separately once.

### OpenCode `/connect` Integration

Deferred because it would require an OpenCode provider/auth integration beyond a standard JSON export.

### Generate Full Agent Configuration

Rejected for V1 because it would overwrite or conflict with unrelated user preferences and expands scope beyond provider/model setup.

## Consequences

- Users must set the configured key environment variable, which defaults to `NEXUSRELAY_API_KEY`, in their environment or secret-management workflow.
- Non-secret configs can be regenerated for existing keys.
- OpenCode schema changes may require exporter updates and compatibility testing.
- Static cost metadata is omitted because NexusRelay may route one gateway model across providers with different prices.
