# NexusRelay Images

These Dockerfiles implement the Phase 1 image constraints in `docs/design/01-system-topology.md` and `docs/design/11-operations-security-testing.md`. Build from the repository root so the committed Go module, pnpm workspace, lockfile, and pnpm `allowBuilds` policy are in scope.

## Build

`VERSION`, `REVISION`, and `SOURCE_DATE_EPOCH` are required non-secret build arguments. Version accepts only `A-Z`, `a-z`, `0-9`, `.`, `_`, `+`, and `-`. Revision must be 1-128 ASCII letters, numbers, dots, underscores, or hyphens and start with a letter or number so the same value is valid as the Next.js build ID. `SOURCE_DATE_EPOCH` must be a Unix timestamp; use the source revision's commit time rather than the current time.

```sh
VERSION=0.1.0
REVISION="$(git rev-parse HEAD)"
SOURCE_DATE_EPOCH="$(git show -s --format=%ct "$REVISION")"

docker buildx build --load --file deploy/docker/Dockerfile.go-app --build-arg VERSION="$VERSION" --build-arg REVISION="$REVISION" --build-arg SOURCE_DATE_EPOCH="$SOURCE_DATE_EPOCH" --tag "nexusrelay-app:$VERSION" .
docker buildx build --load --file deploy/docker/Dockerfile.web --build-arg VERSION="$VERSION" --build-arg REVISION="$REVISION" --build-arg SOURCE_DATE_EPOCH="$SOURCE_DATE_EPOCH" --tag "nexusrelay-web:$VERSION" .
```

The Go image defaults to the gateway. Compose selects another process without a shell by replacing the command with one absolute executable path:

```sh
docker run --rm nexusrelay-app:0.1.0 /usr/local/bin/nexusrelay-gateway --version
docker run --rm nexusrelay-app:0.1.0 /usr/local/bin/nexusrelay-control-plane --version
docker run --rm nexusrelay-app:0.1.0 /usr/local/bin/nexusrelay-worker --version
docker run --rm nexusrelay-app:0.1.0 /usr/local/bin/nexusrelay-bootstrap --version
docker run --rm --publish 3000:3000 nexusrelay-web:0.1.0
```

Gateway, control-plane, and worker require their documented runtime configuration for normal service startup. The standalone version commands do not read runtime secrets. The web server generates the nonce CSP dynamically through the compiled Next.js proxy and performs no runtime dependency downloads.

## Constraints

The Go runtime uses pinned distroless static Debian 13, runs as UID/GID `65532`, and contains no shell or package manager. Its CA certificates and timezone data come from the distroless base. All four binaries receive the same compiled version and revision as the runtime OCI labels.

The web dependency and build stages use pinned Node 24.18.0, exact Corepack pnpm 11.17.0, the frozen root lockfile, and the root `allowBuilds` policy. No supply-chain policy is bypassed. The runtime uses pinned distroless Node.js 24 Debian 13, runs as UID/GID `65532`, starts through `/nodejs/bin/node`, and contains no shell, npm, pnpm, Corepack, or Yarn. It contains only the traced standalone server and production assets copied from the image build; host `.next` output is ignored.

The one-shot Atlas migration image is separately owned under `deploy/migrate`. These Dockerfiles do not duplicate or wrap it; Compose should reference that independently pinned image.

## Inspect

```sh
docker image inspect nexusrelay-app:0.1.0 --format '{{json .Config.User}} {{json .Config.Entrypoint}} {{json .Config.Cmd}} {{json .Config.Labels}}'
docker image inspect nexusrelay-web:0.1.0 --format '{{json .Config.User}} {{json .Config.Entrypoint}} {{json .Config.Cmd}} {{json .Config.Labels}}'
docker history --no-trunc nexusrelay-app:0.1.0
docker history --no-trunc nexusrelay-web:0.1.0
```

## Reproducibility

With identical source content, lockfiles, build arguments, target platform, BuildKit version, pinned Dockerfile frontend, and `SOURCE_DATE_EPOCH`, the required reproducibility guarantee is identical application payload hashes:

- All four Go binaries must have identical hashes.
- All regular files in the web standalone, static, and public payload must have identical path-to-hash mappings.

Independent uncached BuildKit builds are not required to produce identical image IDs, image configs, or layer digests because archive and directory metadata may vary without changing application payloads. Image ID comparison is diagnostic only and must not gate acceptance.

Release images are content-addressed outputs built once per target platform with provenance enabled, then scanned, signed where configured, and promoted by immutable registry digest. CI verifies pinned inputs and frontend/runtime digests, retained provenance, and repeat-build application payload hashes; it does not rebuild a release independently and require image-digest equality.

```sh
VERSION=0.1.0
REVISION="$(git rev-parse HEAD)"
SOURCE_DATE_EPOCH="$(git show -s --format=%ct "$REVISION")"

docker buildx build --load --platform linux/amd64 --file deploy/docker/Dockerfile.go-app --build-arg VERSION="$VERSION" --build-arg REVISION="$REVISION" --build-arg SOURCE_DATE_EPOCH="$SOURCE_DATE_EPOCH" --tag nexusrelay-app:repeat-a .
docker buildx build --load --platform linux/amd64 --file deploy/docker/Dockerfile.go-app --build-arg VERSION="$VERSION" --build-arg REVISION="$REVISION" --build-arg SOURCE_DATE_EPOCH="$SOURCE_DATE_EPOCH" --tag nexusrelay-app:repeat-b .

for image in nexusrelay-app:repeat-a nexusrelay-app:repeat-b; do container="$(docker create "$image")"; for binary in gateway control-plane worker bootstrap; do docker export "$container" | tar -xOf - "usr/local/bin/nexusrelay-$binary" | shasum -a 256; done; docker rm "$container" >/dev/null; done

docker buildx build --load --platform linux/amd64 --file deploy/docker/Dockerfile.web --build-arg VERSION="$VERSION" --build-arg REVISION="$REVISION" --build-arg SOURCE_DATE_EPOCH="$SOURCE_DATE_EPOCH" --tag nexusrelay-web:repeat-a .
docker buildx build --load --platform linux/amd64 --file deploy/docker/Dockerfile.web --build-arg VERSION="$VERSION" --build-arg REVISION="$REVISION" --build-arg SOURCE_DATE_EPOCH="$SOURCE_DATE_EPOCH" --tag nexusrelay-web:repeat-b .

for image in nexusrelay-web:repeat-a nexusrelay-web:repeat-b; do container="$(docker create "$image")"; directory="$(mktemp -d)"; docker export "$container" | tar -xf - -C "$directory"; (cd "$directory" && find app -type f -print0 | LC_ALL=C sort -z | xargs -0 shasum -a 256); docker rm "$container" >/dev/null; rm -rf "$directory"; done

# Optional, non-gating diagnostic only:
docker image inspect nexusrelay-app:repeat-a nexusrelay-app:repeat-b nexusrelay-web:repeat-a nexusrelay-web:repeat-b --format '{{.RepoTags}} {{.Id}}'
```
