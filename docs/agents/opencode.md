# OpenCode Exporter Profile

- State: `contract_verified`
- Owner/reviewer: NexusRelay maintainers
- Retrieved: 2026-07-26
- Review due: 2026-10-26, before changing the supported OpenCode version, or before V1 release, whichever comes first
- Verified package: `opencode-ai@1.18.5`
- Release commit: `2b2aacc93975330f9fd045d4306f698b0c6a8f8f`

## Authoritative Sources

- Configuration documentation: `https://opencode.ai/docs/config/`
- Provider documentation: `https://opencode.ai/docs/providers/`
- Schema source: `https://opencode.ai/config.json`

The schema retrieved on 2026-07-26 had SHA-256 `8ffffc8622f2bbee5e9b1e57bf2509910f2a6dfc237458766bfaa5e295787a2e`. The package had npm integrity `sha512-Q0jlX4ihn7veMeYsLX3c4PYFAKIURU3GIpXt1FnhNxNn3v8+RpIZ8z9umG5D0r8g8Smp9fZLGjgLe/9mJ4NyYw==` and npm SHA-1 `91dcee1ca87ac6f445b4fbf7a3375de170acbfe6`.

The deterministic golden contract is pinned under `docs/agents/fixtures/opencode/`. Runtime generation and blocking CI validation do not fetch the mutable schema URL or package metadata.

## Verified Contract

- Output media type: `application/json`.
- Suggested filename: `nexusrelay-opencode.json`.
- Custom provider package: `@ai-sdk/openai-compatible`.
- `options.baseURL` is validated `PUBLIC_API_BASE_URL` ending in `/v1`.
- `options.apiKey` renders `{env:<AGENT_API_KEY_ENV>}`.
- Models are explicit entries keyed by NexusRelay gateway model key.
- Export does not emit `enabled_providers`, default model, small model, agents, tools, permissions, MCP servers, or unrelated settings.
- Existing provider-ID collisions require explicit guidance.

The verified provider shape is:

```json
{
  "$schema": "https://opencode.ai/config.json",
  "provider": {
    "nexusrelay": {
      "name": "NexusRelay",
      "npm": "@ai-sdk/openai-compatible",
      "options": {
        "baseURL": "https://gateway.example.com/v1",
        "apiKey": "{env:NEXUSRELAY_API_KEY}"
      },
      "models": {
        "gateway-model": {}
      }
    }
  }
}
```

## Supported Placement And Invocation

OpenCode deep-merges configuration and project configuration overrides global configuration. A global NexusRelay provider entry is therefore unsafe: a repository can replace `options.baseURL` while retaining the trusted `{env:NEXUSRELAY_API_KEY}` reference.

The supported artifact is a run-scoped JSON fragment supplied through `OPENCODE_CONFIG_CONTENT`, which the pinned source applies after user global, custom-path, project, and `.opencode` configuration. The supported invocation also sets `OPENCODE_DISABLE_PROJECT_CONFIG=1` so an untrusted repository configuration is not loaded:

```sh
OPENCODE_DISABLE_PROJECT_CONFIG=1 \
OPENCODE_CONFIG_CONTENT="$(cat /path/to/nexusrelay-opencode.json)" \
opencode
```

The control plane must render shell-specific quoting or provide equivalent process environment setup; the JSON file itself remains non-secret. Users set `NEXUSRELAY_API_KEY` separately in the process environment. OpenCode must be started as a new process because configuration is loaded at startup.

Static installation into `~/.config/opencode/opencode.json`, `opencode.json`, `opencode.jsonc`, or `.opencode/opencode.json` is not support-claimed. Generic merge instructions must not suggest copying the provider into those files. If the configured provider ID collides with a lower-precedence user entry, the run-scoped fragment authoritatively replaces the package, base URL, key reference, and selected model entries emitted by NexusRelay; generation still returns a collision warning so the user can choose a different provider ID.

OpenCode organization-console configuration and operating-system managed configuration are loaded after inline content in the pinned source. Those are trusted administrative policy tiers, not repository-controlled input. An installation that does not trust those administrators must not use the exporter. The smoke test must inspect the resolved configuration and fail if a later managed tier changes the generated provider package, base URL, key reference, or selected models.

## Metadata Rules

- Emit `name` when a safe configured display name is available.
- Emit model `name`, `attachment`, `reasoning`, `temperature`, `tool_call`, `limit`, and `modalities` only when guaranteed under the shared export rules.
- Omit cost, upstream IDs, route details, unknown limits, and unknown capabilities.
- Preserve deterministic provider, option, model, and metadata ordering in serialized output.

## Validation

- Validate generated output against the pinned golden contract and representative minimal, capability, and omission fixtures.
- Assert the exact package, base URL, environment reference, provider ID, and selected-model intersection.
- Reject output containing a plaintext-key sentinel or any key value other than `{env:<validated-name>}`.
- Assert omission of `enabled_providers` and unrelated top-level settings.
- Exercise the project-override threat fixture and require the supported run-scoped invocation.
- Inspect resolved configuration and reject a later organization/managed override of the generated provider entry.
- Treat upstream schema drift as a non-blocking review signal until the pinned profile is deliberately updated.

## Remaining Smoke Gate

- Run an opt-in smoke test with pinned `opencode-ai@1.18.5` against a mock or local NexusRelay gateway.
- Confirm the selected models are listed and a request reaches only the configured NexusRelay base URL.

The exporter is contract-verified but not `smoke_verified`. A version change requires deliberate profile and fixture review.
