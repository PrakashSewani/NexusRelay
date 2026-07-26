# V1 API Compatibility Matrix

## Status Definitions

- `supported`: NexusRelay preserves the documented semantics across at least one eligible adapter.
- `capability-gated`: accepted only when an eligible target explicitly supports it; otherwise rejected before dispatch.
- `constrained`: supported with documented V1 bounds or normalized subset.
- `rejected`: recognized but not honored in V1; returns a stable validation error.
- `not in V1`: endpoint/field is outside the committed public surface.

This is the gateway baseline. Provider profiles may narrow support but cannot widen the public contract without updating this matrix and tests.

## Common HTTP Behavior

| Behavior | V1 status | Notes |
| --- | --- | --- |
| Bearer API key | supported | Exactly one `Authorization: Bearer` value |
| JSON request body | supported | Configurable bounded size |
| Gzip request body | rejected initially | Avoid decompression abuse until explicitly designed |
| `x-request-id` response header | supported | Gateway-generated ID |
| Client correlation ID | constrained | Stored separately after strict validation; never replaces gateway ID |
| SSE streaming | supported | Incremental, bounded, no fallback after commitment |
| OpenAI-compatible error envelope | supported | Includes stable gateway code and request ID |
| Provider-specific arbitrary fields | rejected | Future namespaced extension only |
| Unknown core request fields | rejected | Stable `unknown_field` error with field path |

## Normative Error Contract

All pre-commit errors use `Content-Type: application/json`, the envelope in `05-inference-protocol.md`, and `x-request-id`. `param` is present only for a safe client field path. Authentication failures do not distinguish malformed, unknown, expired, disabled, or revoked keys publicly.

| Gateway code/category | HTTP | OpenAI-style `type` | Retry metadata |
| --- | --- | --- | --- |
| `authentication_error` | 401 | `authentication_error` | none |
| `permission_denied` | 403 | `permission_error` | none |
| `invalid_request`, including malformed JSON/media type/body limit/unknown field | 400 | `invalid_request_error` | none |
| `model_not_found`, `model_not_allowed` | 404 | `invalid_request_error` | none |
| `unsupported_model_capability` | 400 | `invalid_request_error` | none |
| `gateway_rate_limited` | 429 | `rate_limit_error` | `Retry-After` and bounded gateway rate-limit headers when known |
| `budget_exceeded` | 429 | `rate_limit_error` | no fabricated reset time; budget period metadata is administrative only |
| `provider_rate_limited` after fallback exhaustion | 429 | `rate_limit_error` | sanitized bounded `Retry-After` when trustworthy |
| `provider_unavailable` | 503 | `server_error` | optional bounded `Retry-After` |
| `request_timeout` | 504 | `server_error` | none |
| `content_filtered` | 400 | `invalid_request_error` | none |
| `upstream_error` | 502 | `server_error` | none |
| `internal_error` before response commitment | 500 | `server_error` | none |
| finalization unavailable before response commitment | 503 | `server_error` | none |

After SSE commitment, HTTP status cannot change. Chat streams may emit a final sanitized JSON error `data` event only when the pinned SDK fixture accepts it, then close without `[DONE]`; otherwise they close. Responses streams emit `response.failed`, then close. A success terminal event is never emitted when durable finalization failed.

## `GET /v1/models`

| Field/behavior | V1 status | Notes |
| --- | --- | --- |
| List object | supported | Only enabled aliases allowed for the key |
| `id` | supported | Gateway model key |
| `object = model` | supported | Compatibility value |
| `created` | constrained | Gateway model creation Unix time |
| `owned_by` | constrained | Stable NexusRelay/organization-safe value |
| Provider/upstream IDs | rejected from output | Internal routing details are not exposed |
| Retrieve/delete model endpoints | not in V1 | Administration uses control-plane API |

## `POST /v1/chat/completions`

| Request field | V1 status | Notes |
| --- | --- | --- |
| `model` | supported | Required gateway model key |
| `messages` | supported | Ordered system/developer/user/assistant/tool roles in normalized subset |
| Text content | supported | String and normalized text-part forms |
| Image input parts | capability-gated | NexusRelay never dereferences remote URLs in V1; it validates URL syntax/length and forwards only to an eligible provider. Data URLs have decoded byte/media-type limits. |
| Audio input parts | rejected initially | Add only with normalized modality design |
| `stream` | supported | Boolean |
| `stream_options.include_usage` | constrained | Usage emitted when available at completion |
| `temperature` | capability-gated | Validated range; no silent dropping |
| `top_p` | capability-gated | Validated range; no silent dropping |
| `max_tokens` | constrained | Compatibility alias; supplying both token-limit fields is `invalid_request` |
| `max_completion_tokens` | supported | Subject to target limits |
| `stop` | capability-gated | String or bounded array |
| `n` | constrained | V1 supports `1`; values greater than one rejected |
| `tools` | capability-gated | Function tools with bounded JSON Schema |
| `tool_choice` | capability-gated | `none`, `auto`, `required`, or named function where provider supports it |
| `parallel_tool_calls` | capability-gated | Rejected when target lacks support |
| `response_format: json_object` | capability-gated | JSON mode |
| `response_format: json_schema` | capability-gated | Strict structured output only where semantics can be honored |
| `seed` | rejected initially | No cross-provider determinism guarantee |
| `logprobs`, `top_logprobs` | rejected initially | Add only with normalized response design |
| `logit_bias` | rejected initially | Provider semantics vary materially |
| `frequency_penalty`, `presence_penalty` | capability-gated | Explicit provider support required |
| `user` | constrained | Accepted as non-authoritative client metadata; hashed/not persisted by default |
| `service_tier`, `store`, `metadata` | rejected initially | No implicit provider pass-through/storage |

### Chat Response

| Field/behavior | V1 status | Notes |
| --- | --- | --- |
| `id`, `object`, `created`, `model` | supported | `model` remains gateway alias |
| `choices[].message.content` | supported | Text output |
| `choices[].message.tool_calls` | capability-gated | Normalized function calls |
| `choices[].finish_reason` | supported | Stable normalized mapping |
| `usage.prompt_tokens/completion_tokens/total_tokens` | constrained | Present only for provider-reported values; estimates remain in administrative APIs |
| Provider fingerprint/tier fields | rejected or omitted | Not synthesized without semantics |

### Chat Streaming

| Event behavior | V1 status | Notes |
| --- | --- | --- |
| Role/content deltas | supported | Ordered incremental deltas |
| Tool-call deltas | capability-gated | Stable index/ID assembly |
| Finish reason | supported | Final choice delta/chunk |
| Usage final chunk | constrained | When requested and provider-reported |
| `[DONE]` terminator | supported | Sent only after durable finalization |
| Mid-stream gateway error | constrained | Best protocol-compatible termination; HTTP status cannot change after commit |

Normative chat ordering for one-choice V1 streams is: one role delta, zero or more ordered content/tool-call deltas, one finish-reason chunk, optional provider-reported usage chunk when requested, then `[DONE]` after durable finalization. Every chunk uses the gateway request-derived completion ID, gateway model alias, stable choice index `0`, and `chat.completion.chunk` object. Tool-call fragments preserve stable call index and ID; arguments remain incremental strings and are never parsed for logging.

## `POST /v1/responses`

V1 supports a normalized subset rather than every OpenAI Responses feature.

| Request field/behavior | V1 status | Notes |
| --- | --- | --- |
| `model` | supported | Gateway model key |
| String text input | supported | Normalized to user text item |
| Structured input message items | constrained | Text, image, tool call/result subset |
| `instructions` | supported | Normalized system/developer instruction |
| `stream` | supported | SSE event normalization |
| `max_output_tokens` | supported | Target limit applies |
| `temperature`, `top_p` | capability-gated | No silent dropping |
| `tools`, `tool_choice`, parallel calls | capability-gated | Function tools only in initial V1 |
| Text format JSON schema | capability-gated | Same semantics as structured output |
| Built-in web/file/computer/code tools | rejected | NexusRelay does not host tools in V1 |
| Conversation IDs/previous response state | rejected | Gateway is stateless for model content |
| Background responses | rejected | No asynchronous model-content storage |
| `store` | rejected | Content is not persisted |
| Provider-specific reasoning controls | rejected initially | Add only after normalized semantics |

### Responses Output and Streaming

| Behavior | V1 status | Notes |
| --- | --- | --- |
| Text output items | supported | Normalized response object |
| Function-call output items | capability-gated | Function tools only |
| Usage | constrained | Provider-reported values only in standard compatible fields |
| Lifecycle stream events | constrained | Exact V1 grammar below |
| Durable stored response retrieval | rejected | No content persistence |

Normative Responses event order is:

```text
response.created
response.in_progress
response.output_item.added
zero or more response.output_text.delta or response.function_call_arguments.delta
response.output_item.done
response.completed
```

`response.failed` replaces `response.completed` on a terminal post-commit error. Sequence numbers are monotonically increasing from zero. Response and item IDs remain stable across the stream. Gateway-generated `created`/`in_progress` events are buffered until the upstream route is viable. Golden fixtures define exact JSON fields, null/omission rules, tool-call lifecycle, cancellation, and usage for pinned official SDK versions.

## `POST /v1/embeddings`

| Request/response field | V1 status | Notes |
| --- | --- | --- |
| `model` | supported | Embedding-capable gateway model |
| String input | supported | Bounded length |
| Array of strings | supported | Bounded item count and aggregate size |
| Token-ID arrays | rejected initially | Tokenizer/provider semantics differ |
| `encoding_format: float` | supported | JSON number vector |
| `encoding_format: base64` | capability-gated | Only with stable normalized encoding |
| `dimensions` | capability-gated | Target must support dimension control |
| `user` | constrained | Non-authoritative metadata, not persisted by default |
| Ordered embedding objects | supported | Preserve input order/index |
| Usage | constrained | Provider-reported token usage only in the standard response field |

## Validation and Disclosure

- Known unsupported and genuinely unknown core fields return `invalid_request` with a stable field/code.
- Capability-gated fields filter incompatible targets and require at least one remaining eligible target; they do not require every configured target to support the feature.
- If the key cannot access a model, response does not disclose hidden provider/target details.
- Provider adapters may reject stricter provider-specific bounds after route selection; those failures are normalized and are not silently retried when client input is invalid.
- Exact official SDK versions used for compatibility are pinned in test documentation.
- The baseline is not implementation-ready until test documentation names exact SDK language/version pairs and commits golden request, response, stream, and error fixtures.

## Change Policy

Adding support requires updates to normalized types, adapter capability/translation, renderer, this matrix, API documentation, provider profiles, and compatibility tests. Removing previously supported behavior is a public compatibility change and requires versioning or a documented migration path.
