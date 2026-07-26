# Ollama Provider Profile

## Verification Record

| Field | Value |
| --- | --- |
| Provider type | `ollama` |
| Intended adapter | Dedicated native Ollama adapter |
| Status | `contract_verified` |
| Owner / reviewer | NexusRelay maintainers |
| Verified at / review due | 2026-07-26 / 2026-10-26 |
| API version | Unversioned stable native API; published OpenAPI `0.1.0` |
| Source identity | Official URLs, published OpenAPI version, and retrieval date; docs state the API is not strictly versioned |
| Live smoke state | Not run |

Downgrade on incompatible native API/OpenAPI, stream, auth, usage, or error changes; fixture failure; or review expiry.

## Sources

Official Ollama documentation retrieved 2026-07-26:

- [API introduction](https://docs.ollama.com/api/introduction)
- [Chat](https://docs.ollama.com/api/chat)
- [Embeddings](https://docs.ollama.com/api/embed)
- [List models](https://docs.ollama.com/api/tags)
- [Errors](https://docs.ollama.com/api/errors)
- [Streaming](https://docs.ollama.com/capabilities/streaming)
- [Tool calling](https://docs.ollama.com/capabilities/tool-calling)
- [Structured output](https://docs.ollama.com/capabilities/structured-outputs)
- [Vision](https://docs.ollama.com/capabilities/vision)
- [Cloud](https://docs.ollama.com/cloud)
- [OpenAI compatibility](https://docs.ollama.com/openai)

## Connection and Security

- Native local default: `http://localhost:11434/api`, no authentication.
- Native cloud: `https://ollama.com/api`, `Authorization: Bearer <OLLAMA_API_KEY>`.
- A local connection requires an explicit named private-network policy. HTTP is allowed only under that policy. Redirects remain disabled and every resolved address must satisfy the configured CIDRs.
- Base URL is operator-configurable for self-hosted Ollama and normalized to the native `/api` root. NexusRelay does not use Ollama's partial `/v1` compatibility layer because native NDJSON, usage, and errors require an explicit profile.
- Connection test/health probe: `GET /api/tags`. It has no model-generation cost locally; cloud quota/cost for listing is undocumented. A successful list does not prove a model is loaded or supports a feature.

## Discovery and Operations

- `GET /api/tags` returns locally/credential-available models with name, digest, modification time, size, family, parameters, and quantization. It does not provide a complete capability matrix.
- Chat: `POST /api/chat`. Map system/user/assistant/tool messages, text, base64 image data, function tools/calls, generation options, and `format: "json"` or a JSON schema only after model capability checks.
- Ollama has no native OpenAI Responses-equivalent. NexusRelay does not expose Responses through this provider in V1.
- Embeddings: `POST /api/embed`, accepting one string or an array. Send `truncate: false` to prevent silent truncation. `dimensions` is capability-gated. Preserve vector order and map `prompt_eval_count`.
- Native reasoning/thinking, logprobs, image output, model management, pulls, pushes, creation, blobs, and arbitrary runtime options are outside the V1 public matrix.

## Streaming

- Chat streams by default as `application/x-ndjson`, one JSON object per line. Non-streaming sends `stream: false` and returns one JSON object.
- Content and tool-call fragments are incremental. The terminal object has `done: true` and may include `done_reason`, duration fields, `prompt_eval_count`, and `eval_count`. There is no `[DONE]` marker.
- A mid-stream error is an NDJSON object containing `error`; the HTTP status remains committed. It is terminal and must not be followed by a public success terminator.
- Enforce bounded line size, incremental parsing, backpressure, idle timeout, and EOF validation.

## Usage, Cost, Errors, and Cancellation

- Chat reports prompt tokens as `prompt_eval_count` and output tokens as `eval_count` on completion. Embeddings report `prompt_eval_count`. Duration fields are operational metadata, not usage.
- Local Ollama has zero provider monetary charge for NexusRelay accounting, while operator electricity/hardware cost is outside the provider ledger. Ollama Cloud does not document authoritative per-request cost in the inference response; cloud routes require a versioned price record or are ineligible under hard monetary budgets.
- Native errors use bounded `{ "error": "..." }` bodies. Map 400 validation, 404 model missing, 429 rate limit, 500 internal, and 502 cloud-unreachable conditions without exposing raw messages.
- Bind requests to context and close connections on cancellation. Ollama does not document a processing/billing guarantee after disconnect; cloud cancellation is potentially billable. Local cancelled attempts retain estimated usage when final counts are absent.

## Verification Plan and Limitations

Fixtures must cover local unauthenticated and cloud Bearer modes, private-network policy, model listing, text/image/tools/schema Chat, non-stream usage, NDJSON text/tools/terminal/error/truncation, embeddings order/dimensions/no-truncate, status mapping, cancellation, EOF, body closure, bounds, and redaction. Smoke tests support an explicit `OLLAMA_BASE_URL`, optional `OLLAMA_API_KEY`, and explicit Chat/Embeddings model IDs.

Control-plane warnings: local endpoints expand SSRF trust; model capabilities are not fully discoverable; Responses is unsupported; cloud prices/quotas are mutable and not returned per request; Ollama's OpenAI compatibility endpoint is not the contract used by NexusRelay.
