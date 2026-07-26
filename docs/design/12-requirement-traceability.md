# Requirement Traceability

## Purpose

This matrix maps requirement groups from `docs/requirements.md` to their primary low-level designs and verification methods. A mapping means design ownership is identified; it does not by itself mean provider research, ADRs, implementation, or verification are complete. Many cross-cutting requirements appear in more than one design; the primary design owns detailed behavior while supporting designs own enforcement at their boundaries.

## Design Maturity

| Area | Status | Remaining gate |
| --- | --- | --- |
| Topology, persistence, tenancy, identity | Contract revised; implementation-ready after consistency review | Database-role and Redis protocol tests |
| Gateway protocol baseline | Draft contract | Exact stream fixtures, pinned SDK versions, provider-specific constraints |
| Models, routing, keys, limits, budgets | Contract revised | Concurrency/load evidence and Redis limiter ADR |
| Usage, workers, analytics, audit | Contract revised | Exact analytics schema migration and scale validation |
| Control plane and web | Draft contract | Authoritative OpenAPI schema and generated-client verification |
| Operations and tests | Framework complete | Reproducible backup, benchmark, alert, and recovery profiles |
| Provider adapters | Blocked pending profiles | Authoritative provider notes in [14-provider-verification.md](14-provider-verification.md) |
| Agent exporters | Framework complete; OpenCode profile pending verification | [15 Agent Config Export](15-agent-config-export.md) and `docs/agents/*` profiles |

## Functional Requirements

| Requirement group | Primary design | Supporting design | Primary verification |
| --- | --- | --- | --- |
| FR-ORG-001 through FR-ORG-006 | [03 Identity and Access](03-identity-access.md) | [02 Persistence and Tenancy](02-persistence-tenancy.md) | Multi-organization integration tests, RLS denial, operator-only organization creation |
| FR-AUTH-001 through FR-AUTH-010 | [03 Identity and Access](03-identity-access.md) | [10 Control Plane and Web](10-control-plane-web.md), [11 Operations](11-operations-security-testing.md) | Login/session/CSRF/self-revocation/org-only access removal/bootstrap tests |
| FR-RBAC-001 through FR-RBAC-008 | [03 Identity and Access](03-identity-access.md) | [10 Control Plane and Web](10-control-plane-web.md), [02 Persistence](02-persistence-tenancy.md) | Own/all permission matrix, final-owner concurrency, cross-tenant denial |
| FR-PROV-001 through FR-PROV-011 | [04 Providers and Secrets](04-providers-secrets.md) | [09 Workers and Health](09-workers-health-analytics-audit.md), [10 Control Plane](10-control-plane-web.md) | Adapter contracts, encryption/redaction, SSRF, provider admin E2E |
| FR-MODEL-001 through FR-MODEL-009 | [06 Models and Routing](06-models-routing.md) | [05 Inference Protocol](05-inference-protocol.md), [10 Control Plane](10-control-plane-web.md) | Model CRUD, key-filtered `/v1/models`, target/capability tests |
| FR-API-001 through FR-API-014 | [05 Inference Protocol](05-inference-protocol.md), [13 Compatibility Matrix](13-api-compatibility-matrix.md) | [01 Topology](01-system-topology.md), [06 Routing](06-models-routing.md), [11 Operations](11-operations-security-testing.md) | Official SDK, protocol contracts, SSE/cancellation/timeout tests |
| FR-KEY-001 through FR-KEY-012 | [07 API Keys and Rate Limits](07-api-keys-rate-limits.md) | [03 Identity](03-identity-access.md), [10 Control Plane](10-control-plane-web.md) | One-time key, hash/redaction, restrictions, immediate authority reduction |
| FR-ROUTE-001 through FR-ROUTE-012 | [06 Models and Routing](06-models-routing.md) | [04 Providers](04-providers-secrets.md), [08 Usage and Budgets](08-usage-pricing-budgets.md) | Eligibility/ranking determinism, fallback/deadline/commit tests |
| FR-HEALTH-001 through FR-HEALTH-007 | [09 Workers and Health](09-workers-health-analytics-audit.md) | [06 Models and Routing](06-models-routing.md), [04 Providers](04-providers-secrets.md) | Probe/passive observation, state/hysteresis/freshness tests |
| FR-USAGE-001 through FR-USAGE-011 | [08 Usage, Pricing, and Budgets](08-usage-pricing-budgets.md) | [02 Persistence](02-persistence-tenancy.md), [09 Analytics](09-workers-health-analytics-audit.md) | Request lifecycle, per-dimension source, non-overlapping billing, filters, privacy, finalization tests |
| FR-BUDGET-001 through FR-BUDGET-010 | [08 Usage, Pricing, and Budgets](08-usage-pricing-budgets.md) | [09 Workers](09-workers-health-analytics-audit.md), [10 Control Plane](10-control-plane-web.md) | Arithmetic, timezone, concurrent reservations, warning/denial tests |
| FR-RATE-001 through FR-RATE-004 | [07 API Keys and Rate Limits](07-api-keys-rate-limits.md) | [11 Operations](11-operations-security-testing.md) | Multi-replica Redis atomicity, TPM reconciliation, outage tests |
| FR-ANALYTICS-001 through FR-ANALYTICS-006 | [09 Workers and Analytics](09-workers-health-analytics-audit.md) | [10 Control Plane and Web](10-control-plane-web.md), [08 Usage](08-usage-pricing-budgets.md) | Idempotent rollups, filters, freshness, UI state tests |
| FR-AUDIT-001 through FR-AUDIT-006 | [09 Workers and Audit](09-workers-health-analytics-audit.md) | [02 Persistence](02-persistence-tenancy.md), [03 Identity](03-identity-access.md) | Atomic mutation/audit, DB immutability, redaction/filter tests |
| FR-UI-001 through FR-UI-006 | [10 Control Plane and Web](10-control-plane-web.md) | [03 Identity](03-identity-access.md), [11 Operations](11-operations-security-testing.md) | Responsive, accessibility, form/error/permission E2E |
| FR-CONFIG-001 through FR-CONFIG-005 | [11 Operations](11-operations-security-testing.md) | [01 Topology](01-system-topology.md), [02 Persistence](02-persistence-tenancy.md) | Startup validation, secret-safe errors, invalidation tests |
| FR-EXPORT-001 through FR-EXPORT-012 | [15 Agent Config Export](15-agent-config-export.md) | [07 API Keys](07-api-keys-rate-limits.md), [10 Control Plane](10-control-plane-web.md) | Exporter registry, pinned agent schema/fixtures, key/model restriction and secret-separation tests |

## Provider Adapter Contract

| Requirement | Design ownership | Verification |
| --- | --- | --- |
| Configuration validation and connectivity test | [04 Providers and Secrets](04-providers-secrets.md) | Provider-specific validation and deterministic test-server contracts |
| Capability reporting and model listing | [04 Providers](04-providers-secrets.md), [06 Models](06-models-routing.md) | Capability intersection and discovery tests |
| Chat, Responses, and Embeddings translation | [05 Inference Protocol](05-inference-protocol.md), [13 Compatibility Matrix](13-api-compatibility-matrix.md), [04 Providers](04-providers-secrets.md) | Golden translation and official SDK compatibility tests |
| Incremental streaming | [05 Inference Protocol](05-inference-protocol.md) | Chunk/event, bounded buffering, slow client, cancellation tests |
| Error normalization | [04 Providers](04-providers-secrets.md), [05 Protocol](05-inference-protocol.md) | HTTP/transport/provider-code mapping tests |
| Usage and cost extraction | [04 Providers](04-providers-secrets.md), [08 Usage](08-usage-pricing-budgets.md) | Provider-reported/estimated source tests |
| Deadlines, cancellation, redaction | [04 Providers](04-providers-secrets.md), [11 Operations](11-operations-security-testing.md) | Timeout/disconnect and sentinel privacy tests |

## Error Requirements

The stable categories in requirements Section 10 are primarily designed in [05 Inference Protocol](05-inference-protocol.md). Provider classification originates in [04 Providers and Secrets](04-providers-secrets.md), routing exhaustion precedence in [06 Models and Routing](06-models-routing.md), and gateway policy errors in [07 API Keys](07-api-keys-rate-limits.md) and [08 Budgets](08-usage-pricing-budgets.md).

Verification consists of table-driven internal-to-public mappings, endpoint-specific contract tests, sanitized raw-upstream failures, and stable request-ID propagation.

## Security Requirements

| Requirement | Primary design | Verification |
| --- | --- | --- |
| SEC-001 TLS | [11 Operations](11-operations-security-testing.md) | Proxy configuration and deployment test |
| SEC-002 provider encryption | [04 Providers and Secrets](04-providers-secrets.md) | AEAD tamper/wrong-key/rotation tests |
| SEC-003 API key entropy | [07 API Keys](07-api-keys-rate-limits.md) | Generation/format/hash tests |
| SEC-004 CSRF | [03 Identity](03-identity-access.md) | Browser mutation/origin/token tests |
| SEC-005 parameterized SQL | [02 Persistence](02-persistence-tenancy.md) | `sqlc` use, query review, injection tests |
| SEC-006/007 SSRF and redirects | [04 Providers](04-providers-secrets.md) | URL/IP/DNS/redirect test matrix |
| SEC-008 redaction | [11 Operations](11-operations-security-testing.md) | Sentinel scan across logs/traces/errors/storage |
| SEC-009 non-root/minimal images | [11 Operations](11-operations-security-testing.md) | Image/runtime inspection |
| SEC-010 vulnerability scans | [11 Operations](11-operations-security-testing.md) | CI scan gates and SBOM |
| SEC-011 tenant negative tests | [02 Persistence](02-persistence-tenancy.md) | Two-tenant test suite for every resource |
| SEC-012 CORS | [10 Control Plane](10-control-plane-web.md), [11 Operations](11-operations-security-testing.md) | Origin/CORS response tests |
| SEC-013 security headers | [10 Control Plane and Web](10-control-plane-web.md) | Header/CSP browser tests |
| SEC-014 constant-time comparison | [07 API Keys](07-api-keys-rate-limits.md), [03 Identity](03-identity-access.md) | Code review and auth behavior tests |
| SEC-015 no outbound telemetry | [11 Operations](11-operations-security-testing.md) | Default configuration/network test |
| SEC-016 through SEC-018 ingress isolation/trust | [01 Topology](01-system-topology.md), [11 Operations](11-operations-security-testing.md) | Public/private hostname denial, forwarded-header, TLS, and tunnel rule tests |
| SEC-019 provider header safety | [04 Providers](04-providers-secrets.md) | Forbidden-header, control-character, Host/SNI derivation, and request-smuggling tests |

## Reliability and Performance Requirements

| Requirement | Design ownership | Verification |
| --- | --- | --- |
| NFR-001 stateless replicas | [01 Topology](01-system-topology.md) | Multi-replica Compose tests |
| NFR-002 graceful shutdown | [01 Topology](01-system-topology.md) | Active request/stream shutdown tests |
| NFR-003 bounded streaming | [05 Protocol](05-inference-protocol.md) | Memory/backpressure/slow-client tests |
| NFR-004/005 latency and load | [11 Operations](11-operations-security-testing.md) | Documented reference load suite |
| NFR-006 background failure isolation | [01 Topology](01-system-topology.md), [09 Workers](09-workers-health-analytics-audit.md) | Kill/failure-injection tests |
| NFR-007 migrations | [02 Persistence](02-persistence-tenancy.md) | Empty/previous schema and lock tests |
| NFR-008 bounded retries | [06 Routing](06-models-routing.md) | Attempts/deadline/backoff tests |
| NFR-009 Redis loss | [07 API Keys](07-api-keys-rate-limits.md), [11 Operations](11-operations-security-testing.md) | Redis outage/restart policy tests |
| NFR-010 PostgreSQL outage | [02 Persistence](02-persistence-tenancy.md) | Stage-specific database failure tests |

## Observability Requirements

OBS-001 through OBS-007 are primarily covered by [11 Operations, Security, and Testing](11-operations-security-testing.md). Worker-specific lag and health metrics are detailed in [09 Workers](09-workers-health-analytics-audit.md), request/attempt observability in [08 Usage](08-usage-pricing-budgets.md), and routing explanations in [06 Models and Routing](06-models-routing.md).

Verification includes metric cardinality checks, structured log schema tests, sentinel privacy scans, trace body/header capture tests, and liveness/readiness dependency behavior.

## Data Retention Requirements

DATA-001 is enforced by [05 Inference Protocol](05-inference-protocol.md) and [08 Usage](08-usage-pricing-budgets.md). DATA-002 through DATA-005 are designed in [09 Workers, Health, Analytics, and Audit](09-workers-health-analytics-audit.md). DATA-006 remains optional for V1 and therefore has no implementation design beyond preserving queryable source data.

## Testing Requirements

TEST-001 through TEST-010 are consolidated in [11 Operations, Security, and Testing](11-operations-security-testing.md), with subsystem-specific scenarios listed in every design document. CI must report the relevant test layer before a requirement is considered implemented.

## V1 Acceptance Criteria Mapping

| Acceptance outcome | Primary designs |
| --- | --- |
| Clean Docker Compose deployment | [01 Topology](01-system-topology.md), [11 Operations](11-operations-security-testing.md) |
| Configured public API URL and public-host route isolation | [01 Topology](01-system-topology.md), [11 Operations](11-operations-security-testing.md) |
| Configured administrative URL and exposure mode | [01 Topology](01-system-topology.md), [10 Control Plane](10-control-plane-web.md), [11 Operations](11-operations-security-testing.md) |
| Initial owner and secure login | [03 Identity](03-identity-access.md), [10 Control Plane](10-control-plane-web.md) |
| All target providers configurable | [04 Providers](04-providers-secrets.md) plus provider-specific verified notes tracked in [14 Provider Verification](14-provider-verification.md) |
| Gateway models and multiple targets | [06 Models and Routing](06-models-routing.md) |
| Restricted one-time API key | [07 API Keys](07-api-keys-rate-limits.md), [10 Control Plane](10-control-plane-web.md) |
| Schema-valid V1 OpenCode export through generic agent-export framework | [15 Agent Config Export](15-agent-config-export.md), [10 Control Plane](10-control-plane-web.md) |
| Official SDK model/chat/stream support | [05 Inference Protocol](05-inference-protocol.md), [11 Testing](11-operations-security-testing.md) |
| Chat/Responses routing across providers | [04 Providers](04-providers-secrets.md), [05 Protocol](05-inference-protocol.md), [06 Routing](06-models-routing.md) |
| Eligibility and bounded fallback | [06 Models and Routing](06-models-routing.md) |
| Restrictions, revocation, rates, budgets | [07 API Keys](07-api-keys-rate-limits.md), [08 Budgets](08-usage-pricing-budgets.md) |
| Usage/cost filters and analytics | [08 Usage](08-usage-pricing-budgets.md), [09 Analytics](09-workers-health-analytics-audit.md), [10 Web](10-control-plane-web.md) |
| Immutable redacted audit | [09 Audit](09-workers-health-analytics-audit.md) |
| No secrets/model content in observability | [04 Providers](04-providers-secrets.md), [05 Protocol](05-inference-protocol.md), [11 Operations](11-operations-security-testing.md) |
| Automated compatibility/security/load evidence | [11 Operations, Security, and Testing](11-operations-security-testing.md) |

## Design Completion Gates

A requirement is ready for implementation only when:

- Its primary design has no unresolved correctness or security ambiguity.
- Relevant data ownership and transaction boundaries are explicit.
- Public/control-plane contract behavior is explicit.
- Failure and concurrency behavior is explicit.
- Tests capable of proving the behavior are identified.
- Provider-specific facts are verified from authoritative documentation where applicable.
- Any consequential unresolved choice has an ADR rather than an implicit implementation decision.
