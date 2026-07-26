# Inference Protocol Design

## Scope

This design defines the public OpenAI-compatible API boundary, provider-neutral domain types, validation, request identity, normalized errors, streaming, cancellation, timeouts, response commitment, and compatibility policy.

## Public Endpoints

```text
GET  /v1/models
POST /v1/chat/completions
POST /v1/responses
POST /v1/embeddings
```

Chat Completions is delivered before Responses and Embeddings, but all are V1 scope. Unsupported fields or operations return explicit errors; the gateway never silently claims full OpenAI compatibility.

## HTTP Boundary

- Authentication uses `Authorization: Bearer <gateway-key>`.
- Content type is `application/json` for requests.
- Streaming responses use `text/event-stream` and disable intermediary buffering.
- Request body size is bounded before JSON decoding.
- Unknown core JSON fields are rejected with a stable `unknown_field` validation error. Accepted pass-through is never implicit; listed compatibility fields follow the endpoint matrix.
- JSON number and integer bounds are validated before conversion.
- Public responses include `x-request-id`.
- A valid client-supplied request ID may be accepted as a separate `client_request_id` correlation value only. The gateway always generates its own globally unique public `request_id`.

## Request Identity

At the start of every request, the gateway creates:

- `request_id`: opaque public correlation identifier.
- `request_entity_id`: internal UUIDv7 persisted primary key.
- Trace context: continued from valid W3C headers or newly created.

The public request ID appears in response headers and error bodies. It propagates to logs, traces, attempts, usage, and audit correlation, but never becomes a Prometheus label.

## Normalized Domain

Provider-neutral types represent semantics rather than a copy of any provider SDK.

### Common Request Context

```text
operation
gateway_model_id
stream
input modalities
output modalities
sampling parameters
maximum output tokens
stop conditions
tools and tool choice
structured output constraints
client metadata allowed by policy
required capability set
```

### Content Model

Messages/items contain ordered content parts with explicit variants:

```text
text
image reference or encoded image (only when supported and bounded)
tool call
tool result
refusal
```

Tool arguments remain raw JSON at the forwarding boundary but are never logged or persisted. Validation enforces structural and size limits without interpreting application semantics.

### Normalized Usage

```text
input_tokens:          { value, source }
output_tokens:         { value, source }
total_tokens:          { value, source }
cached_input_tokens:   { value, source }
reasoning_tokens:      { value, source }
provider_observed_cost { nanos, unit, currency, source }
```

Each token dimension has an independent source of `provider_reported`, `estimated`, or `unavailable`; optional values distinguish zero from unavailable. Provider-observed cost is preserved separately from the authoritative budget charge and is used for that charge only when its unit/currency semantics are verified and compatible.

### Normalized Response

The normalized response carries:

- Provider-neutral output choices/items.
- Finish reason mapped to a stable internal enum.
- Usage.
- Provider request ID.
- Safe provider metadata needed for protocol rendering.

The HTTP renderer converts normalized responses back to the endpoint-specific OpenAI-compatible shape.

## Capability Derivation

Transport validation derives required capabilities from request fields before routing. Examples:

- `stream: true` requires streaming.
- Non-empty tools require tool support.
- Strict JSON schema requires structured-output support.
- Image content requires image input.
- Embeddings endpoint requires embeddings.

The route engine receives this set and excludes incompatible targets. If the visible model has no target that can satisfy it, return HTTP 400 `invalid_request` with stable code `unsupported_model_capability`.

## Validation Layers

1. HTTP validation: method, content type, body size, JSON syntax.
2. Protocol validation: required fields, ranges, endpoint-specific combinations.
3. Normalization validation: representable provider-neutral semantics.
4. Policy validation: key restrictions, model access, budgets, limits.
5. Route validation: at least one target supports required capabilities.
6. Adapter validation: provider-specific constraints that cannot be known earlier.

Errors are returned at the earliest safe layer. A request rejected after API key identity is known is still recorded according to usage design.

## Models Endpoint

`GET /v1/models` returns enabled gateway model aliases visible to the authenticated key. It does not expose provider connection IDs, upstream credentials, route order, or internal health state.

Each item includes at minimum an OpenAI-compatible object identifier, model ID, creation metadata where meaningful, and owner value representing NexusRelay/organization policy. Unsupported OpenAI model retrieval/deletion endpoints are not exposed unless separately designed.

## Non-Streaming Flow

1. Authenticate key and resolve organization.
2. Decode and perform basic request-shape validation.
3. Consume RPM capacity.
4. Normalize the request, derive capability requirements, and enforce key/model restrictions that do not depend on a selected target.
5. Build a non-dispatching route plan with effective candidate prices and attempt slots.
6. Reserve TPM capacity and all applicable budgets against that immutable plan while persisting request start.
7. Durably mark each attempt before provider dispatch and execute within the overall deadline.
8. Normalize the successful provider response and persist attempt-level usage.
9. Finalize request totals, costs, and budgets.
10. Render the endpoint-compatible response.

If finalization temporarily fails after a successful upstream call, the gateway performs bounded retries. The client response policy must favor avoiding duplicate upstream generation: it does not retry the provider solely because persistence failed.

## Streaming Flow

### Phases

```text
pre-dispatch
awaiting-first-event
committed
completed
aborted
```

The gateway may fallback only in pre-dispatch or awaiting-first-event before any response content is committed. HTTP headers alone should be delayed until an eligible first normalized event is available when feasible.

### Commitment Point

The response is committed when any SSE event representing model output, tool call data, refusal, or endpoint-visible stream state is written to the client. Gateway-generated Responses lifecycle events are buffered until the selected upstream has produced the first valid event that establishes route viability; emitting a synthetic `created` event must not eliminate otherwise legal first-event fallback. After commitment:

- No provider switch is allowed.
- Upstream malformed events or failure terminate the stream using the closest protocol-compatible error behavior.
- The request records partial output status without storing output content.

### Backpressure

- Adapter readers feed a bounded event channel or synchronous iterator.
- The client writer controls consumption rate.
- Buffer limits are measured by event count and bytes.
- A slow client eventually triggers idle/write timeout and upstream cancellation.
- Complete upstream responses are never buffered for streaming translation.

### Completion

The gateway captures final usage events when providers emit them, persists attempt usage, finalizes request totals, and reconciles budgets before sending the protocol terminal event. For SSE protocols where a terminal event is optional, persistence still completes before the connection is deliberately closed. If provider-reported usage is unavailable, finalization uses an explicit conservative estimate; it does not defer hard-budget reconciliation until after stream success is signaled. If finalization retries are exhausted after commitment, the stream closes without a success terminal marker and the request remains eligible for conservative worker reconciliation.

### Client Disconnect

Request context cancellation propagates to the provider HTTP request. The attempt records `client_cancelled` separately from provider timeout. Budget reservation reconciles to known usage or a conservative estimate if provider billing may have occurred.

## Timeout Model

Timeouts are independently configurable with validated relationships:

- Connection timeout: establish TCP/TLS/provider connection.
- First-byte timeout: wait for provider response headers/first event.
- Idle-stream timeout: maximum silence between upstream events and separately bounded client-write progress.
- Total request deadline: all attempts, fallback delay, and upstream execution.

The effective deadline is the minimum of deployment policy, gateway model policy, API key policy if introduced, and client context. Each fallback receives only remaining time.

## Error Envelope

Public errors use an OpenAI-compatible structure with NexusRelay extensions that do not reveal internals:

```json
{
  "error": {
    "message": "The requested model is not available for this API key.",
    "type": "invalid_request_error",
    "param": "model",
    "code": "model_not_allowed",
    "request_id": "req_..."
  }
}
```

Stable internal categories map to one exact HTTP status and OpenAI-style `type` value. The normative V1 mapping is:

| Category | HTTP |
| --- | --- |
| `authentication_error` | 401 |
| `permission_denied` | 403 |
| `invalid_request` | 400 |
| `model_not_found` | 404 |
| `model_not_allowed` | 404 |
| `gateway_rate_limited` | 429 |
| `budget_exceeded` | 429 |
| `provider_rate_limited` | 429 |
| `provider_unavailable` | 503 |
| `request_timeout` | 504 |
| `content_filtered` | 400 |
| `upstream_error` | 502 |
| `internal_error` | 500 |

The compatibility matrix defines exact `type`, `code`, `param`, retry-header, and streaming-terminal behavior. Implementations must not choose alternative statuses.

## Retry and Idempotency Inputs

The protocol layer identifies whether request semantics are safe for fallback, but the router owns execution. Non-streaming generation may be retried only before returning output and within policy. Streaming cannot retry after commitment. Embedding requests are normally safe to retry before response return. Client-provided idempotency keys are not a V1 guarantee unless explicitly added later.

## Compatibility Matrix

Each endpoint maintains a version-controlled matrix with the same statuses used by `13-api-compatibility-matrix.md`:

```text
supported
capability-gated
constrained
rejected
not in V1
```

The matrix covers request fields, response fields, stream events, tool semantics, structured output, usage, and errors. Provider capability does not automatically make a public field supported; normalization and rendering must also preserve its semantics.

## Content Privacy

- Request bodies are handled in memory only by default.
- Access logs record body size, not body content.
- Traces use operation/size/capability metadata, never content.
- Panic/error recovery does not serialize request objects.
- Debug mode cannot enable model-content logging without a separate explicit future privacy design.
- Tool names may be considered metadata, but tool arguments and results are always sensitive; V1 should avoid storing names unless needed.

## Verification

- Official OpenAI SDK compatibility tests per endpoint.
- Golden protocol translation tests that use synthetic non-sensitive fixtures.
- Unknown/unsupported field and range validation tests.
- SSE incremental delivery, backpressure, cancellation, disconnect, malformed event, and timeout tests.
- Tests proving no fallback after commitment.
- Request-ID propagation and error sanitization tests.
- Body-size and decompression-bomb protections if compressed requests are accepted.

## Requirement Coverage

This design satisfies FR-API-001 through FR-API-014, FR-MODEL-007, the normalized error requirements in Section 10, NFR-003, NFR-004, NFR-008, OBS-005, DATA-001, TEST-006, TEST-007, and the OpenAI SDK release criteria.
