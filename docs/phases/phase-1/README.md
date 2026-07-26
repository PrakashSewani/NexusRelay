# Phase 1: Repository Foundation

## Purpose

This directory records the repository foundation completed in Phase 1. Read this handoff before changing established tooling, process boundaries, configuration, generated contracts, images, migration infrastructure, or CI/security policy.

This document is implementation context, not a new source of product requirements. `docs/requirements.md`, the applicable low-level design, and accepted ADRs remain authoritative. If this record conflicts with those documents, stop and reconcile the documentation before changing behavior.

## Completion Status

Phase 1 is complete. The accepted implementation is represented by commit `2c99100` plus the CI portability fixes in commits `4938f95` and `935fe95`.

Phase 1 established compileable and tested foundations. It did not implement a runnable NexusRelay deployment or functional product APIs. Phase 2 begins with the generic core Docker Compose deployment.

## Accomplishments

### Repository And Tooling

- Initialized the Go module and pnpm workspace with exact toolchain versions.
- Added root formatting, validation, test, generation, build, migration, SDK replay, and image targets in `Makefile`.
- Added repository, editor, Git, and Docker build-context policies.
- Pinned Go `1.25.12`, Node.js `24.18.0`, pnpm `11.17.0`, Atlas Community `1.2.3`, and the images/actions used by builds and CI.
- Retained pnpm 11's default dependency-cooling policy with exact reviewed package-version exclusions, explicit native-build approvals, lockfiles, and scoped time-bounded vulnerability exceptions.

### Go Process Boundaries

- Added separate `gateway`, `control-plane`, and `worker` entrypoints.
- Added a compileable `bootstrap` CLI boundary with help/version output only.
- Added shared build identity, HTTP lifecycle, health endpoints, graceful shutdown, and runtime ownership packages.
- Kept readiness fail-closed until real PostgreSQL, Redis, schema, and cryptographic initialization exists.
- Preserved separate binaries and runtime responsibilities rather than creating a shared service executable.

### Typed Configuration

- Added typed startup configuration for the complete `.env.example` inventory.
- Added strict explicit env-file parsing with no shell interpolation or ambient-environment merge.
- Added protected `*_FILE` secret readers, key-ring parsing, URI validation, file permission checks, and disclosure-safe errors.
- Enforced process-specific configuration views so services receive only settings they currently consume.
- Enforced separate gateway, control-plane, worker, migration, and cluster-admin database principals and distinct password files/values at deployment validation.
- Kept deferred Tailscale settings unconsumed while ADR 0002 remains Proposed; enabling the profile fails closed.

### API And Web Foundations

- Added the authoritative OpenAPI 3.0.3 source skeleton under `api/control/v1`.
- Added pinned linting, negative lint tests, Go server generation, TypeScript generation, and generated-file drift checks.
- Added a strict Next.js administrative shell with standalone output, nonce-based CSP handling, accessibility checks, tests, and deterministic image build metadata.
- Kept the Phase 1 administrative contract intentionally free of functional product operations.

### Migration Foundation

- Added Atlas configuration, an empty valid migration directory, and committed `atlas.sum` integrity metadata.
- Added a pinned, non-root migration image and a disclosure-safe entrypoint.
- Added semantic validation using pinned PostgreSQL and a 32-case migration configuration harness.
- Established the fixed V1 PostgreSQL principal and role graph in design/configuration prerequisites.
- Did not add schema SQL or PostgreSQL role initialization; those remain Phase 2 and Phase 3 responsibilities.

### Containers And Compose Groundwork

- Added pinned multi-stage Go and Next.js Dockerfiles with non-root distroless runtimes.
- Added version/revision labels and deterministic build inputs.
- Added a shared Go application image for gateway, control plane, worker, and bootstrap binaries.
- Added a project-scoped shared-configuration volume publisher with an explicit allowlist, immutable revision directories, digest manifest, locking, and atomic `current` symlink publication.
- Kept `deploy/compose.yaml` explicitly non-runnable: it omits PostgreSQL, Redis, Traefik, protected runtime secrets, complete networking, and real dependency readiness.

### Compatibility Fixtures

- Added isolated official OpenAI SDK replay suites for JavaScript, Go, and Python.
- Pinned exact SDK versions and hash-locked Python dependencies.
- Added representative Models, Chat, Responses, Embeddings, streaming, usage, and error fixture replay coverage.
- Preserved the distinction between direct fixture replay and later gateway compatibility evidence required in Phases 6 and 10.

### CI And Security

- Added pinned GitHub Actions workflows on Ubuntu 24.04.
- Added Go formatting, golangci-lint, unit tests, race tests, vet, and build gates.
- Added web lint, typecheck, unit/accessibility tests, and production build gates.
- Added OpenAPI validation/drift, Atlas integrity/semantic/configuration tests, and all SDK replays.
- Added full-history Gitleaks scanning with non-cancelling security concurrency.
- Added vulnerability scans for both Go modules and every JavaScript/Python lockfile.
- Added app, web, and migration image vulnerability scans, retained JSON reports, and CycloneDX SBOM artifacts.
- Ensured all dependency scopes and image SBOMs are processed before final failure enforcement so failed runs retain complete evidence.

## Foundation Decisions To Preserve

Changes that alter these decisions require review against the applicable design and ADR, and may require a new ADR:

- Go services use standard `net/http` with Chi and normalized domains must not import provider SDK types.
- The control-plane source contract is OpenAPI 3.0.3; generated Go and TypeScript files are not hand-edited.
- Atlas Community executes repository-owned forward SQL with committed integrity metadata.
- Runtime containers are non-root, pinned, minimal, and receive only required secrets.
- PostgreSQL is durable truth; Redis is disposable or reconstructable.
- The five fixed PostgreSQL LOGIN principals and closed NOLOGIN role graph are deployment invariants.
- Configuration is typed once at startup; secrets use protected files and disclosure-safe errors.
- Process configuration and credentials remain least-privilege and process-specific.
- Readiness stays fail-closed until required dependencies and cryptographic state are genuinely initialized.
- CI actions, scanner images, build images, and production dependencies remain immutably or exactly pinned.
- Vulnerability exceptions must be scoped, justified, visible, and expiring; broad audit suppression is prohibited.

## Verification Evidence

The completed foundation was verified with:

```text
make verify VERSION=0.1.0-phase1 REVISION=phase1 SOURCE_DATE_EPOCH=1785062400 IMAGE_TAG=phase1-final
docker compose -f deploy/compose.yaml config
docker compose -f deploy/compose.yaml --profile foundation config
actionlint
Gitleaks against the staged repository snapshot and full Git history
OSV scans for all committed JavaScript and Python lockfiles
govulncheck for the root and SDK Go modules
Trivy scans and CycloneDX generation for app, web, and migration images
```

Hosted CI and Security passed at commit `935fe95`. CI covered Go, web, OpenAPI, Atlas, and all three official SDK replay jobs; Security covered full-history secret scanning, all committed dependency scopes, and app, web, and migration image reports/SBOMs.

## Deferred Work

Phase 1 does not provide:

- A runnable core Docker Compose profile.
- PostgreSQL empty-volume initialization or role-creation SQL.
- Redis, Traefik, complete networking, protected runtime secret mounts, or resource limits.
- Real PostgreSQL/Redis/schema/cryptographic readiness initialization.
- Functional owner bootstrap, authentication, organizations, RBAC, providers, models, keys, inference, routing, accounting, budgets, analytics, audit, or retention.
- Implemented provider adapters or coding-agent exporters.
- Gateway-level OpenAI SDK compatibility evidence.
- Accepted Tailscale/CoreDNS deployment behavior.

Do not infer these capabilities from the presence of entrypoints, configuration types, fixtures, images, or the partial Compose fragment.

## Phase 2 Handoff

Start Phase 2 with the earliest unchecked item in `TODO.md`: build the generic core Compose profile with Traefik, web, gateway, control plane, worker, migrate, PostgreSQL, Redis, trusted empty-volume PostgreSQL initialization, protected secrets, health checks, and localhost operation.

Preserve the Phase 1 process, credential, image, migration, configuration, and CI boundaries while completing that topology. Update this record only to correct Phase 1 history; record Phase 2 accomplishments in a separate phase directory.
