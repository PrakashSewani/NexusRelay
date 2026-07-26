# OpenRouter Provider Profile

## Verification Record

| Field | Value |
| --- | --- |
| Provider type | `openrouter` |
| Intended adapter | Shared OpenAI-compatible adapter with a typed OpenRouter profile |
| Status | `profile_drafted` |
| Owner / reviewer | NexusRelay maintainers |
| Drafted at / review due | 2026-07-26 / before promotion or V1 scope freeze |
| API version | `/api/v1`; current official OpenAPI and versioning policy |
| Source identity | Official URLs and retrieval date; mutable OpenAPI must be hashed when deterministic fixtures are generated |
| Live smoke state | Not run |

This profile is not implementation-ready. Promotion to `contract_verified` requires a versioned authoritative snapshot that establishes the accepted Responses stream event families, exact terminal/error transitions, required fields and usage placement, followed by deterministic fixtures. After promotion, downgrade on material OpenAPI, routing, stream, usage/cost, error, or cancellation changes, fixture failure, or review expiry.

## Sources

Official OpenRouter documentation retrieved 2026-07-26:

- [API overview](https://openrouter.ai/docs/api_reference/overview)
- [Authentication](https://openrouter.ai/docs/api_reference/authentication)
- [Streaming](https://openrouter.ai/docs/api_reference/streaming)
- [Errors](https://openrouter.ai/docs/api_reference/errors-and-debugging)
- [Limits](https://openrouter.ai/docs/api_reference/limits)
- [Usage accounting](https://openrouter.ai/docs/cookbook/administration/usage-accounting)
- [Embeddings](https://openrouter.ai/docs/api_reference/embeddings)
- [Responses](https://openrouter.ai/docs/api_reference/responses/overview)
- [Provider routing](https://openrouter.ai/docs/guides/routing/provider-selection)
- [OpenAPI](https://openrouter.ai/openapi.json)

The OpenAPI and model/pricing catalogs are mutable. Snapshot their retrieval time and content hash when fixtures are generated.

## Connection and Discovery

- Fixed base URL: `https://openrouter.ai/api/v1`; authenticate with `Authorization: Bearer <key>`.
- Optional attribution headers are not required and are disabled by default. Never forward client headers or enable debug echo, which can return model content.
- Endpoints: `GET /models`, `POST /chat/completions`, `POST /responses`, `POST /embeddings`, and embedding-specific discovery at `GET /embeddings/models`.
- Connection test/health probe: authenticated `GET /models`. It consumes request quota but has no documented inference charge. Capture only bounded model metadata.
- Model endpoint metadata, supported parameters, modalities, context length, endpoint availability, and pricing are mutable discovery snapshots. Capability routing uses their intersection with adapter support and administrator narrowing.

## Operations

### Chat

- The reviewed NexusRelay fields use OpenAI-compatible shapes. OpenRouter may silently ignore parameters unsupported by a selected upstream, so the named typed profile applies the code-owned `openrouter_require_parameters_v1` augmentation: every Chat request containing capability-gated fields receives fixed `provider.require_parameters: true` after NexusRelay capability filtering. Clients and administrators cannot supply or override the `provider` object, and no other OpenRouter routing field is passed through.
- NexusRelay supplies one explicit model ID. OpenRouter model fallback lists, auto routers, provider ordering, plugins, hosted tools, debug output, and arbitrary routing controls are outside V1.
- Tools, image input, JSON mode, strict structured output, penalties, sampling, and token limits remain model/endpoint capability-gated.
- OpenRouter normalizes finish reasons to `tool_calls`, `stop`, `length`, `content_filter`, or `error`; raw provider finish reasons are internal metadata only.

### Chat Streaming

- SSE `data:` events contain OpenAI-style chunks. Ignore SSE comments such as `: OPENROUTER PROCESSING`.
- Usage is always included exactly once in the final chunk, with an empty choices array, before `data: [DONE]`; legacy include-usage parameters have no effect.
- Mid-stream errors are a final chunk with top-level `error` and `choices[0].finish_reason: "error"`, followed by stream termination. Do not emit public `[DONE]` on failure.
- Capture `X-Generation-Id` as an upstream correlation ID.

### Responses and Embeddings

- `POST /responses` supports the stateless OpenAI-compatible subset. `store: true` and non-null `previous_response_id` are rejected; NexusRelay also excludes hosted tools and provider-specific reasoning controls.
- Responses streaming is documented as typed SSE, but the currently captured sources do not yet define a sufficiently exact accepted lifecycle for NexusRelay. The relationship among success events and `response.failed`, `response.error`, and `error`, required fields, terminal ordering, and final usage placement must be pinned before implementation; unknown events cannot be ignored or guessed.
- `POST /embeddings` accepts a string or array of strings for NexusRelay. Preserve `data[].index`; only float encoding and explicitly supported dimensions are exposed. Embeddings do not stream.

## Usage, Cost, Errors, and Cancellation

- OpenRouter reports native-tokenizer prompt/input, completion/output, total, cached, cache-write, reasoning and modality details when applicable, plus `cost` in account credits and optional cost details. Treat reported `cost` as authoritative charged cost after conversion through a versioned credit-to-USD policy; do not use binary floating point.
- If usage/cost is absent after interruption, reserve and reconcile conservatively. Prompt processing can be charged even when no content is generated.
- Parse bounded errors using HTTP status and stable `error.metadata.error_type`. Distinguish authentication, payment/quota, rate limit, provider overload/unavailable, validation, content policy, timeout, and server failures. Honor bounded `Retry-After` on 429/503.
- OpenRouter can transparently fallback before content starts. NexusRelay treats that as one upstream attempt/cost outcome and does not claim visibility into every internal provider attempt.
- Aborting a stream stops processing/billing only for OpenRouter's documented supported upstream providers. It does not work for non-stream requests or unsupported providers, so every cancellation remains potentially billable unless final usage proves otherwise.

## Verification Plan and Limitations

Promotion fixtures must cover auth/header construction, model and embedding-model discovery, exact augmentation emission/conflict rejection for `require_parameters`, Chat text/tools/images/schema, final usage and cost, SSE comments/usage/`[DONE]`, mid-stream errors, the fully pinned stateless Responses lifecycle, embeddings, all stable error types, retry hints, cancellation-supported and unsupported outcomes, redirects, size bounds, and redaction. Smoke tests use `OPENROUTER_API_KEY` and explicit Chat, Responses, and Embeddings model IDs.

Control-plane warnings: OpenRouter performs internal routing/fallback; model features/prices are mutable; cancellation support depends on the selected upstream; provider details and raw errors are not exposed; NexusRelay disables OpenRouter plugins, hosted tools, debug, automatic model selection, and arbitrary routing controls.
