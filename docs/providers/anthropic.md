# Anthropic Provider Profile

## Verification Record

| Field | Value |
| --- | --- |
| Provider type | `anthropic` |
| Intended adapter | Dedicated native Messages adapter using direct HTTP behind the provider-neutral adapter boundary |
| Status | `contract_verified` |
| Owner | NexusRelay maintainers |
| Reviewer | NexusRelay maintainers |
| Verified at | 2026-07-26 |
| Review due | 2026-10-26 |
| Documentation/API version | Claude API `v1` with required `anthropic-version: 2023-06-01` |
| Source identity | Official Anthropic documentation retrieved 2026-07-26; URLs and retrieval date form the reviewed source set because the rendered documentation does not publish immutable content hashes |
| Live smoke state | Not run; no credentials required for `contract_verified` |

Downgrade this profile to `research_in_progress` if Anthropic changes authentication, endpoint paths, required versioning, Messages content-block or stream grammar, token-usage billing semantics, or the review due date expires. Downgrade it to `profile_drafted` if a deterministic contract fixture fails until the difference is reviewed. A live smoke failure blocks release readiness but does not override deterministic evidence when the failure is conclusively account-, quota-, region-, or model-availability-specific.

## Sources

All sources are authoritative Anthropic documentation retrieved on 2026-07-26:

- [API overview, authentication, request IDs, pagination, and request limits](https://docs.anthropic.com/en/api/getting-started)
- [API versioning](https://docs.anthropic.com/en/api/versioning)
- [Create a Message](https://docs.anthropic.com/en/api/messages)
- [Streaming Messages](https://docs.anthropic.com/en/api/messages-streaming)
- [List models](https://docs.anthropic.com/en/api/models-list)
- [Count message tokens](https://docs.anthropic.com/en/api/messages-count-tokens)
- [API errors](https://docs.anthropic.com/en/api/errors)
- [Rate limits](https://docs.anthropic.com/en/api/rate-limits)
- [Tool use](https://docs.anthropic.com/en/docs/build-with-claude/tool-use/overview)
- [Structured outputs](https://docs.anthropic.com/en/docs/build-with-claude/structured-outputs)
- [Prompt caching](https://docs.anthropic.com/en/docs/build-with-claude/prompt-caching)
- [Vision](https://docs.anthropic.com/en/docs/build-with-claude/vision)
- [Stop reasons and fallback](https://docs.anthropic.com/en/docs/build-with-claude/handling-stop-reasons)
- [Token counting](https://docs.anthropic.com/en/docs/build-with-claude/token-counting)
- [Pricing](https://docs.anthropic.com/en/docs/about-claude/pricing)

Official SDK versions are intentionally not selected by this profile. NexusRelay uses direct HTTP for the adapter, and the separate public compatibility gate pins official OpenAI SDK versions and fixtures for the gateway-facing protocol.

## Connection

### Base URL and Endpoints

- Default base URL: `https://api.anthropic.com`.
- The native Anthropic provider does not accept an organization-configured base URL in V1. Anthropic-compatible cloud-platform endpoints require separately verified provider types because authentication, endpoint paths, limits, features, request IDs, and billing differ.
- Native endpoint paths are joined beneath the fixed host without path replacement:
  - `GET /v1/models`
  - `POST /v1/messages`
  - `POST /v1/messages/count_tokens`
- Messages and token-counting requests have an upstream 32 MB limit. NexusRelay applies its stricter configured public and normalized-content limits first.
- TLS certificate verification is mandatory. Redirects remain disabled under the common provider policy because the reviewed Claude API contract does not require redirects.
- The public-network SSRF policy applies. Private-network destinations are not valid for the native Anthropic profile.

### Authentication and Headers

- Send exactly one `x-api-key: <credential>` header for the V1 credential flow.
- Send `anthropic-version: 2023-06-01` on every request and `content-type: application/json` on JSON requests.
- Anthropic also documents bearer workload-identity tokens. NexusRelay does not implement that authentication flow in V1 and never sends both authentication methods.
- Do not send `anthropic-beta`. Beta-only user profiles, Files API references, server-side fallback, fine-grained tool streaming, managed agents, and other beta features are outside this profile.
- Do not forward client-supplied Anthropic authentication, version, beta, organization, or routing headers. Only validated provider configuration supplies upstream headers.
- Capture the `request-id` response header as the upstream request ID. Error bodies may repeat it as `request_id`; disagreement is retained only as sanitized diagnostic metadata. `anthropic-organization-id` is sensitive connection metadata and is neither logged nor exposed publicly.

### Connection Test and Health Probe

- Connection test operation: `GET /v1/models?limit=1` with the configured API key and version header.
- Success: HTTP 200 with a bounded object containing a `data` array and pagination fields. Each accepted model entry has `id`, `type: "model"`, `display_name`, `created_at`, limits, and capability metadata where returned.
- Authentication, permission, or billing failure is a configuration/account failure. HTTP 429, 500, 504, and 529 are provider-policy or availability failures, not invalid-credential proof.
- Model listing does not generate model output. It may consume request quota but has no documented token charge.
- Active health uses the same bounded operation with the common dedicated provider-probe timeout. It records latency, normalized status, upstream request ID, and safe rate-limit metadata without logging or auditing the model-list body.
- A successful model list does not prove entitlement to every listed feature or successful inference. Model-specific eligibility remains explicit.

## Model Discovery and Capabilities

- `GET /v1/models` lists models available to the credential, newest first. Pagination uses `limit` from 1 through 1000 and opaque `after_id` or `before_id` cursors; responses contain `data`, `first_id`, `last_id`, and `has_more`.
- Discovery snapshots store model ID, display name, release timestamp, reported input/output limits, returned capability flags, retrieval time, and source. Zero, absent, unknown, or newly added values are not treated as unlimited or supported.
- Current model records can include image input, structured output, thinking, citations, batch, code-execution, context-management, and effort capabilities. NexusRelay consumes only fields represented by the V1 normalized matrix and ignores unknown additions after bounded parsing.
- Effective capabilities are the intersection of adapter support, current discovered or reviewed model metadata, and administrator narrowing. Unknown capabilities fail closed before dispatch.
- Anthropic model aliases, availability, tokenizer generation, limits, feature support, quotas, and prices are mutable. They must not be hardcoded from this profile as permanent contract facts.
- The V1 adapter may advertise Chat Completions and Responses-equivalent text generation, streaming, client function tools, strict structured output, image input, stop sequences, temperature, top-p, and provider-reported usage only when the selected model supports each feature.
- Anthropic has no native OpenAI Embeddings equivalent in this profile. Anthropic targets are ineligible for `/v1/embeddings`.
- Audio, PDFs/documents, citations, prompt-cache controls, thinking/reasoning controls, prefills, server tools, built-in client tools, MCP, Files API references, batches, containers, managed agents, service tiers, inference geography, and arbitrary native fields are outside NexusRelay V1 even where Anthropic supports them.

## Operations

### Messages Translation

- Native endpoint: `POST /v1/messages`.
- The Messages API is stateless. Every public Chat or Responses request is translated into one complete native request; NexusRelay does not depend on provider-side response retrieval or conversation storage.
- `max_tokens` is required upstream. Public `max_completion_tokens`, Chat `max_tokens`, or Responses `max_output_tokens` normalize to this field. The selected value must be positive for normal inference, within gateway and discovered model limits, and never uses Anthropic's cache-prewarming value of zero.
- `temperature` maps directly only in Anthropic's documented inclusive range 0 through 1. `top_p` maps only after common validation; the provider documentation does not establish an additional transport bound that NexusRelay may invent.
- Public `stop` maps to `stop_sequences`. A returned match is available through `stop_sequence`.
- Frequency and presence penalties have no native Messages mapping and make an Anthropic target ineligible.
- Anthropic combines consecutive messages with the same `user` or `assistant` role. NexusRelay therefore canonicalizes public messages deterministically before dispatch and does not rely on provider-side combining to preserve message boundaries.

### Roles and Content

- Anthropic conversational turns use `user` and `assistant`. System instructions use top-level `system`, not an ordinary conversation role in the verified V1 subset.
- Leading system/developer instructions are concatenated in original order into bounded top-level system text with explicit separators. A system/developer message after conversational content begins cannot be represented without changing order or using newer provider-specific semantics, so that target is rejected before dispatch.
- Public user text maps to Anthropic user `text` blocks. Public assistant text maps to assistant `text` blocks.
- Public image input maps only in user turns to an Anthropic `image` block. Base64 data maps to `source.type: "base64"` with JPEG, PNG, GIF, or WebP media type. A validated remote URL maps to `source.type: "url"`; NexusRelay never fetches the URL. Files API IDs are not accepted.
- Public assistant function calls map to Anthropic assistant `tool_use` blocks with the same opaque call ID, function name, and parsed JSON object input. A public call whose arguments are not a complete JSON object cannot be replayed to Anthropic and is rejected before dispatch.
- Public tool results map to `tool_result` blocks in the next Anthropic `user` turn and correlate through `tool_use_id`. The initial V1 mapping sends bounded string content and `is_error` only when the normalized contract carries an error result.
- Anthropic requires tool results immediately after their corresponding tool calls. The adapter rejects orphaned, duplicate, reordered, or interleaved tool results rather than silently changing conversation semantics.
- Native `thinking`, `redacted_thinking`, `server_tool_use`, server-tool result, fallback, document, search-result, citation-only, container, or unknown content blocks have no V1 public representation. An unexpected block before response commitment fails the attempt as an unsupported upstream contract; after commitment it terminates the stream with a sanitized error and no success terminal.

### Chat Completions Mapping

- Public Chat requests translate to one native Message. `n` must be 1 as required by the public matrix; Anthropic returns one assistant message.
- Non-stream output maps ordered native `text` and `tool_use` blocks into one Chat assistant message. Tool inputs are serialized as compact JSON argument strings without logging or semantic reinterpretation.
- Native `end_turn` and `stop_sequence` map to Chat `finish_reason: "stop"`; `max_tokens` and `model_context_window_exceeded` map to `length`; `tool_use` maps to `tool_calls`.
- Native `pause_turn` is outside the V1 server-tool-free subset and is an unsupported upstream outcome. Native `refusal` maps to the normalized refusal outcome rather than successful schema output or a transport error.
- The public renderer uses the gateway model alias and gateway-generated completion ID, not Anthropic's model ID or `msg_` identifier.

### Responses Mapping

- Anthropic has no separate native Responses endpoint. NexusRelay translates the stateless V1 Responses subset onto `POST /v1/messages`.
- `instructions` and leading developer/system input map to top-level `system`. Structured input maps only normalized text, image, function-call, and function-call-output forms.
- Native text blocks map to output-message text items. Native `tool_use` blocks map to function-call output items, retaining the native tool-use ID as the normalized call ID.
- Provider state IDs are not exposed as durable gateway retrieval handles. Stored responses, previous-response state, conversations, background mode, and built-in tools remain rejected.
- Native `max_tokens` or `model_context_window_exceeded` produces an incomplete length outcome. `refusal` produces a refusal outcome. `pause_turn` is unsupported.

### Client Function Tools

- Public function tools map to native `tools[].{name,description,input_schema}`. Only client-defined custom tools are used; dated Anthropic-provided client tools and server tools are rejected.
- Public `tool_choice` maps as follows: `none` to `{type:"none"}`, `auto` to `{type:"auto"}`, `required` to `{type:"any"}`, and a named function to `{type:"tool",name:<name>}`.
- Public `parallel_tool_calls: false` maps to `disable_parallel_tool_use: true`; `true` maps to false or omission. Under `auto`, disabling parallel calls permits at most one call; under required or named choice it requires exactly one.
- Strict function tools set `strict: true` and must pass the Structured Schema Constraints below. Non-strict tools still use bounded schemas but do not claim guaranteed provider validation.
- The response `tool_use` block contains `id`, `name`, and an input object. Multiple blocks are preserved in content order.

### Structured Output

- Strict public JSON-schema output maps to `output_config.format` with `type: "json_schema"` and the provider schema. Anthropic has no transport field corresponding to the OpenAI schema `name`; it remains gateway metadata.
- Anthropic returns valid JSON as a string in a normal `text` content block on ordinary successful completion. The adapter does not synthesize a distinct native JSON content type.
- JSON output and strict tools may be combined. A turn may emit tool calls before a later turn emits final JSON, so tool-use output is not a schema violation.
- NexusRelay routes only strict JSON-schema output to Anthropic. Chat `json_object` without a schema is unsupported because the provider contract does not expose equivalent schema-free JSON mode semantics.
- Safety refusal and token/context truncation override the schema guarantee. Refusal text or incomplete JSON is never reported as a successful structured result.

### Streaming

- Set `stream: true`. Anthropic returns named SSE events whose event name matches the JSON `type`.
- The verified grammar is:
  - one `message_start` containing a Message with empty `content`;
  - for each ordered content index, `content_block_start`, zero or more `content_block_delta`, then `content_block_stop`;
  - one or more `message_delta` events carrying top-level changes and cumulative usage;
  - one final `message_stop`.
- Any number of `ping` events may appear and are ignored after bounded validation. Anthropic may add event types under its versioning policy; unknown events are ignored only when they do not violate active block/message state or contain required terminal semantics.
- There is no `data: [DONE]` marker for `anthropic-version: 2023-06-01`. NexusRelay emits its own public terminal event only after durable finalization.
- Text uses `content_block_delta.delta.type: "text_delta"`. Tool input uses `input_json_delta` with a partial JSON string. Fragments are appended verbatim by content index and parsed once at `content_block_stop`; they are never parsed for logging.
- Each `tool_use` block starts with its ID, name, and empty input object. The adapter correlates blocks by content index and preserves a stable public tool-call index and ID.
- `message_delta.usage` is cumulative, not incremental. The adapter replaces prior observed values rather than adding them.
- A provider `error` SSE event can arrive after HTTP 200, including `overloaded_error`. It terminates the attempt as failed. NexusRelay never emits a success finish reason or `[DONE]` after a provider stream error or failed durable finalization.
- Structured JSON text is forwarded incrementally as text but parsed or schema-validated only after the full block and successful terminal reason. Partial deltas need not be valid JSON.
- The V1 adapter does not enable thinking or fine-grained/eager tool streaming. `thinking_delta`, `signature_delta`, and unsupported block types fail closed if unexpectedly returned.

## Structured Schema Constraints

Strict JSON output and strict function tools use the same verified Anthropic subset. Validate it before routing:

- Supported types are object, array, string, integer, number, boolean, and null.
- Every object sets `additionalProperties: false`; open-ended map objects are unsupported.
- Supported composition includes `anyOf` and non-reference `allOf`. Internal `$ref`, `$defs`, and `definitions` are supported; external and recursive references and `allOf` involving `$ref` are rejected.
- `enum` values are limited to strings, numbers, booleans, or null. `const`, `default`, and `required` are supported.
- Supported string formats are `date-time`, `time`, `date`, `duration`, `email`, `hostname`, `uri`, `ipv4`, `ipv6`, and `uuid`.
- `minItems` is accepted only as 0 or 1. Other array-size constraints, numeric constraints, `minLength`, `maxLength`, and unsupported composition keywords are rejected.
- Regex patterns are limited to the provider's simple supported subset. Backreferences, lookaround, word boundaries, and complex large-range quantifiers are rejected.
- NexusRelay never strips unsupported constraints or moves them into descriptions because that would silently weaken public strict semantics.
- Provider schema-complexity limits, strict-tool counts, optional/union parameter counts, compilation timeout, cache lifetime, and model availability are mutable capability data. NexusRelay applies equal or stricter public limits and treats provider `schema too complex` responses as target validation failures, not transient outages.
- Do not place secrets, tenant identifiers, user data, or model content in schema names, property names, descriptions, enums, constants, or patterns; Anthropic documents compiled-schema caching.

## Usage and Cost

### Token Usage and Counting

- Core Messages usage fields are `input_tokens` and `output_tokens`.
- Prompt caching can additionally report `cache_creation_input_tokens`, `cache_read_input_tokens`, and a cache-creation TTL breakdown. Unlike providers where cached tokens are a subset of input tokens, Anthropic total logical input is:

```text
input_tokens + cache_creation_input_tokens + cache_read_input_tokens
```

- Normalize these as non-overlapping uncached input, cache-write input, and cache-read input dimensions. Do not infer omitted dimensions as zero unless the field's versioned contract establishes that behavior.
- Output-token details may identify thinking tokens, but `output_tokens` is the inclusive billed total. Reasoning details are not added again and reasoning controls are outside V1.
- Streaming `message_delta` usage is cumulative. Interrupted streams can lack final usage and require conservative accounting.
- `POST /v1/messages/count_tokens` accepts the corresponding message, system, tool, image, and structured-output inputs and returns `input_tokens`. It is free but separately rate-limited.
- Anthropic explicitly calls token-count results estimates; actual billed input can differ slightly, automatically added system tokens are not billed, and tokenizer behavior is model-dependent and mutable. Token counting is useful for admission estimates but never overrides provider-reported final usage.

### Pricing and Hard-Budget Policy

- Messages responses do not provide authoritative per-request monetary cost. NexusRelay computes cost from a versioned Anthropic price record effective at request time.
- Pricing is denominated in USD and mutable by model, input, cache-write TTL, cache read, output, platform, region, and provider feature. Current numeric prices are not copied into this contract profile.
- The V1 adapter does not expose prompt-cache controls or server tools, but usage can still contain newly added categories. A route is ineligible under a hard monetary budget unless the effective price record covers every possible billable category for the selected model and request mode.
- Pre-dispatch reservation uses the token-count estimate when available plus bounded output maximums and all applicable price categories. Because token counting is approximate, reservation includes the common configured estimation margin and never assumes exact equality.
- If provider usage is unavailable after dispatch, reconcile with the greater of locally estimated actual usage and the pre-dispatch conservative reservation, bounded by request/model maximums. Never reconcile a potentially billable attempt to implicit zero.
- Anthropic does not provide a universal guarantee that failed, timed-out, disconnected, cancelled, refused, or truncated requests are free. Once dispatch may have reached Anthropic, the attempt is potentially billable until provider usage or later billing evidence proves otherwise.

## Errors, Limits, and Retry Safety

### Response Metadata

- Capture `request-id` from every response when available.
- Parse `retry-after` as bounded seconds and these rate-limit header families as advisory metadata:
  - `anthropic-ratelimit-requests-{limit,remaining,reset}`
  - `anthropic-ratelimit-tokens-{limit,remaining,reset}`
  - `anthropic-ratelimit-input-tokens-{limit,remaining,reset}`
  - `anthropic-ratelimit-output-tokens-{limit,remaining,reset}`
- Reset timestamps are RFC 3339. Token remaining values may be rounded. Limits use token buckets, can apply over shorter intervals, vary by organization/workspace/model, and do not authorize bypassing NexusRelay policy.

### Normalization

| Upstream condition | Normalized category | Retry/fallback policy before commitment |
| --- | --- | --- |
| HTTP 400 `invalid_request_error` | `invalid_request` or `unsupported_model_capability` according to the safe reviewed cause | Never retry the same incompatible request; target-specific fallback is allowed only when semantic compatibility was proven before dispatch |
| HTTP 401 `authentication_error` | `provider_unavailable` with provider-authentication reason | Do not retry the same connection; another independently configured target may be attempted |
| HTTP 402 `billing_error` | `provider_unavailable` with provider-billing reason | Do not hot-retry the same connection; fallback may use another target |
| HTTP 403 `permission_error` | `provider_unavailable` with provider-permission reason | Do not retry the same connection; fallback may use another target |
| HTTP 404 `not_found_error` | `model_not_found` for a configured upstream model, otherwise sanitized `upstream_error` | Do not retry the same target; fallback may use another target |
| HTTP 409 `conflict_error` | `upstream_error` | Retry only when the operation is idempotent and no response was committed |
| HTTP 413 `request_too_large` | `invalid_request` | Never retry without changing the request |
| HTTP 429 `rate_limit_error` | `provider_rate_limited`, unless account spend/quota evidence establishes provider unavailability | Bounded jittered fallback/backoff; honor trustworthy bounded `retry-after` |
| HTTP 500 `api_error` | `provider_unavailable` | Bounded fallback/backoff before commitment only |
| HTTP 504 `timeout_error` | `request_timeout` with ambiguous upstream outcome | Bounded non-stream fallback only under the common attempt policy and conservative billing |
| HTTP 529 `overloaded_error` | `provider_unavailable` | Bounded fallback/backoff before commitment only |
| Other 4xx/5xx or malformed bounded error body | `invalid_request`, `provider_unavailable`, or `upstream_error` according to status and safe metadata | Retry only under the configured safe status set and remaining deadline |
| DNS, connect, TLS, or pre-header timeout | `provider_unavailable` or `request_timeout` | Bounded fallback before commitment |
| Timeout/disconnect after request transmission | `request_timeout` with ambiguous upstream outcome | Never retry streaming after commitment; non-stream fallback remains bounded and conservatively billed |

- Error JSON has top-level `type: "error"`, nested `error.type` and `error.message`, and `request_id`. Type values may expand under the versioning policy.
- Sanitize every public error. Never return or log raw Anthropic bodies, API keys, organization IDs, model content, refusal explanations, tool arguments, or schema data.
- Read at most the common configured bounded error-body size, retain only allowlisted error type and request ID, and discard the raw body.
- Upstream authentication, billing, and permission failures never become public NexusRelay `authentication_error` or `permission_denied`, which are reserved for gateway-key and gateway-policy failures.
- Official SDK retry behavior is not copied. NexusRelay owns retries and keeps them bounded by count, deadline, commitment state, idempotency, and conservative billing.

### Refusal and Filtering

- A successful Message can stop with `stop_reason: "refusal"` and optional `stop_details` containing a machine-readable category and unstable human explanation. This is model refusal output, not an HTTP transport error.
- Preserve refusal state without logging generated refusal text or explanation. For Chat/Responses rendering, use the normalized refusal path defined by the public protocol; do not claim structured-schema success.
- Refusal tokens are billable according to Anthropic's structured-output documentation. Do not automatically retry a refusal on the same or another provider unless a future explicit routing policy defines safe behavior.
- Anthropic's API may also reject prohibited image or request content with an HTTP error. Transport errors and successful model refusals remain distinct.

## Cancellation

- Every HTTP request is bound to its context. Client disconnect, gateway timeout, or routing cancellation closes the request body or response stream and propagates cancellation to Anthropic.
- Anthropic does not guarantee that disconnecting a synchronous HTTP request prevents processing or billing. After dispatch, cancellation remains an ambiguous potentially billable outcome unless final usage is received.
- No provider switch occurs after response commitment. Anthropic documents model-specific continuation strategies for interrupted streams, but NexusRelay does not issue continuation requests because they can duplicate or semantically alter public output.

## Verification Plan

The adapter is not release-supported until deterministic local mock-server tests implement these scenarios. Fixtures belong under `internal/providers/anthropic/testdata/` when the Go module is scaffolded:

- `models/list.json`: valid capability/limit discovery, forward pagination, empty page, unknown capability, zero/absent limits, malformed response, and oversized response.
- `messages/text.json`: non-stream text, request ID, gateway alias replacement, stop reasons, and core/cache usage dimensions.
- `messages/images_request.json`: bounded base64 and URL image translation without gateway URL retrieval.
- `messages/tools.json`: one and parallel client tool calls, choice mapping, tool-result replay, JSON argument serialization, and invalid call/result ordering.
- `messages/structured.json`: strict JSON text, strict tool input, combined tools/output format, unsupported schema rejection, refusal, max-token truncation, and schema-complexity 400.
- `messages/refusal.json`: HTTP-success refusal with nullable/unknown stop details and no structured-success claim.
- `stream/text.sse`: message start, ping, text block lifecycle, cumulative usage delta, message stop, and no upstream `[DONE]`.
- `stream/tools.sse`: multiple indexed tool blocks with partial JSON fragments, block completion parsing, and tool-use finish.
- `stream/structured.sse`: incremental JSON text that is invalid until completion and terminal validation.
- `stream/error.sse`: post-200 overloaded error and other error event without success terminal.
- `stream/truncated.sse`: missing block stop/message stop, malformed JSON event, mismatched event name/type, unknown harmless event, unknown state-changing event, oversized event, idle timeout, cancellation, and missing final usage.
- `count_tokens/basic.json`: translated request, estimated input count, malformed response, rate limit, and confirmation that count usage remains estimated.
- `errors/*.json`: 400, 401, 402, 403, 404, 409, 413, 429, 500, 504, 529, malformed body, oversized body, request-ID mismatch, and trustworthy/untrustworthy retry hints.
- Transport tests: required header construction, beta-header absence, upstream request-ID capture, redirect rejection, TLS failure, cancellation, deadline, body closure, response-size bounds, and redaction sentinels.
- Capability tests: no embeddings, no mid-conversation system role, no penalties, no JSON-object mode, no audio/PDF/citations/thinking/server tools/Files API, and fail-closed behavior for unknown native blocks.

Optional live smoke tests use:

- `ANTHROPIC_API_KEY` required.
- Explicit model environment variables selected by the operator for text, tools, image input, and structured output; tests do not assume a globally available model ID.

The smoke suite lists models, counts a minimal prompt, performs minimal non-stream and stream calls for explicitly configured capabilities, checks request IDs and usage when returned, and sends no production or user content. It is opt-in, never part of default CI, and never records credentials, prompts, outputs, tool arguments, refusal text, or schema content.

## Control-Plane Limitations

Display these limitations for Anthropic connections and models:

- The native adapter uses the direct Claude API and fixed `anthropic-version: 2023-06-01`; Bedrock, Vertex AI, Claude Platform on AWS, and Microsoft Foundry require separately verified providers.
- Model listing and returned capability metadata are mutable and do not replace administrator narrowing or current pricing records.
- Anthropic provides native Messages, not OpenAI Chat, Responses, or Embeddings; NexusRelay translates the supported stateless text-generation subset and does not offer Anthropic embeddings.
- System/developer instructions are supported only before conversational turns. Frequency and presence penalties, schema-free JSON mode, and mid-conversation system instructions are unsupported.
- Only client-defined function tools are supported. Anthropic server tools, built-in client tools, MCP, containers, managed agents, and Files API dependencies are unavailable.
- Image input supports bounded base64 and pass-through URLs. NexusRelay does not fetch client URLs or accept Anthropic file IDs.
- Prompt-cache controls, reasoning/thinking controls, citations, PDF/document input, service tiers, and inference geography are not exposed in V1.
- Usage may be unavailable on interrupted streams or ambiguous failures; NexusRelay may apply conservative estimated accounting. Token-count results are estimates, not final billed usage.
