# Groq Provider Profile

## Verification Record

| Field | Value |
| --- | --- |
| Provider type | `groq` |
| Intended adapter | Shared OpenAI-compatible adapter with a typed Groq profile |
| Status | `profile_drafted` |
| Owner / reviewer | NexusRelay maintainers |
| Drafted at / review due | 2026-07-26 / before promotion or V1 scope freeze |
| API version | OpenAI-compatible `/openai/v1`; Responses currently beta |
| Source identity | Official URLs and retrieval date; rendered documentation publishes no immutable content hash |
| Live smoke state | Not run |

This profile is not implementation-ready. Promotion to `contract_verified` requires authoritative or reproducibly pinned evidence for the exact final Chat stream usage chunk shape and ordering, plus the accepted beta Responses lifecycle/terminal grammar. After promotion, downgrade on material endpoint, beta Responses, stream, usage, error, rate-limit, or billing changes, fixture failure, or review expiry.

## Sources

Official Groq documentation retrieved 2026-07-26:

- [OpenAI compatibility](https://console.groq.com/docs/openai)
- [API reference](https://console.groq.com/docs/api-reference)
- [Responses API](https://console.groq.com/docs/responses-api)
- [Models](https://console.groq.com/docs/models)
- [Tool use](https://console.groq.com/docs/tool-use/overview)
- [Structured outputs](https://console.groq.com/docs/structured-outputs)
- [Vision](https://console.groq.com/docs/vision)
- [Rate limits](https://console.groq.com/docs/rate-limits)
- [Errors](https://console.groq.com/docs/errors)
- [Billing](https://console.groq.com/docs/billing-faqs)

## Connection and Discovery

- Fixed base URL: `https://api.groq.com/openai/v1`; authenticate with `Authorization: Bearer <GROQ_API_KEY>`.
- Endpoints used by V1: `GET /models`, `POST /chat/completions`, and beta `POST /responses`. Groq publishes no Embeddings endpoint; embeddings are unsupported for this provider.
- TLS verification is mandatory, redirects are disabled, and the public-network SSRF policy applies.
- Connection test/health probe: authenticated `GET /models`. Capture bounded identity, active state, context window, and output limit metadata. Listing consumes request quota but has no documented token charge.
- Operation and feature support remains model-dependent. Frequency/presence penalties, logit bias, logprobs/top-logprobs, metadata, and storage are explicitly unsupported for Chat and must be rejected before dispatch.

## Chat and Responses

- Chat uses the OpenAI-compatible schema with `n` fixed to one. `temperature: 0` is transformed by Groq to `1e-8`; NexusRelay records this semantic and does not promise exact zero-temperature behavior.
- Text, image input, function tools, named/required tool choice, parallel tool calls, JSON mode, and strict structured output are capability-gated. Built-in Groq tools, Compound fields, documents, search, reasoning controls, service tiers, and provider-specific fields are outside V1.
- Chat SSE uses data-only OpenAI chunks and terminates with `data: [DONE]`. The exact final usage shape, whether it is a distinct empty-choices chunk, its ordering relative to finish chunks and `[DONE]`, and behavior when usage is unavailable remain unresolved and block implementation; interrupted streams may lack usage.
- Responses is beta and stateless in the NexusRelay subset. Groq explicitly does not support `previous_response_id`, `store`, `truncation`, `include`, reusable prompts, safety identifiers, or prompt-cache keys. Function tools, image input, structured text, and streaming are capability-gated; built-in tools and reasoning output are excluded. Exact accepted stream event families, terminal transitions, and usage placement must be pinned before promotion.
- Beta status is a control-plane warning and downgrade trigger, not a reason to infer unsupported fields.

## Usage, Cost, Limits, and Errors

- Chat reports `prompt_tokens`, `completion_tokens`, and `total_tokens`, plus timing metadata. Responses reports `input_tokens`, `output_tokens`, `total_tokens`, cached input details, and reasoning output details when applicable. Missing details are unavailable, not zero.
- Groq does not return authoritative monetary cost in inference responses. Use versioned USD model pricing effective at request time. Hard-budget routes require coverage for every billable token/category; interrupted or cancelled requests reconcile conservatively.
- Groq states 500/502/503 server-error responses are not charged. No general free-of-charge guarantee is documented for client errors, timeouts, disconnects, or cancellations.
- Rate limits are organization/model dependent and measured across request, token, daily, and modality dimensions. Parse `retry-after`, request-day and token-minute limit/remaining/reset headers as advisory metadata. Cached tokens do not count toward Groq rate limits.
- Parse bounded `{error:{message,type}}` envelopes. Normalize 400/413/422 as request failures, 401/403 as provider auth/permission, 404 as model/resource missing, 429 as provider rate limit, 498 as flex capacity/unavailable, and retryable 500/502/503 as provider unavailable. Groq uses 499 in logs for caller cancellation.

## Cancellation and Verification

- Bind HTTP requests to context and close streams on cancellation. Groq documents cancellation classification but not a guarantee that disconnect stops processing or billing, so post-dispatch cancellation remains ambiguous.
- Promotion fixtures must cover auth, model listing, unsupported-field rejection, temperature-zero transformation, Chat text/images/tools/schema, the exact final SSE usage/finish/`[DONE]` ordering and missing-usage cases, the fully pinned beta Responses text/tools/images/schema lifecycle, all documented errors and rate headers, cancellation, redirects, bounds, body closure, and redaction.
- Optional smoke tests use `GROQ_API_KEY` and explicit Chat and Responses model IDs. Embeddings smoke tests do not exist.

Control-plane warnings: Responses is beta; Embeddings is unsupported; model capabilities/prices/limits are mutable; unsupported OpenAI fields return 400 rather than being silently dropped; provider-specific tools, service tiers, audio, Compound, batches, and fine-tuning are outside NexusRelay V1.
