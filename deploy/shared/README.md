# Shared Runtime Configuration

This foundation publishes allowlisted assets into a project-scoped
`shared-config` Compose named volume. Gateway, control-plane, worker, and the
optional bootstrap scaffold mount the volume read-only at
`/etc/nexusrelay/shared` and read the active release through
`/etc/nexusrelay/shared/current`.

The initializer runs only with the `foundation` or `bootstrap` profile. Each
validated `NEXUSRELAY_REVISION` is published once under
`releases/<revision>`. Repeating the same revision verifies that its payload is
identical; a changed payload fails instead of overwriting it. A new revision is
fully staged before the `current` symlink is atomically replaced, and prior
releases are retained for readers that already resolved the old path.

The current allowlist contains only this `README.md`. The Compose file mounts
that file directly into the initializer, so hidden, untracked, or unrelated
files in `deploy/shared` cannot be copied. Future assets must be added to an
explicit reviewed allowlist and integrity manifest rather than copied with a
directory wildcard.

Deployment settings continue to come from typed environment configuration, and
secret values must use their dedicated protected `*_FILE` mounts. Never place
`.env`, credentials, key rings, passwords, tokens, or writable application state
in this directory or its named volume.

`NEXUSRELAY_IMAGE_TAG` controls image references independently from the
application's `NEXUSRELAY_VERSION` build metadata. It defaults to the safe local
tag `dev`; operators must supply a valid Docker tag when overriding it. This is
only an image and shared-asset foundation, not a runnable Phase 2 deployment.
