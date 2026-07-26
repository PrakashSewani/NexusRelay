# ADR 0004: Control-Plane Cryptographic Trust Boundary

- Status: Accepted
- Date: 2026-07-26

## Context

Provider creation and rotation must encrypt credentials in the same transaction that stores connection metadata, audit history, and invalidation events. API-key creation must hash the generated plaintext before the one-time response is returned. Moving either operation through an asynchronous worker would complicate transactionality, error handling, and one-time secret semantics.

AES-256-GCM provider encryption is symmetric. A process that can encrypt with the provider master key can also decrypt cryptographically, even when product behavior permits decryption only in narrow workflows.

## Decision

- The control-plane process is part of the trusted cryptographic boundary.
- Control plane receives `MASTER_KEYRING_FILE` and `API_KEY_PEPPER_RING_FILE` through protected read-only secret mounts.
- Provider creation, provider secret rotation, unsaved provider-test submission, and API-key creation perform cryptographic work synchronously before committing their documented PostgreSQL transaction.
- Administrative read APIs never decrypt or return provider credentials.
- Provider decryption in control plane is restricted to reviewed credential validation, replacement, and one-time test-envelope workflows. Ordinary provider reads operate only on metadata.
- Gateway receives both rings because it authenticates API keys and constructs provider clients. Worker receives only rings required by enabled jobs and reconciliation duties.
- Services validate required ring formats and active key references at startup and fail with non-secret errors.
- Process memory is considered sensitive. Logs, traces, metrics, panic recovery, and diagnostics must not serialize plaintext secret-bearing structures.

## Alternatives

### Worker-Owned Provider Encryption

Rejected for V1 because asynchronous encryption would split provider creation across transactions or require a more complex pending-resource state machine. It would not remove the API-key pepper requirement from control plane.

### Dedicated Cryptographic Service

Deferred because it adds a new service, authentication boundary, availability dependency, and deployment burden before scale or compliance requirements justify it.

### Public-Key Wrapping in Control Plane

Deferred because provider credential rotation and validation still require a carefully designed decryption authority and key-management lifecycle.

## Consequences

- Compromise of control plane can expose provider credentials and create gateway keys, so it receives the same hardening, secret-mount, logging, and memory-handling scrutiny as gateway and worker.
- Authorization and API separation, not asymmetric cryptography, prevent ordinary administrative reads from exposing credentials.
- The design preserves atomic provider/key creation and one-time secret behavior without adding a runtime service.
