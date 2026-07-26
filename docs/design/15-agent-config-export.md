# Agent Configuration Export Framework

## Purpose

NexusRelay generates merge-safe connection and model configuration for verified external coding agents. Export is a convenience artifact, not a new gateway protocol. OpenCode is the first required V1 exporter. Kilo, CommandCode, and future agents use the same framework only after authoritative verification.

This design implements FR-EXPORT-001 through FR-EXPORT-012 and ADR 0005.

## Internal Export Contract

Agent-specific schemas never enter gateway, routing, or API-key domain types. A narrow exporter registry consumes one neutral input:

```text
AgentExportInput
  exporter_id
  public_api_base_url
  api_key_id and safe key metadata
  api_key_environment_variable
  selected gateway models and guaranteed metadata
  exporter options validated by that profile

AgentExportResult
  exporter_id and exporter_version
  media_type
  suggested_filename
  structured_preview where representable
  serialized_artifact
  schema/profile source and verified_at
  supported run-scoped invocation metadata
  placement guidance and collision warnings
```

Exporter implementations are pure with respect to secrets and persistence. They receive no plaintext gateway key or provider credential. Registry lookup rejects unknown, disabled, or unverified exporters.

## Verification States

Each agent has `docs/agents/<agent>.md` with one state:

```text
not_researched
research_in_progress
profile_drafted
contract_verified
smoke_verified
blocked
```

`contract_verified` is required before an exporter can be enabled. A profile records authoritative documentation, schema or grammar source, retrieval date, version/hash, key-reference syntax, base-URL semantics, model declaration shape, merge behavior, output filename/location guidance, validation fixtures, and known limitations.

V1 ledger:

| Agent | State | Release role |
| --- | --- | --- |
| OpenCode | contract_verified | V1 exporter; supported only through the verified run-scoped invocation |
| Kilo | blocked | Schema shape found, but no authoritative fail-closed placement verified |
| CommandCode | blocked | BYO-provider grammar and safe secret reference are not authoritatively documented |

Names in this ledger do not claim support. Additional agents require only a profile and exporter implementation, not a gateway protocol change.

## Shared Export Rules

- Base URL is the validated runtime `PUBLIC_API_BASE_URL` and ends at `/v1`.
- Selected models are the intersection of enabled gateway models, the selected key's model/provider restrictions, and explicit user selection.
- At least one selected model is required.
- Export generation captures one configuration/version snapshot and does not depend on transient provider health.
- Guaranteed model limits and boolean capabilities are emitted only when every enabled, key-allowed, policy-reachable route that may legally serve the model supports them. Unknown values are omitted.
- Provider connection IDs, upstream model IDs, routing order, health state, prices, and credentials are never exported.
- Cost is omitted because one gateway model may route across targets with different prices.
- Output contains only the NexusRelay connection/provider entry and selected models.
- Exporters must not set agent defaults, active provider lists, workflows, tools, permissions, MCP servers, or unrelated settings.
- A provider/connection identifier collision produces explicit merge guidance; the exporter never silently overwrites an existing user configuration.
- Support requires a fail-closed placement that keeps the base URL and environment-backed key reference authoritative together. Schema validity alone is insufficient.
- If untrusted project configuration can override a trusted global entry and redirect its key reference, static global installation must not be support-claimed.
- Run instructions are part of the versioned exporter contract and must be generated from the verified profile rather than generic merge guidance.

## API-Key Separation

The shared deployment setting `AGENT_API_KEY_ENV` defaults to `NEXUSRELAY_API_KEY` and must match `[A-Z_][A-Z0-9_]*`. Each exporter renders that variable using its verified agent-specific syntax.

Generated artifacts never contain plaintext gateway keys. During API-key creation, the one-time key response separately shows the plaintext and shell setup command. Plaintext exists only in transient request/response memory and client state; it is unavailable to every later export request regardless of whether the UI remains open.

Existing keys can regenerate exports because generation uses only key metadata and an environment-variable reference.

## Control-Plane Contract

```text
GET  /api/control/v1/agent-exporters
POST /api/control/v1/agent-configs/preview
POST /api/control/v1/agent-configs/render
```

`GET /agent-exporters` returns enabled exporter IDs, display names, profile versions, output media types, and safe option schemas.

Preview/render request:

```json
{
  "exporter_id": "opencode",
  "api_key_id": "uuid",
  "model_ids": ["uuid"],
  "options": {}
}
```

Response:

```json
{
  "exporter_id": "opencode",
  "exporter_version": "2026-07-26",
  "media_type": "application/json",
  "filename": "nexusrelay-opencode.json",
  "base_url": "https://gateway.example.com/v1",
  "environment": {
    "variable": "NEXUSRELAY_API_KEY",
    "contains_secret": false
  },
  "config": {},
  "invocation": {
    "placement": "run_scoped_highest_user_merge",
    "disable_project_config": true
  },
  "merge_warnings": []
}
```

Preview returns structured data when the exporter supports it. Render returns the serialized artifact with `Cache-Control: no-store`, a profile-defined media type, and safe `Content-Disposition` filename. Both endpoints are non-secret but remain non-cacheable to avoid confusion in the one-time-key flow.

Authorization requires key-read scope for the selected key: `api_keys.read_own` with ownership enforcement or `api_keys.read_all`. Revoked, disabled, or expired keys are rejected with stable status-specific codes. Cross-tenant and invisible keys map to disclosure-safe 404.

## OpenCode V1 Exporter

With the OpenCode profile at `contract_verified` against authoritative configuration/provider documentation and repository-pinned schema provenance plus golden fixtures, its V1 output is:

- Uses a configured provider ID and display name with safe defaults.
- Uses `npm: "@ai-sdk/openai-compatible"`.
- Uses `options.baseURL` from `PUBLIC_API_BASE_URL`.
- Renders the key as `{env:<AGENT_API_KEY_ENV>}`.
- Emits selected models under the provider entry.
- Does not emit `enabled_providers`, `model`, `small_model`, agents, tools, permissions, or MCP settings.
- Is supplied through `OPENCODE_CONFIG_CONTENT`, the verified highest user/project-controlled merge, for each launched OpenCode process.
- Is launched with `OPENCODE_DISABLE_PROJECT_CONFIG=1` so an untrusted repository cannot redirect the trusted key reference through project configuration.

OpenCode global or project-file installation is not support-claimed. The generated fragment authoritatively binds `npm`, `options.baseURL`, `options.apiKey`, and explicit selected models against lower user/project-controlled tiers. Provider-ID collisions still produce a warning. Organization-console and operating-system managed settings are later trusted administrative tiers; the smoke test inspects resolved configuration and fails if they alter the generated provider entry.

The repository-pinned profile records `opencode-ai@1.18.5`, release commit and package hashes, schema source URL, retrieval date, retrieved schema hash, and deterministic golden fixtures under `docs/agents/fixtures/opencode/`. CI validates against those fixtures without network access. A separate non-blocking drift job may compare the current upstream schema and open a review item; runtime generation never fetches a schema.

## Agent-Specific Profile Gate

Before enabling Kilo, CommandCode, or another exporter:

1. Locate authoritative configuration documentation and schema/grammar.
2. Verify how custom OpenAI-compatible base URLs and bearer keys are represented.
3. Verify environment-variable reference syntax without embedding plaintext.
4. Verify model declaration and optional capability/limit fields.
5. Document merge order, collision behavior, and settings that could disable unrelated providers.
6. Verify a highest-precedence run-scoped placement or equivalent fail-closed mechanism that prevents project configuration from redirecting the trusted key reference.
7. Pin schema or golden fixtures by version/hash and retrieval date.
8. Add deterministic validation, secret-sentinel, model-intersection, precedence-threat, and merge-safety tests.
9. Reach `contract_verified`; optional smoke verification records agent version/date.

If authoritative behavior is unavailable, the exporter remains blocked. NexusRelay does not infer one agent's schema from another or assume that an OpenAI-compatible runtime implies compatible configuration syntax.

## Failure Handling

- Unknown/unverified exporter: 422 `agent_exporter_not_supported`.
- Key not visible: disclosure-safe 404.
- Revoked key: 409 `api_key_revoked`.
- Disabled key: 409 `api_key_disabled`.
- Expired key: 409 `api_key_expired`.
- Selected model not allowed: 422 `model_not_allowed_for_key`.
- Selected model/config changed after page load: 409 `model_configuration_changed`.
- Missing optional verified metadata: omit it and still generate a valid artifact.
- Provider/connection identifier collision: return merge warning; never silently replace unrelated configuration.

## Verification

- Registry rejects unknown and non-verified exporters.
- Every enabled exporter validates against its pinned schema or golden grammar fixtures.
- Generated output contains the configured public base URL, environment-variable reference, and exact selected-model intersection.
- Plaintext gateway keys never enter output, logs, audit events, traces, or persisted export records.
- Two-organization tests prove key/model isolation and own/all permission scope.
- Deterministic ordering, stable serialization, collision warnings, and omission of unknown metadata are tested.
- Precedence-threat tests prove that supported placement cannot send the configured key reference to a project-controlled base URL.
- Run instructions bind the generated artifact to the verified highest-precedence mechanism and disable unsafe project configuration where the profile requires it.
- Resolved-configuration tests fail if a later organization or operating-system managed tier changes the generated provider package, base URL, key reference, or selected models.
- OpenCode opt-in smoke test loads the rendered artifact, lists selected models, and performs a gateway request.
- Agent profile sources and pinned artifacts are rechecked before releases that claim support.

## Requirement Coverage

This design satisfies FR-EXPORT-001 through FR-EXPORT-012 and the V1 agent-configuration acceptance criterion.
