# Custom OpenAI-Compatible Provider Profile

## Verification Record

| Field | Value |
| --- | --- |
| Provider type | `openai_compatible` |
| Intended adapter | Shared OpenAI-compatible adapter with bounded typed connection settings |
| Profile status | `contract_verified` |
| Endpoint verification state | `operator_asserted` for every configured deployment endpoint |
| Owner / reviewer | NexusRelay maintainers |
| Verified at / review due | 2026-07-26 / 2026-10-26 |
| Contract version | NexusRelay custom OpenAI-compatible profile `v1` |
| Source identity | FR-PROV-005/006 and the approved contracts in designs 04, 05, 13, and 14 at the verification date |
| Live smoke state | Not applicable globally; recorded per configured endpoint when an operator runs it |

`contract_verified` verifies the NexusRelay configuration schema, security boundary, translation bounds, and deterministic fixture contract. It does not verify, certify, or imply compatibility of any arbitrary URL. A connection, its model IDs, and every asserted capability remain `operator_asserted` until tested in that deployment, and a successful test still does not convert them to provider-verified facts.

Downgrade this profile if the shared adapter accepts an unbounded dialect or header option, silently drops a supported public field, weakens fail-closed capability handling or SSRF controls, changes usage/error semantics without fixtures, fails a deterministic contract fixture, or passes its review due date.

## Contract Sources and Scope

This is a NexusRelay-owned configurable contract, not an external provider profile. There is no single upstream authority that can document all custom endpoints. The reviewed sources are:

- [Requirements](../requirements.md), especially FR-PROV-005 through FR-PROV-010, FR-API-003 through FR-API-014, FR-USAGE-004 through FR-USAGE-011, and SEC-006, SEC-007, and SEC-019.
- [Providers and Secrets Design](../design/04-providers-secrets.md), especially the capability model, shared adapter, outbound URL policy, connection testing, model discovery, error normalization, and verification sections.
- [Inference Protocol Design](../design/05-inference-protocol.md) for normalized request, response, error, timeout, cancellation, and streaming behavior.
- [V1 API Compatibility Matrix](../design/13-api-compatibility-matrix.md), which is the maximum public field set this profile may translate.
- [Provider Verification Ledger](../design/14-provider-verification.md), especially the shared-adapter and custom-endpoint gates.

The contract covers only the explicitly selected standard OpenAI dialects below. Claims such as "OpenAI compatible," an OpenAPI document supplied by an operator, or a successful request are evidence for operator review, not authority that widens this profile.

## Typed Connection Configuration

The `openai_compatible` connection uses a versioned schema. Unknown fields and unknown enum values are rejected. The control plane must show the resulting behavior before save and must not provide free-form request/response transformations.

### Base URL and Network Policy

- `base_url` is required and operator-asserted. It is an absolute URL without userinfo, query, or fragment. A normalized base path is allowed, and operation paths are appended beneath it without path escape.
- Standard operation suffixes are fixed by the selected dialect: `GET /models`, `POST /chat/completions`, `POST /responses`, and `POST /embeddings`. V1 does not accept arbitrary per-operation paths.
- `network_policy` is a required reference to either the deployment's public outbound policy or an authorized named private-network policy. It is not an arbitrary CIDR list embedded in a provider connection.
- `https` is required under the public policy. `http` is allowed only with an explicitly selected named private-network policy that permits the resolved destination.
- Redirects are disabled. Proxy environment variables are ignored unless the deployment's approved outbound policy explicitly enables a proxy.
- Every resolution and dial follows design 04: public destinations reject non-public ranges; private destinations require every resolved address to be within the named policy CIDRs; dialing pins a validated address while preserving URL-derived HTTP Host and TLS SNI; DNS is periodically revalidated.
- A named private-network policy expands the organization's SSRF trust boundary. It does not exempt URL, DNS, address, redirect, or header validation.

### Authentication and Optional Headers

- `authentication` is one of `bearer_api_key` or `none`. `bearer_api_key` sends exactly one `Authorization: Bearer <decrypted API key>` header. `none` is permitted only as an explicit choice for an endpoint that requires no credential.
- The API key is encrypted provider-secret material. It is never returned after save and never appears in connection-test output, logs, traces, audit metadata, fixtures, or errors.
- Optional headers are a bounded list with unique case-insensitive names, deployment-defined count/name/value size limits, and RFC-valid names and values. Client request headers are never forwarded into this list.
- Header values are secret by default and are stored in the encrypted provider-secret payload. A header may be stored as non-secret only when the schema explicitly classifies that header name as safe non-sensitive metadata.
- Reject authorization/cookie credentials outside the typed authentication field; control characters; `Host`; content length and transfer framing; `Connection`; `TE`; `Trailer`; `Upgrade`; proxy authorization; proxy, forwarding, and routing headers; and all other hop-by-hop or transport-owned headers. HTTP Host and TLS SNI derive only from `base_url`.

### Timeouts

The connection may narrow, but never exceed, deployment-enforced timeout ceilings through four positive bounded durations:

- `connect_timeout`
- `first_byte_timeout`
- `idle_stream_timeout`
- `total_timeout`

Omitted values use typed deployment defaults. `total_timeout` must be compatible with the other configured phases. A single ambiguous timeout field and disabling a timeout with zero or an unbounded value are rejected. Connection tests use their separate stricter worker timeout.

### Operation Availability and Static Models

Each operation has an explicit availability enum:

| Setting | Allowed values | Standard contract when enabled |
| --- | --- | --- |
| `models_endpoint` | `disabled`, `openai_models_v1` | `GET /models` with an OpenAI list envelope |
| `chat_endpoint` | `disabled`, `openai_chat_completions_v1` | `POST /chat/completions` |
| `responses_endpoint` | `disabled`, `openai_responses_v1` | `POST /responses` |
| `embeddings_endpoint` | `disabled`, `openai_embeddings_v1` | `POST /embeddings` |

- At least one inference operation must be enabled. An operation marked `disabled` is never dispatched or advertised by this connection.
- `static_model_ids` is a required, non-empty, bounded set unless the connection is being saved disabled for later completion. IDs must be non-empty bounded strings without control characters and are treated as opaque upstream identifiers.
- Model discovery, when enabled, produces suggestions and observed availability only. It never creates gateway models, deletes static IDs, or asserts operation/feature capability.
- A discovered model not present in the operator-approved static set is ineligible until explicitly added. A configured model omitted by later discovery becomes stale for review; it is not silently removed.
- Each model/operation association and feature capability records provenance as `operator_asserted`. Test observations may additionally be recorded as time-bounded `observed`, but not as `verified_profile`.

## Bounded Dialect Settings

The profile permits only the following typed behavior choices. Unknown or future variants require a profile/design update and deterministic fixtures; they cannot be entered as JSON templates, regular expressions, scripts, field maps, event maps, arbitrary status-code tables, or named-provider request augmentations. In particular, this custom profile cannot enable OpenRouter's `provider.require_parameters` object or any other provider-owned request field; those belong only to a compiled named profile reviewed under designs 04 and 14.

### Streaming

| Setting | Allowed values | Meaning |
| --- | --- | --- |
| `chat_stream_dialect` | `disabled`, `openai_sse_data_done_v1` | JSON in SSE `data` events using Chat chunks and terminal `data: [DONE]` |
| `responses_stream_dialect` | `disabled`, `openai_responses_sse_v1` | Typed OpenAI Responses JSON events with a terminal response event; no Chat-style terminator is assumed |

- Streaming is unavailable unless both the operation and its stream dialect are enabled and the selected model has an operator-asserted streaming capability.
- Parsers enforce bounded event size, valid UTF-8/JSON, expected object/event types, lifecycle ordering, stable identifiers/indexes, EOF rules, idle timeout, cancellation, and backpressure.
- SSE comments may be ignored. Unknown data event shapes, malformed JSON, invalid lifecycle transitions, missing required terminal state, and oversized events fail the attempt; they are never silently discarded as compatibility quirks.
- NexusRelay renders its own public stream grammar and success terminator only after durable finalization. It does not blindly relay the upstream terminator.

### Usage

| Setting | Allowed values | Meaning |
| --- | --- | --- |
| `chat_usage` | `unavailable`, `openai_chat_usage_v1` | Read `prompt_tokens`, `completion_tokens`, and `total_tokens`; optional standard detail fields remain individually nullable |
| `chat_stream_usage` | `unavailable`, `openai_chat_final_usage_chunk_v1` | Read one final empty-choices usage chunk before `[DONE]` |
| `responses_usage` | `unavailable`, `openai_responses_usage_v1` | Read `input_tokens`, `output_tokens`, `total_tokens`, and bounded standard detail fields from the response object |
| `embeddings_usage` | `unavailable`, `openai_embeddings_usage_v1` | Read `prompt_tokens` and `total_tokens` |

- Selecting a usage dialect asserts only the response shape. Values are provider-reported only when present and valid; absent categories remain unavailable, not zero.
- The adapter validates non-negative integer counts and internally checks standard total relationships. Invalid or contradictory usage is quarantined from authoritative accounting and the attempt is reconciled conservatively.
- No option treats arbitrary JSON paths, headers, text, or tokenizer estimates as provider-reported usage.

### Errors and Retry Hints

| Setting | Allowed values | Meaning |
| --- | --- | --- |
| `error_dialect` | `openai_error_envelope_v1`, `http_status_only` | Parse a bounded `{error:{message,type,param,code}}` envelope or classify from transport/HTTP status only |
| `retry_after` | `disabled`, `standard_retry_after` | Ignore retry timing or parse a bounded standard `Retry-After` value |

- HTTP status and transport phase remain authoritative inputs to the common normalized categories. An upstream error message, type, or code may refine internal classification only through adapter-owned allowlists; it is never returned raw.
- Provider authentication/permission failures do not become public gateway-key authentication/authorization errors. Rate limiting, quota exhaustion, model absence, invalid requests, timeouts, unavailable service, and unknown upstream failures remain distinct internal reasons where evidence permits.
- No configurable retryable-status list is accepted. The router applies the common bounded retry/fallback policy, and no fallback occurs after stream commitment.
- Error and response bodies are bounded before parsing and then discarded. Raw bodies and configured header values are never logged or exposed.

## Capabilities and Fail-Closed Behavior

- The shared adapter's implementation capability is only an upper bound. Effective capability is the intersection of the selected operation dialect, adapter translation support, operator assertion for the specific static model, any safe observed metadata, and administrator narrowing.
- All model feature capabilities default to false/unknown, including streaming, image input, tools, parallel tools, JSON mode, strict structured output, penalties, sampling fields, stop sequences, embedding base64, and embedding dimensions.
- Enabling an endpoint does not enable every field in that endpoint. Every capability-gated field in design 13 requires a separate model-level assertion supported by the shared adapter.
- An operator cannot assert a feature the selected dialect does not translate. An administrator may always narrow or disable a capability.
- Unsupported or unknown operation/feature combinations make the route ineligible and fail before dispatch if no eligible route remains. The adapter never silently drops a field to make a request appear compatible.
- Context limits, output limits, embedding dimensions, modality limits, and structured-schema limits are absent until explicitly configured from endpoint documentation or operator evidence. NexusRelay's own stricter safety bounds still apply.

## Connection Test and Health

- If `models_endpoint` is enabled, the default connection test and active health probe use authenticated `GET /models`, validate a bounded OpenAI list envelope, and report latency plus a normalized result. The body is not logged, and returned IDs are only observed suggestions.
- If Models is disabled, the connection test cannot claim inference compatibility. It performs configuration and SSRF validation and returns `not_testable_without_inference` unless the operator explicitly selects a configured static model and operation for a minimal billable smoke test.
- A billable smoke test is never the periodic default. It requires explicit confirmation, uses synthetic non-user content, is rate limited, observes the strict test timeout, and identifies which operation/model was tested.
- A test success proves only that the exact tested operation, model, credential, and dialect worked at that time. It does not prove untested operations, streaming, tools, structured output, usage, cost, model limits, or future availability.
- A connection remains disabled/ineligible until explicitly enabled; testing does not change status or route configuration.

## Cost and Billing Policy

- This generic contract has no authoritative provider pricing source and no standard per-request monetary cost field. The shared adapter must not infer provider cost from an arbitrary response extension.
- Operator-managed, versioned price records effective at request time are the only V1 monetary pricing source for a custom endpoint. Currency, token categories, and rates must cover every potentially billable category used by the route.
- A custom route is ineligible under a hard monetary budget when no complete applicable price record exists. Unknown price is never treated as zero.
- If a deployment intentionally asserts that a self-hosted endpoint has zero external provider charge, that is an explicit versioned operator pricing assertion. Hardware, electricity, and hosting allocation remain outside provider billing unless separately modeled in a future approved design.
- Once a request may have reached the endpoint, failed, timed-out, disconnected, cancelled, and missing-usage attempts are potentially billable. Reconciliation uses the greater of the conservative reservation and any valid local estimate, bounded by request/model maxima, until authoritative evidence exists. It never reconciles an ambiguous dispatched attempt to implicit zero.

## Deterministic Verification Fixtures

The shared adapter is not release-supported for custom endpoints until deterministic local mock-server tests cover at least:

- Schema rejection for unknown settings/enums, invalid timeout combinations, missing static models, and operation/dialect conflicts.
- Bearer and no-auth modes; secret/non-secret optional headers; duplicate and forbidden headers; redaction sentinels.
- Public and named private-network policies; HTTPS/HTTP rules; URL normalization and path joining; userinfo/query/fragment rejection; redirect denial; DNS rebinding; mixed allowed/disallowed DNS answers; IPv4/IPv6 and encoded-host cases; Host/SNI derivation.
- Models enabled/disabled behavior, bounded discovery, static allowlist intersection, stale observations, and the non-inference test result.
- Non-stream Chat, Responses, and Embeddings standard requests/responses, including unsupported-field pre-dispatch rejection.
- Chat SSE text/tool fragments, final usage chunk, `[DONE]`, comments, malformed/oversized events, unexpected shapes, missing terminator, timeout, EOF, cancellation, and body closure.
- Responses SSE lifecycle, text/function fragments, completed/failed/error terminals, malformed transitions, timeout, EOF, cancellation, and body closure.
- Every usage enum with valid, missing, negative, overflowing, and contradictory counts; no implicit zero and conservative accounting fallback.
- OpenAI error-envelope and status-only modes across 400, 401, 403, 404, 422, rate-limit 429, quota-like 429 where safely identifiable, 500, 502, 503, malformed/oversized bodies, and bounded/untrusted `Retry-After`.
- Capability intersection and narrowing for every design 13 capability-gated field, proving unknown/false capabilities never dispatch and no field is silently dropped.
- Ambiguous post-dispatch timeout/cancellation and prevention of retry after stream commitment.

An optional deployment smoke suite accepts an explicit test base URL, secret API key or no-auth selection, named network policy, safe optional headers, and explicit model IDs per operation. It records endpoint configuration version, operation, model ID, dialect, timestamp, and normalized outcome without credentials or model content. Results expire and never change the profile or endpoint state to provider-verified.

## Control-Plane Warnings

Display these warnings on create, edit, test, model mapping, and route eligibility views:

- NexusRelay verifies only the bounded adapter contract. This endpoint and its compatibility claims are operator-asserted, not externally provider-verified.
- A successful connection or model-list test does not prove inference, streaming, feature, usage, error, pricing, cancellation, or billing semantics.
- Every model ID and capability must be explicitly asserted and defaults fail closed. Unsupported fields are rejected rather than dropped.
- Custom URLs are SSRF-sensitive. Selecting a named private-network policy expands trusted egress to that policy's CIDRs; HTTP is allowed only under such a policy.
- Optional headers may contain secrets, are never forwarded from clients, and cannot override transport, routing, proxy, forwarding, Host, or authorization behavior outside the typed fields.
- Pricing and billing are operator-maintained. Routes without complete versioned pricing are ineligible under hard monetary budgets, and ambiguous dispatched attempts are reconciled conservatively.
- Model discovery is an observed snapshot, not a capability or long-term availability guarantee.
- Changing dialect, network policy, base URL, authentication, headers, endpoint availability, usage/error settings, timeout settings, model IDs, or capabilities invalidates prior test observations and requires retesting before operator reliance.
