# OpenAI SDK Compatibility Fixtures

## Purpose and Authority

This document pins the official OpenAI SDK compatibility baseline required by `docs/design/05-inference-protocol.md`, `docs/design/11-operations-security-testing.md`, and `docs/design/13-api-compatibility-matrix.md`. It covers only the V1 public matrix. The fixtures do not widen support to OpenAI fields, endpoints, or lifecycle operations that the matrix rejects or excludes.

Provider fixtures and public SDK fixtures have different boundaries:

- Provider contract fixtures test NexusRelay-to-provider translation and belong with the provider adapter.
- Phase 0 expected request shapes record the intended method, path, and supported JSON fields for each pinned official SDK scenario. They are provisional planning evidence, not captured observations or a normative definition of the public request contract.
- Wire response goldens under `docs/testing/fixtures/openai-sdk/` are normative for NexusRelay's public HTTP renderer, subject to the status and field rules in the compatibility matrix.

All model input, model output, refusal text, tool arguments, IDs, and metadata in this fixture set are synthetic `NR_SENTINEL_*` values. They are not copied from real users or live providers.

## Pinned SDKs

| Language | Exact dependency | Integrity and runtime requirement |
| --- | --- | --- |
| Python | `openai==2.48.0` | Wheel SHA-256 `c98df30aaaf93c51979f64d3e7c5b76464f8be0173368266229eb8fe6bd30f2c` |
| JavaScript | `openai@6.49.0` | npm integrity `sha512-aYCc0C6L864eR6WSYIwQGyXriw/nIyZx0ObvhzOEVuk0zoBDpynjSbrionWI7q65B5H8jJX0DXR9snEzM6bfPg==`; do not use a caret, tilde, tag, or range |
| Go | `github.com/openai/openai-go/v3@v3.46.0` | Exact module version; requires Go `1.25.0` |

The future isolated compatibility harness must commit its Python hash lock, npm lockfile, and `go.sum`. A version update is a reviewed compatibility change: regenerate captured request observations, replay every wire fixture through every SDK, and review the diff before changing these pins.

Dependency acquisition examples for the future isolated harness are:

```sh
python3 -m pip download --only-binary=:all: --no-deps openai==2.48.0
shasum -a 256 openai-2.48.0-py3-none-any.whl
npm install --save-exact openai@6.49.0
go mod edit -go=1.25.0
go get github.com/openai/openai-go/v3@v3.46.0
```

The Python digest output must equal the value above. npm and Go integrity must additionally be enforced by their committed lock/checksum files once the harness exists.

## Deterministic Local Architecture

Compatibility tests run without OpenAI credentials or outbound network access:

```text
pinned official SDK runner
        |
        | HTTP to a loopback base URL
        v
real NexusRelay gateway HTTP boundary
        |
        | normal validation, normalization, routing, rendering, and cancellation
        v
deterministic local provider mock
```

The fixture environment must create one isolated organization, one synthetic gateway key, and explicit aliases `nr-chat-sentinel`, `nr-responses-sentinel`, and `nr-embedding-sentinel`. The key and mock credential are fixture-only constants that are rejected by privacy-sentinel scans if they appear outside harness configuration. The environment must fix clock values, gateway request IDs, completion/response/item/call IDs, route choice, provider usage, and mock event timing. It must disable retries except where a test explicitly exercises pre-commit fallback.

The local provider mock is transport-real: it accepts HTTP, checks only synthetic expected requests, and emits deterministic bounded provider responses. The SDK must call the gateway rather than a fixture server directly. This preserves coverage of authentication, request parsing, capability derivation, public error mapping, incremental SSE rendering, durable-finalization gates, and client cancellation. Provider-specific translation assertions remain in the separate provider contract suite.

In the real gateway suite, each language runner performs the same semantic scenarios for the endpoints implemented in that phase. Phase 6 covers Models and Chat; Phase 10 adds Responses and Embeddings. The capture proxy at the gateway boundary records only method, path, content type, and parsed supported body fields. It removes authorization, user agent, SDK telemetry, retry headers, multipart boundaries, transfer framing, connection details, and other volatile transport metadata before comparison with the reviewed captured replacement for `requests/sdk-request-observations.json`.

## Fixture Classes

`docs/testing/fixtures/openai-sdk/manifest.json` is the concise inventory and expected HTTP metadata.

- `requests/sdk-request-observations.json` contains provisional expected, sanitized request shapes. These are non-normative and must be replaced by captured evidence when the real gateway/capture harness is scaffolded.
- `models/*.json`, `chat/*.json`, `responses/*.json`, and `embeddings/*.json` are exact non-stream public response bodies.
- `chat/*.sse`, `responses/*.sse`, and `failures/*.sse` are exact public SSE bytes.
- `errors/precommit.responses.jsonl` contains one exact pre-commit error body per LF-terminated line; the manifest assigns status and headers to each line.
- The Phase 0 error manifest contains representative authentication, validation/capability, Chat post-commit EOF, and Responses post-commit failure shapes. Phase 6 expands Models and Chat to every reachable public gateway code/HTTP pairing, including malformed JSON, media type, body limit, model denial, rate/budget/provider retry metadata, timeout, content filtering, sanitized malformed/oversized upstream failure, finalization failure, and internal failure without embedding raw upstream bodies. Phase 10 adds the corresponding exhaustive Responses and Embeddings cases, including rejected Responses `stop`, without embedding raw upstream bodies.
- `failures/cancellation.json` defines deterministic client actions against prefixes of existing stream fixtures. Cancellation has no fabricated server terminal event.

## Exact Wire Rules

All committed fixture files use UTF-8 without a byte-order mark and LF line endings. JSON files use two-space indentation, preserve the committed key order, contain no insignificant trailing spaces, and end with one LF. JSONL records are compact JSON, one record per LF-terminated line. SSE JSON is compact on one `data:` line; events are separated by exactly one empty LF line. Responses SSE includes an `event:` line matching the JSON `type`; Chat SSE uses only `data:` lines.

Tests compare response body bytes exactly after transport decoding. They do not reformat JSON before comparison. HTTP framing, chunk sizes, header order, and HTTP version are not golden. `Content-Type`, status, `x-request-id`, and permitted retry headers are asserted separately from manifest metadata.

The exact null and omission policy is:

- A field is omitted unless the public matrix supports it and the normalized renderer has a value or OpenAI-compatible lifecycle semantics require an explicit null.
- No provider fingerprint, provider model ID, service tier, provider request ID, route detail, estimate, or internal accounting field is synthesized.
- Chat tool-call messages use `content: null`. Chat stream deltas omit fields that do not change. With requested usage, every ordinary Chat chunk has `usage: null`; the final usage chunk has `choices: []` and a non-null provider-reported usage object.
- Chat usage detail objects are omitted when the provider did not report those dimensions. Missing detail is not rendered as zero.
- Successful Responses objects use `error: null` and `incomplete_details: null`. Created and in-progress lifecycle response objects use `usage: null`. Completed objects contain provider-reported usage.
- Responses usage detail objects are present in these goldens because the deterministic provider explicitly reports cached and reasoning token counts as zero. This is reported zero, not unavailable.
- Response stream `sequence_number` starts at `0` and increments by exactly one in emitted gateway events. This is a NexusRelay public rendering rule; the OpenAI provider adapter does not assume upstream sequences start at zero or are contiguous.
- Response, output item, call, completion, and model IDs remain byte-for-byte stable within a scenario. Gateway aliases are rendered instead of upstream model identifiers.
- A successful Chat stream ends with exactly `data: [DONE]\n\n`, and only after durable finalization. A successful Responses stream ends with `response.completed` and has no Chat-style `[DONE]` marker.

## Failure and Cancellation Rules

Pre-commit failures use JSON, the manifest HTTP status, `Content-Type: application/json`, and the same gateway-generated request ID in `x-request-id` and `error.request_id`. `error.param` is omitted when there is no safe client field path; it is never emitted as `null` in these fixtures. Retry headers are absent for the committed pre-commit cases.

After Chat SSE commitment, the normative fixture closes without `[DONE]` and without a success usage chunk. `docs/design/13-api-compatibility-matrix.md` permits a sanitized JSON error `data` event only if pinned SDK evidence proves all supported runners accept it. No such evidence has been captured yet, so this fixture set does not emit that optional event.

After Responses SSE commitment, a terminal gateway failure emits one `response.failed` event and closes. It never emits `response.completed` afterward. The failed response carries a sanitized public error and `usage: null` because no provider-reported final usage is available.

For client cancellation, the harness writes the configured prefix, waits until the SDK has observed that prefix, then cancels or closes the SDK request context. No bytes after cancellation are asserted because the client controls connection lifetime. The required assertions are that no success terminal was observed, the provider request was cancelled, no fallback occurred after commitment, and accounting recorded `client_cancelled` with provider-reported usage or a conservative estimate outside the public response.

## Scenario Assertions

Every pinned SDK must prove:

- Models list deserializes and exposes only the gateway aliases in the golden.
- Chat non-stream deserializes one function call, preserves the raw synthetic argument string, returns `finish_reason: "tool_calls"`, and exposes provider-reported usage.
- Chat stream incrementally assembles the indexed function call and arguments, observes the finish chunk and final usage chunk in order, and reaches `[DONE]` only on success.
- Responses non-stream deserializes a completed text output and provider-reported usage without requiring stored-response operations.
- Responses stream observes the exact created, in-progress, item-added, argument-delta, item-done, completed lifecycle and monotonic sequence numbers.
- Embeddings preserves input order/index and floating-point vectors and exposes prompt/total token usage.
- Pre-commit errors are surfaced by each SDK with the expected status and parseable OpenAI-style error body, including the NexusRelay code and request ID where the SDK exposes response data.
- Every error scenario retains byte-level body plus status/header assertions at the harness boundary even if an SDK hides response fields, retries automatically, or maps multiple statuses to one exception class. Runners must disable automatic retries except in an explicit retry-observation case and must prove no credential, provider body, route detail, or model content appears in exceptions or diagnostics.
- Chat post-commit failure and cancellation are reported as interrupted/incomplete streams, never as successful streams.
- Responses post-commit failure exposes `response.failed` before close; cancellation observes no synthetic failed or completed event after the client closes.

SDK exception class names and convenience-property layouts are observations, not public wire requirements. Tests may assert them in language-specific runner code but must retain the raw status, headers, and body assertion as the stable cross-language evidence.

## Future Verification Harness

Phase 1 establishes the repository test/tooling conventions. The official-SDK harness is implemented with the gateway surface it verifies rather than as Phase 0 executable code. It must:

- Commit exact Python transitive hashes, an npm lockfile, and `go.sum` without adding SDK dependencies to runtime modules.
- Validate fixture inventory and checksums, UTF-8/LF and canonical JSON/JSONL/SSE formatting, privacy sentinels, event ordering, terminal behavior, and cancellation references.
- Serve deterministic provider behavior through the real NexusRelay gateway rather than replaying public responses directly to the SDK.
- Capture sanitized SDK request shapes and replace the provisional request-shape file through reviewed regeneration.
- Deny non-loopback test traffic and require no OpenAI credentials.

Because the gateway and deterministic provider mock do not exist in Phase 0, the committed fixtures do not yet prove request capture through NexusRelay, gateway validation/routing/rendering, status and header behavior, SDK parsing of pre-commit errors, post-commit failure exceptions, cancellation propagation/accounting, non-loopback network denial, or byte-stable regeneration. Phase 6 proves those properties for Models and Chat; Phase 10 extends them to Responses and Embeddings. Fixture inspection or direct replay must not be described as gateway compatibility.

## Known Compatibility Boundary

The optional Chat post-commit JSON error event remains intentionally unclaimed until all three pinned SDK runners provide evidence that it is accepted without being misclassified as a normal chunk. Chat therefore uses EOF without `[DONE]` for the committed baseline. Embeddings `encoding_format: "base64"` also remains capability-gated and has no golden until exact normalized byte encoding is approved and verified. Fields and endpoints marked rejected or not in V1 have no success goldens.
