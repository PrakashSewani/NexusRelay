# Providers and Secrets Design

## Scope

This design defines provider connection configuration, encrypted credentials, adapter registration, capability declarations, connection testing, model discovery, outbound URL security, and provider-specific extension rules.

## Provider Types and Connections

A provider type is implementation metadata compiled into the application. A provider connection is organization-owned runtime configuration.

Initial provider type keys:

```text
openai
anthropic
google_gemini
openrouter
ollama
groq
xiaomi_mimo
commandcode
openai_compatible
```

OpenRouter, Groq, and custom OpenAI-compatible connections should use the shared OpenAI-compatible adapter unless verified behavior requires a dedicated wrapper/configuration profile. Provider type keys are stable persisted values.

## Provider Connection Data

```text
provider_connections
  id                    uuid primary key
  organization_id       uuid not null
  provider_type         text not null
  name                  text not null
  status                text not null  -- enabled, disabled
  base_url              text null
  timeout_config        jsonb not null
  non_secret_config     jsonb not null
  secret_version        bigint not null default 0
  config_version        bigint not null default 1
  created_at            timestamptz not null
  created_by            uuid not null
  updated_at            timestamptz not null
  updated_by            uuid not null
  unique (organization_id, name)
```

`non_secret_config` follows a versioned schema selected by provider type. Examples include API version, organization/project identifier, custom model allowlist, optional non-secret header names, and private-network policy reference. Arbitrary headers are classified as secret unless explicitly proven non-sensitive.

Credential material is stored separately:

```text
provider_secrets
  id                    uuid primary key
  organization_id       uuid not null
  provider_connection_id uuid not null unique
  envelope_version      smallint not null
  key_id                text not null
  nonce                 bytea not null
  ciphertext            bytea not null
  created_at            timestamptz not null
  rotated_at            timestamptz null
```

The plaintext is a small versioned JSON object understood only by the relevant adapter configuration decoder. It may contain API keys, authorization headers, or provider-specific credentials.

## Encryption Envelope

V1 uses application-level AES-256-GCM authenticated encryption through Go's standard cryptographic library:

- The mounted key-ring file contains one active 256-bit key and zero or more decrypt-only 256-bit keys, each identified by a unique non-secret `key_id`.
- Key material is base64-encoded in the mounted secret format, decoded into fixed-size keys at startup, and never stored in PostgreSQL.
- Envelope version `1` uses a fresh cryptographically secure 96-bit nonce for every encryption operation. Nonce reuse with one key is prohibited.
- Additional authenticated data is the canonical length-prefixed encoding of envelope version, organization UUID, provider connection UUID, and provider type. It is reconstructed exactly for decryption.
- Ciphertext stores the GCM authentication tag as produced by the standard AEAD seal operation.
- `key_id` selects the decrypt key without exposing key material.
- Decryption/authentication failure is terminal for that connection and emits a sanitized operational alert.

Rotation procedure:

1. Add a new unique key as active and retain prior keys as decrypt-only.
2. Deploy/restart control-plane, all gateway replicas, and every worker that performs provider-secret jobs with the complete ring and the same active key ID before re-encryption begins.
3. Worker claims credentials in bounded batches, decrypts with the recorded key, re-encrypts with the active key and fresh nonce, and updates only when the credential version/key ID still matches.
4. Verify no credential rows reference an old key and perform a restore/decryption test.
5. Remove an old key only after the verification and backup-retention window completes.

The application refuses startup for duplicate key IDs, invalid key lengths, no active key, multiple active keys, or unknown envelope versions. Database backups are useless for provider secrets without separately protected key-ring backups.

The V1 mounted key-ring is UTF-8 JSON with this exact shape:

```json
{
  "version": 1,
  "active_key_id": "provider-2026-01",
  "keys": [
    {"key_id": "provider-2026-01", "key": "<base64-32-bytes>"}
  ]
}
```

`version` must be `1`. `active_key_id` must name exactly one entry; all other entries are decrypt-only. `key_id` is a non-empty ASCII identifier matching `[A-Za-z0-9][A-Za-z0-9._-]{0,63}`. `key` uses canonical padded base64 and must decode to exactly 32 bytes. Unknown fields, duplicate fields, duplicate key IDs, invalid UTF-8, and trailing JSON values are rejected. The generic secret-file newline and size rules are defined in `11-operations-security-testing.md`.

## Secret Lifecycle

### Create

1. Validate provider type, non-secret settings, URL policy, and required secret fields.
2. Encrypt the secret payload in process memory.
3. Insert connection, secret, audit event, and invalidation outbox event in one transaction.
4. Clear references to plaintext as soon as practical; Go cannot guarantee memory zeroization, so plaintext lifetime and copies are minimized.
5. Return connection metadata and `has_secret: true`, never secret values.

The control plane performs encryption synchronously and therefore receives the provider master ring as defined by ADR 0004. This makes control plane part of the trusted cryptographic boundary; ordinary read handlers still have no code path that decrypts credentials.

### Update

- Non-secret changes do not require resubmitting credentials.
- Secret rotation is an explicit operation that replaces the envelope and increments `secret_version`.
- `provider_type` is immutable after creation because it participates in credential validation and AEAD associated data. Changing provider type creates a new connection.
- Empty/missing secret input means retain existing secret, not clear it.
- Credential removal is allowed only if the provider type can operate without credentials; otherwise validation rejects it.

### Read

- Control-plane responses expose only secret presence, last rotation time, and key metadata suitable for administrators.
- Gateway/worker adapter construction and reviewed control-plane credential create/rotate/test-envelope workflows may decrypt provider secrets. Administrative read APIs cannot decrypt or return them.
- Decrypted credentials are never placed in shared caches, Redis, traces, errors, or audit metadata.

## Adapter Registry

The application has a compile-time registry mapping provider type keys to factories. Conceptual contracts:

```text
ProviderFactory
  Type() ProviderType
  ValidateConfig(nonSecret, secret) ValidationResult
  Capabilities(config) CapabilitySet
  NewClient(connection, decryptedSecret, outboundPolicy) ProviderClient

ProviderClient
  TestConnection(ctx) ConnectionTestResult
  DiscoverModels(ctx, cursor) ModelPage
  Chat(ctx, NormalizedChatRequest) NormalizedChatResponse
  ChatStream(ctx, NormalizedChatRequest) Stream
  Responses(ctx, NormalizedResponseRequest) NormalizedResponse
  ResponsesStream(ctx, NormalizedResponseRequest) Stream
  Embed(ctx, NormalizedEmbeddingRequest) NormalizedEmbeddingResponse
```

Interfaces in code should be narrower and defined near consumers; this list documents required behavior rather than prescribing one broad Go interface.

Every operation accepts context, obeys deadlines, closes response bodies, normalizes errors, and supplies upstream request ID and usage where available.

## Capability Model

Capabilities are explicit booleans and limits rather than inferred from provider name:

```text
operations: chat_completions, responses, embeddings
transport: streaming
input: text, image, audio
output: text, image, audio
features: tools, parallel_tools, structured_output, json_mode, logprobs, reasoning
usage: input_tokens, output_tokens, cached_tokens, provider_cost
limits: context_tokens, max_output_tokens, max_input_items, max_embedding_inputs
```

Capabilities have sources:

- Adapter static capability.
- Provider-discovered model metadata.
- Administrator override where safe.

An override may narrow capabilities but must not claim adapter support that translation cannot provide. Effective capability is the intersection of adapter, discovered/configured model, and administrator constraints.

## Shared OpenAI-Compatible Adapter

The shared adapter supports configuration profiles for:

- Base URL and endpoint prefix.
- Bearer or custom authentication header.
- Additional secret and non-secret headers.
- Chat, Responses, Models, and Embeddings endpoint availability.
- Usage and streaming dialect quirks that are verified and explicitly modeled.
- Static configured model IDs when model listing is unavailable.

Compatibility is not assumed from marketing claims. A provider profile must pass deterministic contract tests before being treated as supported.

Configurable headers accept only RFC-valid names and values. Control characters and `Host`, `Content-Length`, `Transfer-Encoding`, `Connection`, `Proxy-Authorization`, proxy/forwarding headers, and all hop-by-hop or transport-owned headers are rejected. Host routing and TLS SNI derive only from the validated base URL.

## Dedicated Adapters

Anthropic and Gemini require dedicated translation where their native payloads, streaming events, tool semantics, usage, or errors differ materially. Xiaomi MiMo and CommandCode receive dedicated or shared adapters only after authoritative documentation is captured in provider-specific design notes.

Provider SDKs may be used internally when they improve correctness, but SDK types must not cross the adapter boundary. Direct HTTP is preferred when SDK behavior obscures streaming, retries, request bodies, or error handling.

## Outbound URL and SSRF Policy

### URL Validation

- Allowed schemes are `https` by default; `http` is allowed only for connections explicitly marked as private/local, such as Ollama.
- URLs must not contain userinfo credentials.
- Fragments are rejected.
- Query strings are rejected in V1. A verified provider that requires query authentication must receive a typed secret field in a future profile rather than embedding it in the base URL.
- Base paths are normalized and endpoint joining prevents path escape.
- Redirects are disabled by default. If enabled for a verified provider, every redirect target is revalidated and the redirect count is bounded.
- Proxy environment variables are ignored unless deployment configuration explicitly enables an outbound proxy.

### DNS and Address Validation

Before connection:

1. Resolve all target addresses with a controlled resolver.
2. Reject loopback, link-local, multicast, unspecified, carrier-grade NAT, private, and reserved ranges under the public policy.
3. For a private-network connection, every resolved address must belong to the named policy's configured CIDRs. Configured hostnames identify intended destinations but do not bypass address-range checks.
4. Dial a validated resolved address while preserving TLS server name and HTTP Host semantics.
5. Revalidate on DNS refresh and do not trust a previous result indefinitely.

This mitigates DNS rebinding between validation and connection. Connection pools have bounded DNS lifetimes and are rebuilt when connection configuration changes.

### Ollama

Ollama commonly runs on a private address. The administrator must opt the connection into a named private-network policy. The UI clearly identifies the increased SSRF trust and restricts who may configure it through provider-management permission.

## Connection Testing

A test operation:

- Requires `providers.test`; secret rotation separately requires `providers.rotate_secret`.
- Is submitted by control-plane as a durable `provider_test_jobs` row plus encrypted one-time secret envelope when unsaved credentials are involved. The worker, which owns provider egress and adapter construction, claims the job under organization RLS context.
- Encrypts one-time unsaved credentials with the provider master key, binds them to job/organization/provider type as AAD, and deletes the envelope after terminal completion/expiry. Credentials never travel over an ad hoc internal HTTP API.
- Performs the cheapest authoritative operation available, typically model listing or provider metadata.
- Uses a strict test timeout independent of inference timeout.
- Persists success, latency, normalized error category/code, safe provider metadata, and expiry. The control-plane endpoint returns `202 Accepted`; UI polls the job or uses a bounded event channel.
- Stores the submission audit event atomically with job creation and stores a health observation/terminal audit event on completion without raw upstream response.
- Is rate limited to prevent abuse as a network scanner.

Tests never make a provider eligible by themselves; status and routing configuration remain explicit.

## Atomic Secret Rotation

Provider credential rotation and online master-key re-encryption use one concurrency contract:

1. Start a tenant transaction and lock the provider connection and secret row.
2. Compare the expected connection `version`, `secret_version`, and current envelope `key_id`.
3. Validate/decrypt where required, write the replacement envelope, increment `secret_version`, update `rotated_at`, and increment connection version.
4. Insert the audit event and critical invalidation outbox event in the same transaction.
5. Commit before returning success. Batch re-encryption skips a row when any compared value changed and retries from fresh state; it never overwrites an administrator rotation.

## Model Discovery

- Discovery is optional per adapter.
- Discovered models are suggestions/snapshots, not automatically public gateway models.
- Discovery stores safe model identifiers and declared metadata with source and observed timestamp.
- Removal from a later discovery result does not silently delete route targets; it marks discovery state stale/unavailable for administrator review.
- Static model entry remains available for providers without discovery endpoints.

## Error Normalization

Adapters classify HTTP status, transport error, provider error code, and response phase into normalized categories. Raw bodies are read through a small bounded buffer for parsing, discarded after classification, and never included in public errors or normal logs.

Retry hints include:

```text
retryable_before_commit
retry_after
provider_rate_limit_scope
authentication_failure
content_filter
invalid_request
```

The router, not the adapter, decides whether a retry/fallback is permitted.

## Provider Observability

Safe metadata includes provider type, connection ID, upstream model ID, operation, status class, normalized error category, latency, and upstream request ID if safe. Metrics avoid connection ID labels if cardinality is unbounded; detailed connection dimensions remain in structured records and traces under controlled sampling.

## Provider Verification Deliverables

Each supported provider requires:

- Authoritative documentation links and retrieval date.
- Authentication and endpoint specification.
- Capability matrix.
- Request/response and streaming translation notes.
- Error and rate-limit mapping.
- Usage/cost behavior.
- Mock-server contract fixtures containing no real secrets or user content.
- Optional live smoke-test instructions.

Xiaomi MiMo and CommandCode remain blocked from implementation until these deliverables exist.

## Verification

- Encryption/decryption, tamper detection, wrong-AAD, and key rotation tests.
- Secret absence from API output, logs, audit, Redis, and error paths.
- SSRF tests for redirects, DNS rebinding simulation, IPv4/IPv6 private ranges, encoded hosts, and path joining.
- Header-smuggling/forbidden-header tests, immutable provider-type tests, and concurrent administrator-rotation/master-key-re-encryption tests.
- Adapter cancellation, deadline, body-close, malformed response, streaming, and error normalization tests.
- Connection test abuse limits and redaction tests.
- Capability narrowing and unsupported-feature rejection tests.

## Requirement Coverage

This design satisfies FR-PROV-001 through FR-PROV-011, the provider adapter contract in Section 9, SEC-002, SEC-006 through SEC-008, SEC-015, SEC-019, and relevant provider release acceptance criteria.
