# OpenAI Provider Profile

## Verification Record

| Field | Value |
| --- | --- |
| Provider type | `openai` |
| Intended adapter | Native OpenAI baseline using direct HTTP behind the provider-neutral adapter boundary |
| Status | `contract_verified` |
| Owner | NexusRelay maintainers |
| Reviewer | NexusRelay maintainers |
| Verified at | 2026-07-26 |
| Review due | 2026-10-26 |
| Documentation/API version | REST API `v1`; response header currently reports `openai-version: 2020-10-01` |
| Source identity | Official OpenAI developer documentation retrieved 2026-07-26; URLs and retrieval date form the reviewed source set because the rendered documentation does not publish immutable content hashes |
| Live smoke state | Not run; no credentials required for `contract_verified` |

Downgrade this profile to `research_in_progress` if OpenAI announces a breaking `v1` transport change, changes authentication or endpoint paths, materially changes Chat or Responses stream terminal behavior, changes usage-field billing semantics, or the review due date expires. Downgrade it to `profile_drafted` if a deterministic contract fixture or pinned official-SDK compatibility fixture fails until the difference is reviewed. A live smoke failure blocks release readiness but does not override deterministic evidence when the failure is conclusively account-, quota-, or model-availability-specific.

## Sources

All sources are authoritative OpenAI documentation retrieved on 2026-07-26:

- [API overview, authentication, request IDs, headers, and compatibility](https://developers.openai.com/api/reference/overview)
- [List models](https://developers.openai.com/api/reference/resources/models/methods/list)
- [Create Chat Completion](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create)
- [Chat Completions streaming events](https://developers.openai.com/api/reference/resources/chat/subresources/completions/streaming-events)
- [Create a Response](https://developers.openai.com/api/reference/resources/responses/methods/create)
- [Responses streaming events](https://developers.openai.com/api/reference/resources/responses/streaming-events)
- [Create embeddings](https://developers.openai.com/api/reference/resources/embeddings/methods/create)
- [Streaming guide](https://developers.openai.com/api/docs/guides/streaming-responses)
- [Error codes](https://developers.openai.com/api/docs/guides/error-codes)
- [Rate limits](https://developers.openai.com/api/docs/guides/rate-limits)
- [Function calling](https://developers.openai.com/api/docs/guides/function-calling)
- [Structured Outputs](https://developers.openai.com/api/docs/guides/structured-outputs)
- [Images and vision](https://developers.openai.com/api/docs/guides/images-vision)
- [Prompt caching](https://developers.openai.com/api/docs/guides/prompt-caching)
- [Pricing](https://developers.openai.com/api/docs/pricing)

Official SDK versions are intentionally not selected by this provider profile. The separate public compatibility baseline pins them and the committed goldens in [`docs/testing/openai-sdk-compatibility.md`](../testing/openai-sdk-compatibility.md).

## Connection

### Base URL and Endpoints

- Default base URL: `https://api.openai.com/v1`.
- The native OpenAI provider does not accept an organization-configured base URL in V1. Alternative endpoints use the custom OpenAI-compatible provider type and its SSRF policy.
- Endpoint paths are joined beneath the fixed `/v1` prefix without path replacement:
  - `GET /models`
  - `POST /chat/completions`
  - `POST /responses`
  - `POST /embeddings`
- TLS certificate verification is mandatory. Redirects remain disabled under the common provider policy because the reviewed OpenAI contract does not require redirects.
- The public-network SSRF policy applies. Private-network destinations are not valid for the native OpenAI profile.

### Authentication and Routing Headers

- Send exactly one `Authorization: Bearer <credential>` header.
- A standard API key is the V1 credential. OpenAI also documents short-lived workload identity tokens, but NexusRelay does not implement that authentication flow in V1.
- `OpenAI-Organization` and `OpenAI-Project` are optional non-secret connection fields used when the credential can access multiple organizations or projects. Usage is attributed by OpenAI to the selected organization/project.
- Do not forward client-supplied OpenAI organization, project, authorization, or routing headers. Only validated provider-connection configuration supplies them.
- Send a NexusRelay-generated upstream correlation value through `X-Client-Request-Id`. OpenAI limits it to ASCII and 512 characters. It is distinct from the public gateway request ID and may be retained as safe attempt metadata.
- Capture `x-request-id` as the upstream request ID. Safe operational metadata may also capture `openai-processing-ms`, `openai-version`, and `openai-organization`; never expose the upstream request ID as the gateway request ID.

### Connection Test and Health Probe

- Connection test operation: `GET /v1/models` with the configured credential and optional organization/project headers.
- Success: HTTP 200 with `object: "list"` and a bounded `data` array whose entries include `id`, `object: "model"`, `created`, and `owned_by`.
- Authentication/permission failure is a configuration failure. HTTP 429 quota/rate-limit and retryable 5xx responses are provider-policy/availability failures, not invalid-credential proof.
- The operation is documented as model listing and does not generate model output. It may consume request quota but has no documented token charge.
- Active health uses the same operation with the validated dedicated provider-test/probe timeout from the common worker configuration and no OpenAI-specific override. It runs at low frequency and records latency, normalized status, upstream request ID, and rate-limit metadata without storing the model list body in logs or audit records.
- A successful list does not prove inference entitlement or model capability. Model-specific eligibility remains explicit.

## Model Discovery and Capabilities

- `GET /v1/models` is authoritative for model identifiers available to the credential at retrieval time and returns only basic identity metadata. It does not provide a complete machine-readable operation/capability/limit matrix.
- Discovery snapshots store model ID, `created`, `owned_by`, retrieval time, and source. They do not automatically assert Chat, Responses, Embeddings, image, tool, structured-output, context-window, or output-limit capability.
- Effective model capabilities are the intersection of adapter support, reviewed model metadata or operator configuration, and administrator narrowing. Unknown capabilities fail closed before dispatch.
- OpenAI model availability, aliases, model-specific limits, and feature support are mutable. They must not be hardcoded from this profile as permanent contract facts.
- Text input/output, image input, function tools, parallel function calls, JSON mode, strict Structured Outputs, reasoning usage, and embedding dimensions are model-dependent capability gates.
- Audio input/output, image generation, built-in tools, custom tools, hosted tools, background mode, stored responses, conversations, previous-response state, and provider-specific reasoning controls are outside the NexusRelay V1 public matrix even where OpenAI supports them.

## Operations

### Chat Completions

- Native endpoint: `POST /v1/chat/completions`.
- NexusRelay forwards only fields accepted by `docs/design/13-api-compatibility-matrix.md` and only after model capability checks. It does not pass through unknown OpenAI fields.
- Message roles in the normalized subset map to `developer`, `system`, `user`, `assistant`, and `tool`. Tool results use `role: "tool"`, the returned `tool_call_id`, and string content.
- Text parts map to Chat text content. Image inputs map to `type: "image_url"` with either a fully qualified URL or bounded base64 data URL; support and tokenization are model-dependent. NexusRelay does not fetch client image URLs.
- Function tools use Chat's nested shape: `tools[].type: "function"` and `tools[].function.{name,description,parameters,strict}`.
- `tool_choice` maps only `none`, `auto`, `required`, or a named function. `parallel_tool_calls` is forwarded only for models that explicitly support it.
- JSON mode maps to `response_format.type: "json_object"`. Strict structured output maps to `response_format.type: "json_schema"` with `json_schema.{name,schema,strict}` and requires a compatible model.
- Sampling and length fields accepted by the public matrix map to their same-named OpenAI fields: `temperature`, `top_p`, `max_completion_tokens`, `stop`, `frequency_penalty`, and `presence_penalty`. The compatibility alias `max_tokens` is normalized to one upstream token-limit field rather than sending both. Every field remains model-capability-gated because OpenAI documents model-specific support.
- Non-stream responses map `id`, `object: "chat.completion"`, `created`, `model`, `choices[].index`, assistant content/refusal/tool calls, finish reason, and usage. The public renderer replaces the upstream model with the gateway alias and does not expose upstream-only metadata.
- Function calls carry an opaque call `id`, `type: "function"`, function `name`, and JSON-encoded `arguments` string. Arguments are untrusted model output and are never logged, persisted, or assumed valid JSON.
- Finish reasons map from `stop`, `length`, `tool_calls`, `content_filter`, and deprecated `function_call`. Refusal text and `content_filter` are distinct provider semantics and must not be collapsed without endpoint-specific normalization.

#### Chat Streaming

- Set `stream: true`; OpenAI returns server-sent events containing `chat.completion.chunk` JSON objects.
- Preserve a stable completion ID, creation time, model, and choice index while translating ordered role, content, refusal, and tool-call deltas.
- Tool-call deltas are correlated by `index`; `id`, type, name, and argument fragments may be absent from individual deltas. Append argument fragments verbatim and never parse them for logging.
- A finish-reason chunk follows output deltas. With `stream_options.include_usage: true`, ordinary chunks carry `usage: null`, followed by a final chunk with `choices: []` and whole-request usage.
- The stream terminator is `data: [DONE]`. An interrupted or cancelled stream may omit the final usage chunk and terminator.
- NexusRelay sends its own `[DONE]` only after durable finalization as required by `docs/design/05-inference-protocol.md`; it must not blindly relay the upstream terminator before accounting completes.

### Responses

- Native endpoint: `POST /v1/responses`.
- NexusRelay uses the stateless subset: `store: false`, no conversation, no `previous_response_id`, no background mode, and no retrieval/cancel lifecycle dependency.
- String input maps to a user input message. Structured input maps only text, image, function-call, and function-call-output forms defined by the public matrix.
- Instructions map to `instructions`; output limit maps to `max_output_tokens`.
- Function tools use the flat Responses shape `tools[].{type:"function",name,description,parameters,strict}`. Function calls return output items with `type: "function_call"`, item `id`, logical `call_id`, `name`, and JSON-encoded `arguments`. Tool results reference `call_id`, not the output-item ID.
- Strict structured text maps to `text.format.{type:"json_schema",name,schema,strict}`; JSON mode maps to `text.format.type: "json_object"`.
- Non-stream responses map only normalized text messages, refusals, function calls, status/incomplete reason, and provider-reported usage. Provider state IDs are not offered as durable gateway retrieval handles.

#### Responses Streaming

- Set `stream: true`; OpenAI returns typed server-sent event JSON objects. Responses does not document a Chat-style `[DONE]` marker.
- The V1 adapter accepts and normalizes these relevant event families:
  - `response.created`, `response.in_progress`
  - `response.output_item.added`, `response.output_item.done`
  - `response.content_part.added`, `response.content_part.done`
  - `response.output_text.delta`, `response.output_text.done`
  - `response.refusal.delta`, `response.refusal.done`
  - `response.function_call_arguments.delta`, `response.function_call_arguments.done`
  - `response.completed`, `response.incomplete`, `response.failed`, and `error`
- Text and function argument fragments are correlated by response/item identifiers, `output_index`, and `content_index` where present. Complete values from `*.done` events are consistency checks, not additional deltas.
- The provider documents a numeric `sequence_number` on events but does not establish a reliable starting value or contiguity rule. The adapter preserves provider order and validates local state transitions; it does not reject a stream solely because sequence numbers do not begin at a chosen value.
- Usage is carried in the response object, normally populated on `response.completed`; created/in-progress examples use `usage: null`. Failed or incomplete responses may lack usage and therefore require conservative accounting.
- `response.completed`, `response.incomplete`, and `response.failed` are terminal response states. A standalone `error` event closes the normalized stream as failed. NexusRelay does not synthesize a success terminal event after an error or failed durable finalization.

### Embeddings

- Native endpoint: `POST /v1/embeddings`.
- V1 sends a string or bounded array of strings. Token-ID inputs are rejected by the public matrix even though OpenAI accepts them.
- `encoding_format: "float"` is supported. `base64` remains capability-gated until exact byte encoding and official SDK fixtures are pinned.
- `dimensions` is forwarded only for a model explicitly known to support dimension control.
- Preserve input order using response `data[].index`; each item has `object: "embedding"` and an embedding vector. The public renderer uses the gateway model alias.
- Provider usage fields are `prompt_tokens` and `total_tokens`; normalized output tokens are unavailable for embeddings.
- Current OpenAI documentation states per-input and aggregate bounds, but these are mutable model/service constraints. The adapter enforces NexusRelay bounds first and classifies stricter upstream validation without treating the documented current numbers as permanent transport constants.

## Structured Schema Constraints

For strict function tools and strict text output, enforce the supported OpenAI JSON Schema subset before dispatch:

- Root type is `object`; root-level `anyOf` is rejected.
- Every property is listed in `required`; optional values are represented with a nullable type.
- Every object sets `additionalProperties: false`.
- Supported schema features and provider limits are capability/version data. At minimum, the deterministic contracts cover nested objects/arrays/enums and reject unsupported composition keywords such as `allOf`, `not`, conditional schemas, and dependent schemas.
- Provider-documented aggregate limits include nesting, property, enum, and schema-character bounds. NexusRelay applies equal or stricter bounded public limits and does not silently weaken `strict: true`.
- A safety refusal or incomplete/max-token response is not a schema-valid result and is normalized separately.

## Usage and Cost

### Token Usage

- Chat reports `prompt_tokens`, `completion_tokens`, and `total_tokens` when usage is present.
- Chat prompt details may report `cached_tokens` and `cache_write_tokens`; completion details may report `reasoning_tokens` and other provider-specific dimensions. Responses uses `input_tokens`, `output_tokens`, `total_tokens`, `input_tokens_details`, and `output_tokens_details` with corresponding cached/reasoning categories.
- Embeddings reports `prompt_tokens` and `total_tokens`.
- Each normalized dimension records provider-reported, estimated, or unavailable independently. Do not infer a missing detail as zero.
- Cached input tokens are a subset of input/prompt tokens, and reasoning tokens are a subset of output/completion tokens. Pricing must use non-overlapping categories: uncached input equals total input minus cached reads minus separately billed cache writes where the applicable price record requires that distinction; output totals already include reasoning tokens unless a verified future price record says otherwise.
- OpenAI states prompt caching does not reduce TPM consumption. Cache reads and, for applicable models, cache writes can have distinct prices.
- Provider-reported token counts are authoritative. OpenAI does not expose one tokenizer contract through model listing; any local estimate must use a versioned model-aware tokenizer mapping, mark every estimated dimension, and fall back to conservative bounds when the upstream model/tokenizer relationship is unknown.

### Pricing and Hard-Budget Policy

- The inference response does not provide authoritative per-request monetary cost for this V1 profile. NexusRelay computes cost from a versioned OpenAI price record effective at request time.
- Pricing is denominated in USD and is mutable by model, processing tier, context tier, modality, and token category. Current prices are not copied into this contract profile.
- A route is ineligible under a hard monetary budget unless NexusRelay has a reviewed effective price record covering every possible billable category for the selected model and request mode.
- If provider usage is unavailable after dispatch, reconcile with the greater of locally estimated actual usage and the pre-dispatch conservative reservation, bounded by the request/model maximums. Never reconcile a potentially billable attempt to implicit zero.
- OpenAI documentation does not provide a universal guarantee that failed, timed-out, disconnected, or cancelled inference is free. Once dispatch may have reached OpenAI, NexusRelay treats the attempt as potentially billable until provider usage or later billing evidence proves otherwise.

## Errors, Limits, and Retry Safety

### Response Metadata

- Capture `x-request-id` from every response when available.
- Parse documented request/token rate-limit headers and optional project-token variants as bounded advisory metadata:
  - `x-ratelimit-limit-requests`, `x-ratelimit-remaining-requests`, `x-ratelimit-reset-requests`
  - `x-ratelimit-limit-tokens`, `x-ratelimit-remaining-tokens`, `x-ratelimit-reset-tokens`
  - `x-ratelimit-limit-project-tokens`, `x-ratelimit-remaining-project-tokens`, `x-ratelimit-reset-project-tokens`
- Limits are organization/project/model dependent and mutable. Headers do not authorize bypassing NexusRelay policy.

### Normalization

| Upstream condition | Normalized category | Retry/fallback policy before commitment |
| --- | --- | --- |
| HTTP 400 malformed/unsupported request | `invalid_request` | Never retry another provider for the same incompatible request unless the router had already proven semantic compatibility and the failure is a provider-specific target constraint |
| HTTP 401 invalid/revoked/mismatched provider credential | `provider_unavailable` with provider-authentication reason | Do not retry the same connection; another independently configured target may be attempted |
| HTTP 403 provider permission, unsupported region, or IP restriction | `provider_unavailable` with a safe provider-permission/region reason | Do not retry the same connection; fallback may use another eligible target |
| HTTP 404 model/resource not found | `model_not_found` for configured upstream model | Do not retry the same target; fallback may use another target |
| HTTP 409 conflict | `upstream_error` | Retry only when the operation is idempotent and no response was committed |
| HTTP 422 unprocessable request | `invalid_request` | Do not retry unless a target-specific constraint was already modeled as fallback-safe |
| HTTP 429 request/token rate limit | `provider_rate_limited` | Bounded jittered fallback/backoff may apply; honor a trustworthy bounded `Retry-After` or reset hint |
| HTTP 429 exhausted quota/spend limit | `provider_unavailable` with provider-quota reason | Do not hot-retry the same connection; fallback may use another provider connection |
| HTTP 500 or retryable 503 overload/slow-down | `provider_unavailable` | Bounded fallback/backoff before commitment only |
| Other 5xx or malformed bounded error body | `upstream_error` | Retry only under the configured safe status set and remaining deadline |
| DNS, connect, TLS, or pre-header timeout | `provider_unavailable` or `request_timeout` | Bounded fallback before commitment |
| Timeout/disconnect after request transmission | `request_timeout` with ambiguous upstream outcome | Never retry a streaming request after commitment; non-stream fallback requires the common bounded-attempt policy and conservative billing |

- Sanitize every public error. Never return or log the raw OpenAI body, authorization value, organization/project identifiers, model content, or tool arguments.
- Read at most the common configured bounded error-body size, parse the OpenAI error envelope when possible, capture only allowlisted code/type/param metadata, and discard the raw body.
- Upstream credential and permission failures never become public NexusRelay `authentication_error` or `permission_denied` responses, which are reserved for gateway-key and gateway-policy failures.
- Distinguish provider request rate limiting from provider quota/spend exhaustion even though both may use HTTP 429.
- OpenAI recommends exponential backoff and notes failed retries consume rate-limit capacity. NexusRelay retries remain bounded by count and total deadline.

### Filtering and Refusal

- Chat `finish_reason: "content_filter"` means output was omitted due to provider filtering and maps to the normalized content-filter outcome.
- Chat/Responses refusal content is model output with explicit refusal semantics, not an HTTP transport error. Preserve it as a refusal item/delta without logging its text.
- Responses `incomplete` with a maximum-output reason maps to an incomplete/length finish, not a provider outage.

## Cancellation

- Every HTTP request is bound to its context. Client disconnect, gateway timeout, or routing cancellation closes the request body/response stream and propagates cancellation to the OpenAI request.
- OpenAI does not guarantee that disconnecting a synchronous HTTP request prevents processing or billing. After dispatch, cancellation remains an ambiguous potentially billable outcome unless final usage is received.
- No provider switch occurs after response commitment. Partial Chat or Responses output closes according to the public stream contract and is reconciled conservatively.

## Verification Plan

The adapter is not release-supported until deterministic local mock-server tests implement these scenarios. Fixtures belong under `internal/providers/openai/testdata/` when the Go module is scaffolded:

- `models/list.json`: valid discovery and malformed/oversized list responses.
- `chat/text.json`: non-stream text, finish reasons, request ID, and usage details.
- `chat/tool_calls.json`: one and parallel function calls with raw argument strings.
- `chat/refusal.json` and `chat/content_filter.json`: distinct refusal/filter behavior.
- `chat/stream_text.sse`: role, text deltas, finish chunk, final usage chunk, and `[DONE]`.
- `chat/stream_tools.sse`: interleaved indexed tool-call fragments.
- `chat/stream_truncated.sse`: missing usage/terminator, malformed JSON event, oversized event, idle timeout, and cancellation.
- `responses/text.json` and `responses/tools.json`: stateless non-stream output and usage mapping.
- `responses/stream_text.sse`: lifecycle, item/content, text delta/done, and completed usage.
- `responses/stream_tools.sse`: function item, argument delta/done, item done, and completion.
- `responses/stream_failed.sse`, `responses/stream_incomplete.sse`, and `responses/stream_error.sse`: terminal failure semantics without a success terminal.
- `embeddings/float.json`: ordered vectors and prompt/total usage.
- `errors/*.json`: 400, 401, 403, 404, 422, rate-limit 429, quota 429, 500, 503, malformed body, oversized body, and trustworthy/untrustworthy retry hints.
- Transport tests: authorization/header construction, optional organization/project headers, upstream request-ID capture, redirect rejection, TLS failure, cancellation, deadline, body closure, response-size bounds, and redaction sentinels.

The separate public compatibility gate pins exact official OpenAI SDK language/version pairs and commits representative golden Chat, Responses, Embeddings, stream, and error fixtures before implementing the gateway endpoints. The real gateway suite later expands request capture and exhaustive error/status coverage. Provider contract fixtures test upstream translation; official SDK fixtures test NexusRelay's public renderer. They are distinct test sets.

Optional live smoke tests use:

- `OPENAI_API_KEY` required.
- `OPENAI_ORGANIZATION` optional.
- `OPENAI_PROJECT` optional.
- Explicit model environment variables selected by the operator for Chat, Responses, and Embeddings; tests do not assume a globally available model ID.

The smoke suite lists models, performs minimal non-stream and stream calls for explicitly configured models, checks request IDs and usage when returned, and sends no production/user content. It is opt-in, never part of default CI, and never records credentials or response content.

## Control-Plane Limitations

Display these limitations for OpenAI connections and models:

- Model listing proves availability to the credential, not operation or feature support.
- Model capabilities and limits require verified metadata or administrator narrowing.
- Pricing and quotas are mutable and must be refreshed independently of model discovery.
- Usage may be unavailable on interrupted streams or ambiguous failures; NexusRelay may apply conservative estimated accounting.
- Native OpenAI uses the fixed public endpoint; custom or private-compatible endpoints require the separate OpenAI-compatible provider type.
- NexusRelay V1 does not expose OpenAI stored responses, conversations, background mode, built-in tools, provider-hosted tools, audio, image generation, or arbitrary pass-through fields.
