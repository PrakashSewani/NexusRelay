# Google Gemini Provider Profile

## Verification Record

| Field | Value |
| --- | --- |
| Provider type | `google_gemini` |
| Intended adapter | Dedicated native Gemini adapter |
| Status | `profile_drafted` |
| Owner / reviewer | NexusRelay maintainers |
| Drafted at / review due | 2026-07-26 / before promotion or V1 scope freeze |
| API version | Gemini Developer API REST `v1beta` |
| Source identity | Official URLs and retrieval date; rendered pages publish no immutable version or content hash |
| Live smoke state | Not run |

This profile is not implementation-ready. Promotion to `contract_verified` requires an exhaustive, reviewed mapping from every accepted native `finishReason` and prompt-feedback terminal condition to the normalized Chat/Responses finish, incomplete, content-filter, or upstream-error outcomes, with deterministic non-stream and stream fixtures. Do not infer unmapped classes from their names. After promotion, downgrade on material authentication, endpoint, stream, usage, model-metadata, billing, or mapping changes, fixture failure, or review expiry.

## Sources

Official Google documentation retrieved 2026-07-26:

- [GenerateContent and streamGenerateContent](https://ai.google.dev/api/generate-content)
- [Embeddings](https://ai.google.dev/api/embeddings)
- [Models](https://ai.google.dev/api/models)
- [API keys](https://ai.google.dev/gemini-api/docs/api-key)
- [Function calling](https://ai.google.dev/gemini-api/docs/function-calling)
- [Structured output](https://ai.google.dev/gemini-api/docs/structured-output)
- [Rate limits](https://ai.google.dev/gemini-api/docs/rate-limits)
- [Troubleshooting](https://ai.google.dev/gemini-api/docs/troubleshooting)
- [Pricing](https://ai.google.dev/gemini-api/docs/pricing)

The rendered documentation does not publish immutable content hashes. Model availability, limits, quotas, preview status, and prices are mutable snapshots rather than permanent profile facts.

## Connection

- Fixed base URL: `https://generativelanguage.googleapis.com/v1beta`.
- Send the configured key as `x-goog-api-key`. Query `key=` is documented, but NexusRelay uses the header so credentials do not enter URLs or access logs.
- New Google AI Studio keys may be service-account-bound authorization keys. NexusRelay treats both accepted key types as opaque secrets and does not implement OAuth in V1.
- TLS verification is mandatory, redirects are disabled, and the public-network SSRF policy applies.
- Connection test and active probe: authenticated `GET /v1beta/models?pageSize=1`. HTTP 200 with a bounded `models` array is success. It consumes request quota but has no documented token charge. A successful list proves authentication and API reachability, not inference entitlement.

## Model Discovery

- `GET /models` is paginated with `pageSize` and `pageToken`; `GET /models/{model}` retrieves one record.
- Capture `name`, `baseModelId`, `version`, `displayName`, `description`, `inputTokenLimit`, `outputTokenLimit`, and `supportedGenerationMethods` when present.
- `supportedGenerationMethods` is authoritative for operation discovery at retrieval time. Tools, images, structured output, and detailed limits remain model-specific capability facts and fail closed when unknown.

## Operations

### Chat Completions Translation

- Non-stream endpoint: `POST /models/{model}:generateContent`.
- Stream endpoint: `POST /models/{model}:streamGenerateContent?alt=sse`.
- Ordered messages map to `contents[].role` and `parts[]`; system/developer instructions map to `systemInstruction`. Text and bounded inline image data map to native parts. NexusRelay does not upload or fetch remote media in this adapter.
- Function declarations map to native `tools[].functionDeclarations`; model `functionCall` parts and client `functionResponse` parts are correlated without logging arguments.
- Sampling, stop, and output-token limits map only where model capability allows. `candidateCount` is fixed to one.
- JSON mode and schema output map to `responseMimeType: application/json` plus `responseJsonSchema` for the reviewed supported subset. Unsupported schema keywords are rejected before dispatch.
- Native candidates map text, function calls, safety/refusal state, and `finishReason`. The exact mapping is intentionally unresolved: Google finish reasons including `STOP`, `MAX_TOKENS`, safety/prohibited-content classes, malformed function/response classes, and tool-call-limit classes require an exhaustive reviewed table before implementation. Until that table exists, Chat and the stateless Responses translation are blocked rather than falling through to guessed defaults.

### Streaming

- The response is SSE. Each `data:` event contains one `GenerateContentResponse`; there is no documented `[DONE]` sentinel. Successful HTTP EOF after a terminal candidate/usage response terminates the upstream stream.
- Preserve ordered text and function-call deltas. Ignore valid SSE comments and reject malformed or oversized events.
- `usageMetadata` can arrive with streamed responses. NexusRelay emits its public usage chunk and `[DONE]` only after durable accounting finalization.
- An error or disconnect before a terminal response is ambiguous and potentially billable; never switch providers after public response commitment.

### Responses and Embeddings

- Gemini has no native OpenAI Responses endpoint. NexusRelay may implement the stateless V1 Responses subset by translating it through GenerateContent; stored responses, background mode, hosted tools, and provider state are excluded.
- Single embedding: `POST /models/{model}:embedContent`.
- Batched strings: `POST /models/{model}:batchEmbedContents`; responses preserve request order.
- `outputDimensionality` is capability-gated. Set auto-truncation false when available so NexusRelay does not silently truncate client input.
- Map vectors and provider `promptTokenCount`; output-token usage is unavailable for embeddings.

## Usage, Cost, and Limits

- Generation usage includes `promptTokenCount`, `cachedContentTokenCount`, `candidatesTokenCount`, `toolUsePromptTokenCount`, `thoughtsTokenCount`, `totalTokenCount`, and modality details when present. Missing dimensions are unavailable, not zero.
- Gemini does not return authoritative per-request monetary cost. Use a versioned USD pricing record effective at request time, including input modality, cached input, output/thinking, service tier, and separately billed tools.
- A route is ineligible under a hard monetary budget unless every possible billable category has a reviewed effective price. Interrupted/failed requests reconcile conservatively because Google does not guarantee they are free.
- Quotas are project/tier/model dependent and mutable. Normalize `429 RESOURCE_EXHAUSTED` as rate or quota failure using safe details; do not copy current numeric limits into adapter constants.

## Errors and Cancellation

- Parse bounded `google.rpc.Status` errors and discard raw bodies. Map `400 INVALID_ARGUMENT` to invalid request, `400 FAILED_PRECONDITION` to provider configuration/availability, `403 PERMISSION_DENIED` to provider authentication/permission, `404 NOT_FOUND` to model/resource not found, `429 RESOURCE_EXHAUSTED` to provider rate/quota, and retryable `408`/`5xx` to timeout/unavailable.
- Safety prompt feedback and candidate safety finish reasons are content-filter outcomes, not gateway authentication errors.
- Bind each request to context and close the HTTP response on cancellation. The docs do not guarantee cancellation stops processing or billing, so cancellation after dispatch is conservatively billable.

## Verification Plan and Limitations

Promotion fixtures must cover model pagination, text and image input, function calls/results, schema output, every documented finish/prompt-feedback class and its exact normalized outcome, non-stream usage, SSE text/tools/usage/EOF, malformed and interrupted streams, embeddings order/dimensions, bounded errors, cancellation, redirects, and redaction. Optional smoke tests use `GEMINI_API_KEY` and explicit Chat and Embeddings model IDs.

Control-plane warnings: model support and prices are mutable; Responses is a stateless translation; remote media is not fetched; native hosted tools, files, audio, video, Live API, tuning, caches, batch jobs, and image generation are outside NexusRelay V1.
