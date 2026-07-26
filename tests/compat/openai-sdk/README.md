# Official OpenAI SDK Fixture Replay

This isolated Phase 0 harness uses the exact SDK pins in `docs/testing/openai-sdk-compatibility.md`. Each runner issues the six documented SDK requests to a loopback-only HTTP server, compares method, path, and parsed JSON body with `docs/testing/fixtures/openai-sdk/requests/sdk-request-observations.json`, serves the corresponding committed response bytes, and verifies SDK deserialization and stream terminal behavior. It does not add dependencies to the NexusRelay application modules.

The request observations remain provisional Phase 0 evidence rather than the public API contract. The runners allow no language-specific body differences for these six scenarios. They disable SDK retries and reject unexpected methods, paths, bodies, query strings, or extra requests.

Static fixture inventory, checksums, formatting, all-fixture privacy sentinels, success and failure ordering, and cancellation metadata are validated with:

```sh
python3 tools/validate_phase0_fixtures.py
```

## Python

CI uses CPython 3.9.23 on Linux amd64 from the exact image below. `requirements.lock` pins every transitive version and includes only the hashes for the wheels selected by that runtime. Other Python versions, operating systems, and architectures, including macOS local installation, are intentionally not supported by this lock.

```sh
docker run --rm --platform linux/amd64 \
  -v "$PWD:/workspace" \
  -w /workspace \
  python:3.9.23-slim-bookworm@sha256:7bffea15bcc3d7fb87cf10a027986203e4281e078fa2f5b234c30fca291f0834 \
  sh -c 'python -m venv /tmp/replay && /tmp/replay/bin/python -m pip install --require-hashes -r tests/compat/openai-sdk/python/requirements.lock && /tmp/replay/bin/python tests/compat/openai-sdk/python/replay.py'
```

## JavaScript

The private npm package pins `openai@6.49.0`; `package-lock.json` records its exact npm integrity. Run from a clean dependency installation with the root-pinned Node 24.18.0 runtime:

```sh
npm --prefix tests/compat/openai-sdk/javascript ci --ignore-scripts
npm --prefix tests/compat/openai-sdk/javascript test
```

## Go

The isolated module pins `github.com/openai/openai-go/v3@v3.46.0` and commits `go.sum`. The verified runner is the exact Go 1.25.0 image used by the compatibility baseline:

```sh
docker run --rm \
  -v "$PWD:/workspace" \
  -w /workspace/tests/compat/openai-sdk/go \
  golang:1.25.0@sha256:5502b0e56fca23feba76dbc5387ba59c593c02ccc2f0f7355871ea9a0852cebe \
  go test ./... -count=1
```

## Remaining Gate

This replay is not the Phase 6 gateway compatibility suite. It proves request serialization against the provisional observations and response parsing against a fixture server, not behavior through NexusRelay. The later gateway suite must replace provisional observations with captured requests, assert status/headers/raw error bodies, exercise reachable pre-commit and post-commit failures, test cancellation propagation and accounting, deny non-loopback traffic, and prove byte-stable regeneration.
