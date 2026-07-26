# Atlas Migration Foundation

NexusRelay uses Atlas Community `1.2.3` from the immutable image digest recorded in `Dockerfile` and `atlas.sh`. The small shell runtime used only by the production entrypoint is also digest-pinned in `Dockerfile`. SQL remains repository-owned under `migrations/`; Atlas Cloud and Atlas Pro are not required.

## Phase Boundary

Phase 1 contains no schema or role migration. The required sequence is:

1. Phase 2 trusted PostgreSQL empty-volume initialization creates the fixed login principals, closed `NOLOGIN` role graph, database, and schema-bootstrap grants.
2. Run the migration artifact as `nexusrelay_migration`.
3. Start the gateway, control-plane, and worker only after migration success.
4. Run `nexusrelay-bootstrap --help` or `--version` independently if desired. The current bootstrap scaffold does not create an owner and migration success must not be described as functional owner bootstrap.

## Commands

Run from the repository root:

```sh
deploy/migrate/atlas.sh version
deploy/migrate/atlas.sh hash
deploy/migrate/atlas.sh validate
deploy/migrate/atlas.sh validate-semantic
deploy/migrate/test-validation.sh
```

Build the migration artifact with required non-secret provenance values:

```sh
VERSION=0.1.0 \
REVISION="$(git rev-parse HEAD)" \
deploy/migrate/atlas.sh build
```

`VERSION` must be non-empty and contain only ASCII letters, digits, `.`, `_`, `+`, or `-`. `REVISION` uses the shared NexusRelay image/build-ID contract: 1 through 128 ASCII letters, digits, dots, underscores, or hyphens, beginning with a letter or digit. The wrapper validates these values before invoking Docker, and the Dockerfile independently validates direct builds. They are public artifact metadata, not secret inputs; never pass credentials through build arguments.

The resulting image overrides inherited Atlas metadata with NexusRelay OCI labels: title `NexusRelay migrations`, a NexusRelay migration-runner description, the supplied version/revision, and source `https://github.com/PrakashSewani/NexusRelay`. Atlas remains separately identifiable at runtime with `deploy/migrate/atlas.sh version` or `/atlas version`; the Atlas base image and digest remain pinned in `Dockerfile`.

`validate-semantic` creates an isolated Docker network and disposable PostgreSQL 18 dev database, waits for readiness, validates SQL semantics, and removes both resources. The validation target is pinned to `postgres:18.4-bookworm@sha256:1961f96e6029a02c3812d7cb329a3b03a3ac2bb067058dec17b0f5596aca9296`.

`test-validation.sh` exercises the production image entrypoint with a harmless mounted Atlas stub, so accepted inputs complete without a database and rejected inputs must stop with the entrypoint's validation exit code. It covers host, port, SSL mode, password-file, fixed-user, and fixed-database contracts, including compressed and uncompressed IPv4-embedded IPv6 regression cases. Set `NEXUSRELAY_MIGRATE_IMAGE` to test an image tag other than `nexusrelay-migrate:local`.

To apply after Phase 2 initialization, export only the non-secret connection settings and the path to the migration password file:

```sh
DATABASE_HOST=postgres \
DATABASE_PORT=5432 \
DATABASE_NAME=nexusrelay \
DATABASE_MIGRATION_USER=nexusrelay_migration \
DATABASE_MIGRATION_PASSWORD_FILE=/absolute/path/to/database_migration_password \
DATABASE_SSLMODE=disable \
deploy/migrate/atlas.sh apply
```

For a database reachable only on an existing Docker network, additionally set `NEXUSRELAY_MIGRATE_DOCKER_NETWORK` to that network name. This is a host-launcher option, not a NexusRelay application setting. Docker Compose should run `nexusrelay-migrate:local` directly on the backend service network with the six documented database settings and the migration password-file mount. The host wrapper deliberately delegates production validation and the fixed Atlas command to the image entrypoint.

The production entrypoint disables shell tracing and fails before Atlas application unless `DATABASE_MIGRATION_USER` is exactly `nexusrelay_migration` and `DATABASE_NAME` is exactly `nexusrelay`. The artifact-level database-name check is intentionally exact; Phase 2 Compose/init separately validates `POSTGRES_DB == DATABASE_NAME`.

`DATABASE_HOST` must be a valid IPv4 literal, IPv6 literal, or conservative DNS hostname. DNS names are limited to 253 bytes with non-empty labels of at most 63 bytes, ASCII letters/digits/hyphens only, and no leading or trailing hyphen. URL delimiters, whitespace, control characters, bracketed input, zone identifiers, and malformed or ambiguous IP literals are rejected. Valid IPv6 literals are bracketed by the entrypoint before the non-secret host representation is passed to `atlas.hcl`. `DATABASE_PORT` must be an integer from 1 through 65535.

`DATABASE_SSLMODE` is allowlisted to `disable`, `require`, `verify-ca`, or `verify-full`; `allow` and `prefer` are rejected. Use `disable` only on the local/private trusted Compose backend network. Prefer `verify-full` when PostgreSQL has a certificate trusted by the image's inherited system CA bundle; `verify-ca` verifies the chain without hostname verification, and `require` encrypts without CA/hostname verification. This artifact does not invent a custom root-certificate setting. Such support must be added through the repository's typed configuration and documented setting inventory if approved later.

The entrypoint requires the password path to be absolute and name a regular non-symlink file. It accepts 1 through 4096 password bytes plus at most one trailing LF or CRLF, with a total file size no greater than 4098 bytes, and rejects an empty password or surrounding ASCII whitespace after newline removal. It does not print, export, or pass the password in arguments. `atlas.hcl` reads the mounted file, removes the permitted trailing newline, URL-escapes the password in memory, and assembles the connection URL. Plaintext is not present in the host command line, container environment, image layer, or Atlas process arguments. Atlas necessarily holds the resulting URL in process memory while connecting. Do not enable shell tracing or Atlas debug output around migration execution.

The production entrypoint first runs `atlas migrate validate` against the committed directory, then executes the fixed apply command. Application explicitly uses Atlas advisory locking with a 60-second lock timeout, linear execution, and one transaction per migration file. It never passes `--baseline`, `--allow-dirty`, `--skip-lock`, `linear-skip`, or `non-linear`.

The final image runs as UID/GID `65532:65532`, is compatible with a read-only root filesystem, and contains Atlas, inherited CA/runtime files, the pinned BusyBox entrypoint runtime, `atlas.hcl`, and the migration directory.

Atlas Community may append an upstream suggestion to install another build with `curl` when a command fails. Never follow that suggestion for NexusRelay. Atlas installation and upgrades must use the reviewed, digest-pinned repository artifact; upgrades require the ADR 0007 verification process.
