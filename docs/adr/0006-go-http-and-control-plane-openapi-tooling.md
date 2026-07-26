# ADR 0006: Go HTTP and Control-Plane OpenAPI Tooling

- Status: Accepted
- Date: 2026-07-26

## Context

NexusRelay needs one HTTP foundation for the Go gateway, control plane, and internal operational endpoints. The control-plane design also requires a versioned OpenAPI source contract that defines transport behavior and generates Go and TypeScript types without becoming the domain model or authorization boundary.

The choice is consequential because it determines handler signatures, middleware composition, schema ownership, generated artifacts, validation behavior, and the web application's dependency on the administrative API. The public inference API has additional streaming and OpenAI-compatibility requirements that must not be constrained by control-plane code generation.

The selected tools must preserve standard `net/http` cancellation and streaming semantics, allow explicit security middleware ordering, support OpenAPI 3.0, and permit generated code to remain at the transport boundary.

## Decision

- Use Go's standard `net/http` server and handler contracts for all Go HTTP services.
- Use `github.com/go-chi/chi/v5` as the router and middleware composition layer.
- Define the authoritative control-plane API as repository-owned OpenAPI 3.0.3 YAML under a versioned API-contract path established during repository scaffolding.
- Use `github.com/oapi-codegen/oapi-codegen/v2` to generate control-plane Go transport models, Chi server bindings, and strict-server request/response interfaces.
- Use OpenAPI request-validation middleware backed by the same loaded contract. Strict-server generation alone is not request validation.
- Use `openapi-typescript` 7.x to generate runtime-free TypeScript types for the web application. A typed fetch wrapper may consume these types, but it must not duplicate backend authorization or domain logic.
- Pin exact router, generator, validation-library, and TypeScript-generator versions in committed module and package lockfiles. Generator configuration is committed and versioned with the source contract.
- Commit generated Go and TypeScript artifacts. CI regenerates them from the source contract and fails when the worktree differs.
- Generated files are never edited manually. Contract changes begin in the OpenAPI source, pass schema lint/validation, regenerate artifacts, and update contract tests in the same change.
- Keep generated transport types out of domain, persistence, routing, provider, usage, and authorization packages. Handlers map generated request objects into domain commands and map service results into generated response objects.
- Authentication, active-organization resolution, CSRF, permission checks, resource policy, tenant scope, and domain invariants remain explicit middleware/service responsibilities. OpenAPI security declarations document the transport requirement but do not authorize a request.
- The control-plane contract explicitly defines operation IDs, request and response schemas, nullability, bounds, headers, cookies, status codes, errors, pagination, ETags, cache policy, and content types. Objects reject undeclared properties unless a reviewed extensibility field requires them.
- Validation and decoding errors are translated through NexusRelay's stable sanitized error envelope and include the generated NexusRelay request ID. Default generator or validator error bodies are not exposed.
- The public OpenAI-compatible gateway may use hand-written transport types and handlers on the same Chi/`net/http` foundation where exact compatibility, incremental SSE, unknown-field policy, or response-commit behavior makes control-plane-style generation unsuitable. Its normative contract remains `docs/design/13-api-compatibility-matrix.md`, official SDK fixtures, and protocol tests; an OpenAPI document may supplement but does not override those artifacts.
- Health, readiness, and metrics endpoints may remain small hand-written `net/http` handlers. They still use the shared security, request-ID, redaction, and timeout policies appropriate to their exposure.

## Middleware and Validation Order

The HTTP stack preserves the design's security sequence:

1. Recover panics into a sanitized response without serializing request or domain objects.
2. Assign the NexusRelay request ID and establish structured request context.
3. Apply trusted-proxy handling, security headers, body limits, content-type policy, and route-level timeouts.
4. Authenticate the session and enforce CSRF where required.
5. Run OpenAPI parameter/body validation and decode into generated request objects.
6. Resolve active organization where the endpoint requires tenant scope.
7. Invoke the strict handler, which calls an application service for permission, resource-policy, transaction, and domain enforcement.
8. Render only a response type declared for that operation.

Authentication may precede full body validation so unauthenticated callers cannot use protected endpoints as a schema oracle. Middleware must not read or buffer SSE response bodies, and gateway request timeouts must use the protocol's staged timeout model rather than a blanket handler timeout.

## Alternatives

### Standard Library Router Only

Rejected as the default because Go's standard mux is viable but Chi provides explicit route groups, composable middleware, and direct `oapi-codegen` support while retaining standard `net/http` contracts. The additional dependency is small and does not introduce a framework-owned application model.

### `ogen` With Standard Library Routing

Rejected for V1 because its more comprehensive generation would place greater ownership of server behavior in one generator. NexusRelay benefits from keeping routing, middleware order, error mapping, and incremental implementation explicit while generating only the repetitive transport boundary.

### Gin, Echo, or Fiber

Rejected because they introduce broader framework-specific contexts or response abstractions without a demonstrated requirement. Standard `net/http` compatibility is preferable for cancellation, streaming, middleware reuse, and small independently deployable services.

### Hand-Written Control-Plane Types and Routes

Rejected because it would allow the documented contract, Go handlers, and TypeScript client to drift independently and would weaken compile-time and CI enforcement of statuses, nullability, and request/response shapes.

### OpenAPI 3.1 Initially

Deferred. The selected generators support it, but OpenAPI 3.0.3 provides the required V1 features with a mature common subset and fewer JSON Schema dialect differences. Moving to 3.1 requires a reviewed contract/tooling compatibility change, not a silent version bump.

## Consequences

- HTTP handlers and middleware remain interoperable with the Go standard library and preserve cancellation and streaming controls.
- The control-plane OpenAPI document becomes the single transport-contract source for backend bindings and web types.
- Generated strict interfaces reduce handler boilerplate but do not replace runtime validation, authentication, authorization, tenant enforcement, or domain validation.
- Committed generated artifacts make builds reproducible and contract changes reviewable, at the cost of CI drift checks and occasional generator-driven diffs.
- The gateway can honor exact OpenAI behavior and SSE commitments without forcing those endpoints through administrative API generation conventions.
- Dependency upgrades can change generated output and therefore require pinned-version updates, regeneration, contract tests, and review.

## Verification

- Contract linting and OpenAPI document validation run in CI.
- Go generation compiles against the pinned Chi and `oapi-codegen` versions.
- TypeScript generation passes strict typecheck in the web application.
- Regeneration leaves the worktree clean.
- Contract tests cover unknown fields, malformed parameters, body limits, content types, nullability, all declared statuses, stable errors, ETags, pagination, cookies, CSRF, and no-store headers.
- Permission and two-organization negative tests prove generated routing and schema declarations cannot bypass application authorization.
- Streaming tests prove shared middleware does not buffer responses or replace staged gateway timeouts.
- Sentinel tests prove validator, decoder, and panic errors do not disclose cookies, authorization values, secrets, or request bodies.

## References

- `docs/requirements.md`: FR-API-001 through FR-API-014, FR-UI-001 through FR-UI-006, SEC-004, SEC-008, SEC-011 through SEC-013, NFR-002 through NFR-004, and TEST-005 through TEST-010.
- `docs/design/05-inference-protocol.md`: HTTP boundary, streaming, timeout, validation, and error behavior.
- `docs/design/10-control-plane-web.md`: authoritative control-plane contract and handler/service sequence.
- `docs/design/11-operations-security-testing.md`: proxy, redaction, security, and CI requirements.
- Chi project documentation: <https://github.com/go-chi/chi>
- `oapi-codegen` project documentation: <https://github.com/oapi-codegen/oapi-codegen>
- `openapi-typescript` documentation: <https://openapi-ts.dev/introduction>
- OpenAPI Specification 3.0.3: <https://spec.openapis.org/oas/v3.0.3.html>
