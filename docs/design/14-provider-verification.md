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

`contract_verified` is the minimum research state for V1 adapter implementation. It does not by itself mean the adapter exists or is release-supported. `live_smoke_verified` is desirable for release providers when credentials are available but is not required in default CI.

Profiles also record owner/reviewer, source version or content hash, `verified_at`, `review_due_at`, and downgrade triggers. A material provider API/documentation change, failed contract fixture, failed release smoke test, or expired review date moves the profile out of `contract_verified` until reviewed.

The custom OpenAI-compatible profile is a special contract profile because no external authority governs arbitrary endpoints. Its `contract_verified` state verifies only NexusRelay's bounded adapter/configuration contract. Every deployment endpoint, model ID, and capability remains `operator_asserted`; tests may add time-bounded observations but cannot label the endpoint `verified_profile` or provider-verified.

## Provider Ledger

| Provider | Intended adapter | Current design state | Next gate |
| --- | --- | --- | --- |
| OpenAI | Native/OpenAI baseline | contract_verified | Implement deterministic fixtures and adapter from [the verified profile](../providers/openai.md) |
| Anthropic | Dedicated native adapter | contract_verified | Implement deterministic fixtures and adapter from [the verified profile](../providers/anthropic.md) |
| Google Gemini | Dedicated native adapter | profile_drafted | Define and review the exhaustive native-to-normalized finish mapping, then fixtures, before implementation from [the draft profile](../providers/gemini.md) |
| OpenRouter | Shared OpenAI-compatible profile | profile_drafted | Pin the accepted Responses lifecycle/terminal grammar and fixtures before implementation from [the draft profile](../providers/openrouter.md) |
| Ollama | Dedicated native adapter | contract_verified | Implement native NDJSON/private-network fixtures from [the verified profile](../providers/ollama.md) |
| Groq | Shared OpenAI-compatible profile | profile_drafted | Establish exact final Chat stream usage shape/ordering and beta Responses lifecycle fixtures before implementation from [the draft profile](../providers/groq.md) |
| Xiaomi MiMo | Undecided pending authoritative docs | blocked | Missing hosted API contract documented in [provider research](../providers/xiaomi-mimo.md) |
| CommandCode Provider API | Undecided pending complete authoritative contract | blocked | Missing mandatory semantics documented in [provider research](../providers/commandcode.md) |
| Custom OpenAI-compatible | Shared configurable adapter | contract_verified for the bounded NexusRelay contract; each endpoint remains operator_asserted | Implement typed schema and deterministic fixtures from [the verified contract profile](../providers/openai-compatible.md) |

No provider below `contract_verified`, including `profile_drafted` and `blocked`, is implementation-ready even though the generic adapter architecture is designed.

## Required Profile Template

Each provider receives `docs/providers/<provider>.md` containing:

External provider profiles use authoritative provider sources. The custom OpenAI-compatible contract profile instead cites the approved NexusRelay requirements and designs that define its bounded schema and behavior; operator-supplied endpoint documentation remains assertion evidence, not an authoritative NexusRelay verification source.

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

Differences are represented by a closed registry of bounded, versioned typed profiles. The operator-configured custom profile exposes only its documented connection/dialect enums. Named profiles may add code-owned operation-specific augmentations only when the exact upstream field, value derivation, conflict rule, and fixtures are reviewed. No profile accepts arbitrary JSON/pass-through maps, JSON paths, scripting, user-supplied transformation templates, client-owned provider objects, or silent field dropping.

For OpenRouter, `openrouter_require_parameters_v1` is the only initial request augmentation: Chat requests containing capability-gated fields receive fixed `provider.require_parameters: true`. Neither the client nor connection configuration may supply or override the upstream `provider` object, and no OpenRouter provider ordering, fallback list, plugin, or routing control is passed through. Adding another augmentation requires a profile revision and deterministic fixtures; the generic custom profile cannot opt into named-provider augmentations.

## Custom Endpoint Baseline

The custom OpenAI-compatible adapter supports only the public compatibility matrix and administrator-configured static model IDs. Its versioned schema permits a validated base URL, typed Bearer-or-none authentication, bounded safe optional headers, four bounded timeout phases, named public/private network policy, and explicit Models/Chat/Responses/Embeddings endpoint availability. Endpoint paths remain the standard OpenAI paths beneath the configured base; arbitrary paths and transformations are prohibited.

Streaming, usage, errors, and retry hints use only reviewed enums defined in the profile. There are no scripts, templates, JSON paths, regular expressions, arbitrary field/status maps, client-header forwarding, or silent field dropping. Unknown schema fields and enum values fail validation.

All endpoint/model feature capabilities default to false or unknown. Effective support is the intersection of shared-adapter translation, enabled operation/dialect, operator assertion for the static model, safe observations, and administrator narrowing. Operator assertions never widen translation support. Unknown or unsupported features make the target ineligible before dispatch.

Capability provenance for custom endpoints is `operator_asserted`, optionally accompanied by an expiring `observed` test result. `verified_profile` is reserved for facts established by an authoritative provider profile and must not be applied merely because a Models request or smoke test succeeded.

Every custom URL follows the design 04 SSRF policy. Public egress rejects non-public addresses; intentional private/local egress requires an authorized named private-network policy and all resolved addresses to fall within that policy's CIDRs. HTTP is private-policy-only, redirects are disabled, and Host/TLS SNI derive solely from the validated URL.

The control plane must warn that endpoint compatibility, capability, availability, cancellation, usage, and billing are operator assertions. Custom endpoints have no generic authoritative cost source: hard monetary budgets require complete versioned operator pricing, and ambiguous dispatched attempts reconcile conservatively rather than as zero. The complete normative contract and fixture matrix are in [the custom OpenAI-compatible profile](../providers/openai-compatible.md).

## Release Gate

Before a provider is marked supported:

1. Provider profile reaches `profile_drafted` with authoritative external sources, or approved NexusRelay contract sources for the custom OpenAI-compatible profile.
2. Security/network behavior, operations, stream grammar, errors, usage, cost policy, and capability gates are reviewed.
3. Status moves to `contract_verified`, which permits adapter and fixture implementation but does not claim release support.
4. Mock contract tests pass for non-streaming, streaming, errors, usage, timeout, and cancellation.
5. Capability matrix is wired into routing and control-plane display.
6. Public compatibility limitations are documented.
7. Optional live smoke verification records provider/API version and date without storing credentials or model content.
8. Review due date and downgrade triggers are recorded and enforced by release tooling.

`profile_drafted` is not implementation-ready. Any profile with an undefined normalized finish mapping, accepted stream lifecycle/terminal grammar, or final stream usage shape remains at `profile_drafted` even when endpoint/authentication facts are otherwise authoritative.

## Blocked Baseline Provider Decision

Xiaomi MiMo and CommandCode Provider API are baseline candidates but currently blocked, not implicit release commitments. Before V1 provider scope freeze, maintainers must record exactly one disposition for each in requirements, this ledger, its provider profile, and traceability:

1. **Verify and include:** first-party evidence fills every mandatory profile field, the profile reaches `contract_verified`, and the normal implementation/release gates apply.
2. **Redefine:** an approved requirements/design change replaces the hosted-provider assumption with a bounded deployment profile, with a new verification basis and no reuse of unsupported hosted-provider claims.
3. **Defer or remove:** requirements and acceptance criteria explicitly remove the provider from the V1 release baseline while retaining the research profile as evidence.

Absence of a decision, marketing compatibility language, a community endpoint, or successful ad hoc traffic cannot advance a blocked provider. V1 release is blocked while a provider remains both in the committed release baseline and below `contract_verified`.
