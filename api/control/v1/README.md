# Control-Plane OpenAPI Contract

`openapi.yaml` is the authoritative OpenAPI 3.0.3 source for administrative
endpoints under `/api/control/v1`.

The Phase 1 contract intentionally has no operations. Health and readiness may
remain handwritten under ADR 0006, while administrative operations are added
with the subsystem phase that fully specifies and implements them.

Run the tooling from the repository root:

```sh
corepack pnpm --dir api/control/v1 --ignore-workspace install --frozen-lockfile
corepack pnpm --dir api/control/v1 --ignore-workspace run validate
corepack pnpm --dir api/control/v1 --ignore-workspace run test
corepack pnpm --dir api/control/v1 --ignore-workspace run generate
corepack pnpm --dir api/control/v1 --ignore-workspace run drift
```

Generation requires the repository's pinned Go toolchain on `PATH`. The local
package pins `oapi-codegen` in the generation command and pins the OpenAPI lint
and TypeScript generation dependencies in `package.json` and `pnpm-lock.yaml`.
The lint test uses `fixtures/missing-operation-id.yaml` to prove that the
required `operationId` rule fails closed without adding an operation to the
public contract.

The committed lint policy also requires unique URL-safe operation IDs, explicit
security intent, success and client-error responses, `x-request-id` response
headers, JSON body media types, valid examples, unambiguous paths and
parameters, unique component names, and strict resolvable references.

Generated files are committed at:

- `internal/transport/controlplane/generated/control_plane.gen.go`
- `apps/web/src/generated/control-plane/schema.d.ts`

Do not edit generated files directly. Runtime request-validation middleware,
authentication, authorization, and handler integration are separate control-
plane implementation work.
