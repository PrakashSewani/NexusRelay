# Provider Verification Ledger

## Purpose

Provider-specific behavior must be verified from authoritative documentation before implementation. This ledger prevents assumptions from entering adapters and distinguishes framework-level design completion from provider readiness.

## Readiness States

```text
not_researched
research_in_progress
profile_drafted
contract_verified
live_smoke_verified
blocked
```

`contract_verified` is the minimum state for V1 support. `live_smoke_verified` is desirable for release providers when credentials are available but is not required in default CI.

Profiles also record owner/reviewer, source version or content hash, `verified_at`, `review_due_at`, and downgrade triggers. A material provider API/documentation change, failed contract fixture, failed release smoke test, or expired review date moves the profile out of `contract_verified` until reviewed.

## Provider Ledger

| Provider | Intended adapter | Current design state | Required before implementation |
| --- | --- | --- | --- |
| OpenAI | Native/OpenAI-compatible baseline | not_researched | Profile and endpoint/stream/error/usage verification |
| Anthropic | Dedicated native adapter | not_researched | Messages/tool/stream/usage/error profile |
| Google Gemini | Dedicated native adapter | not_researched | GenerateContent/stream/tool/usage/error profile |
| OpenRouter | Shared OpenAI-compatible profile | not_researched | Compatibility quirks, headers, usage/cost, model listing |
| Ollama | Shared or dedicated profile based on verification | not_researched | Local/private URL, auth, model listing, streaming, usage behavior |
| Groq | Shared OpenAI-compatible profile | not_researched | Supported endpoints/fields, headers, errors, limits, usage |
| Xiaomi MiMo | Undecided pending authoritative docs | blocked | Authoritative API/auth/specification must be located and reviewed |
| CommandCode Provider API | Undecided pending authoritative docs | blocked | Authoritative API/auth/specification must be located and reviewed |
| Custom OpenAI-compatible | Shared configurable adapter | profile_drafted | Contract matrix and safe configuration constraints |

No provider in `not_researched` or `blocked` is implementation-ready even though the generic adapter architecture is designed.

## Required Profile Template

Each provider receives `docs/providers/<provider>.md` containing:

### Sources

- Authoritative documentation URLs.
- Documentation/API version.
- Retrieval date.
- SDK source/version if used.

### Connection

- Base URL and endpoint joining rules.
- Authentication headers/query parameters.
- Required API version/project/organization fields.
- TLS, redirect, and private-network behavior.
- Connection test operation.
- Ongoing active-health probe operation, expected cost/quota impact, timeout, and result classification.

### Operations

For Chat Completions, Responses-equivalent, and Embeddings:

- Native endpoint.
- Supported request fields from the public matrix.
- Translation constraints and unsupported combinations.
- Response mapping.
- Streaming framing/events and terminator.
- Cancellation behavior.

### Capabilities and Limits

- Model discovery/listing.
- Text/image/audio modalities.
- Tools and parallel calls.
- JSON mode/structured output.
- Context/output/input limits and source of truth.
- Usage fields and tokenizer behavior.

### Errors and Limits

- HTTP/provider error codes to normalized categories.
- Retry safety and `Retry-After` behavior.
- Provider quota versus request rate limits.
- Content filtering/refusal semantics.
- Maximum response/error body handling.

### Cost

- Provider-reported cost availability.
- Token categories, cached/reasoning token handling.
- Pricing source and currency.
- Charge behavior for failed/cancelled requests.
- Mandatory gateway policy when provider billing is undocumented: conservative estimate formula or ineligibility under hard monetary budgets. `contract_verified` cannot leave this unspecified.

### Verification

- Deterministic mock contract scenarios.
- Official SDK compatibility implications.
- Opt-in live smoke test and required environment variables.
- Known limitations displayed in control plane.

## Shared OpenAI-Compatible Gate

A provider may use the shared adapter only if verified for:

- Authentication/header behavior.
- Endpoint paths and base URL joining.
- Request fields claimed as supported.
- SSE framing, data shape, and terminal behavior.
- Error envelope and rate-limit headers.
- Usage fields.
- Model listing or static configuration.

Differences are represented by bounded typed profile options. Arbitrary scripting, user-supplied transformation templates, and silent field dropping are prohibited.

## Custom Endpoint Baseline

The custom OpenAI-compatible adapter supports only the public compatibility matrix and administrator-configured model IDs by default. Optional endpoint availability and safe headers are explicit. A successful connection test does not prove every capability; administrators must select/narrow capabilities, and contract warnings are visible. Capability metadata records provenance as `verified_profile`, `discovered`, or `operator_asserted`. Operator-asserted features are the documented exception to pre-dispatch certainty and never widen translation support implemented by the shared adapter.

## Release Gate

Before a provider is marked supported:

1. Provider profile reaches `profile_drafted` with authoritative sources.
2. Security/network behavior is reviewed.
3. Mock contract tests pass for non-streaming, streaming, errors, usage, timeout, and cancellation.
4. Capability matrix is wired into routing and control-plane display.
5. Public compatibility limitations are documented.
6. Status moves to `contract_verified`.
7. Optional live smoke verification records provider/API version and date without storing credentials or model content.
8. Review due date and downgrade triggers are recorded and enforced by release tooling.
