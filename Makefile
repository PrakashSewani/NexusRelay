SHELL := /bin/sh

.DEFAULT_GOAL := help
.NOTPARALLEL: fmt-check sdk-replay test lint build generate verify atlas-test

.PHONY: help fmt fmt-check go-fmt sdk-go-fmt go-fmt-check sdk-go-fmt-check \
	go-test go-race go-vet go-build \
	web-install web-lint web-typecheck web-test web-build \
	api-install api-validate api-generate api-drift api-test \
	fixtures-validate shell-syntax-check python-syntax-check javascript-syntax-check whitespace-check \
	sdk-replay-js sdk-replay-go sdk-replay-python sdk-replay \
	atlas-hash atlas-validate atlas-validate-semantic atlas-test \
	image-build-go image-build-web image-build-migrate \
	test lint build generate verify

help:
	@printf '%s\n' \
		'Formatting:' \
		'  fmt             Autoformat root-module and SDK replay Go files with gofmt.' \
		'  fmt-check       Check Go formatting, authored syntax/validation, fixture canonical form, and whitespace.' \
		'' \
		'Go: go-fmt sdk-go-fmt go-fmt-check sdk-go-fmt-check go-test go-race go-vet go-build' \
		'Web: web-install web-lint web-typecheck web-test web-build' \
		'API: api-install api-validate api-generate api-drift api-test' \
		'SDK replay: sdk-replay-js sdk-replay-go sdk-replay-python sdk-replay' \
		'Fixtures: fixtures-validate' \
		'Atlas: atlas-hash atlas-validate atlas-validate-semantic atlas-test' \
		'Images: image-build-go image-build-web image-build-migrate' \
		'Aggregates: test lint build generate verify' \
		'' \
		'All image builds require VERSION, REVISION, and IMAGE_TAG.' \
		'Go and web image builds also require SOURCE_DATE_EPOCH.' \
		'test and verify require VERSION, REVISION, and IMAGE_TAG because they run atlas-test.' \
		'IMAGE_TAG is validated independently as a Docker tag and is never derived from VERSION.' \
		'atlas-test builds and tests nexusrelay-migrate:IMAGE_TAG; no target applies migrations.'

fmt:
	$(MAKE) go-fmt
	$(MAKE) sdk-go-fmt

go-fmt:
	scripts/gofmt.sh write root

sdk-go-fmt:
	scripts/gofmt.sh write sdk

fmt-check:
	$(MAKE) go-fmt-check
	$(MAKE) sdk-go-fmt-check
	$(MAKE) web-lint
	$(MAKE) api-validate
	$(MAKE) shell-syntax-check
	$(MAKE) python-syntax-check
	$(MAKE) javascript-syntax-check
	$(MAKE) fixtures-validate
	$(MAKE) whitespace-check

go-fmt-check:
	scripts/gofmt.sh check root

sdk-go-fmt-check:
	scripts/gofmt.sh check sdk

shell-syntax-check:
	sh -n deploy/migrate/atlas.sh deploy/migrate/entrypoint.sh deploy/migrate/test-validation.sh scripts/gofmt.sh

python-syntax-check:
	python3 -c 'from pathlib import Path; paths = sorted(Path("tools").glob("*.py")) + sorted(Path("tests/compat/openai-sdk/python").glob("*.py")); [compile(path.read_bytes(), str(path), "exec") for path in paths]'

javascript-syntax-check:
	node --check apps/web/eslint.config.mjs
	node --check api/control/v1/scripts/drift.mjs
	node --check api/control/v1/scripts/test-lint-negative.mjs
	node --check tests/compat/openai-sdk/javascript/replay.mjs

whitespace-check:
	git diff --check

go-test:
	go test ./...

go-race:
	go test -race ./...

go-vet:
	go vet ./...

go-build:
	go build ./...

web-install:
	corepack pnpm --dir apps/web install --frozen-lockfile

web-lint:
	corepack pnpm --dir apps/web run lint

web-typecheck:
	corepack pnpm --dir apps/web run typecheck

web-test:
	corepack pnpm --dir apps/web run test

web-build:
	corepack pnpm --dir apps/web run build

api-install:
	corepack pnpm -C api/control/v1 install --frozen-lockfile

api-validate:
	corepack pnpm --dir api/control/v1 --ignore-workspace run validate

api-generate:
	corepack pnpm --dir api/control/v1 --ignore-workspace run generate

api-drift:
	corepack pnpm --dir api/control/v1 --ignore-workspace run drift

api-test:
	corepack pnpm --dir api/control/v1 --ignore-workspace run test

fixtures-validate:
	python3 tools/validate_phase0_fixtures.py

sdk-replay-js:
	npm --prefix tests/compat/openai-sdk/javascript ci --ignore-scripts
	npm --prefix tests/compat/openai-sdk/javascript test

sdk-replay-go:
	go -C tests/compat/openai-sdk/go test ./... -count=1

sdk-replay-python:
	docker run --rm --platform linux/amd64 -v "$$PWD:/workspace" -w /workspace python:3.9.23-slim-bookworm@sha256:7bffea15bcc3d7fb87cf10a027986203e4281e078fa2f5b234c30fca291f0834 sh -c 'python -m venv /tmp/replay && /tmp/replay/bin/python -m pip install --require-hashes -r tests/compat/openai-sdk/python/requirements.lock && /tmp/replay/bin/python tests/compat/openai-sdk/python/replay.py'

sdk-replay:
	$(MAKE) sdk-replay-js
	$(MAKE) sdk-replay-go
	$(MAKE) sdk-replay-python

atlas-hash:
	deploy/migrate/atlas.sh hash

atlas-validate:
	deploy/migrate/atlas.sh validate

atlas-validate-semantic:
	deploy/migrate/atlas.sh validate-semantic

atlas-test: image-build-migrate
	NEXUSRELAY_MIGRATE_IMAGE="nexusrelay-migrate:$$IMAGE_TAG" deploy/migrate/test-validation.sh

image-build-go:
	test -n "$${VERSION-}" || { printf '%s\n' 'VERSION is required'; exit 2; }
	test -n "$${REVISION-}" || { printf '%s\n' 'REVISION is required'; exit 2; }
	test -n "$${SOURCE_DATE_EPOCH-}" || { printf '%s\n' 'SOURCE_DATE_EPOCH is required'; exit 2; }
	test -n "$${IMAGE_TAG-}" || { printf '%s\n' 'IMAGE_TAG is required'; exit 2; }
	case "$$IMAGE_TAG" in [A-Za-z0-9_]*) ;; *) printf '%s\n' 'IMAGE_TAG must start with an ASCII letter, digit, or underscore'; exit 2;; esac
	case "$$IMAGE_TAG" in *[!A-Za-z0-9_.-]*) printf '%s\n' 'IMAGE_TAG must match [A-Za-z0-9_][A-Za-z0-9_.-]{0,127}'; exit 2;; esac
	test "$${#IMAGE_TAG}" -le 128 || { printf '%s\n' 'IMAGE_TAG must be at most 128 characters'; exit 2; }
	docker buildx build --load --file deploy/docker/Dockerfile.go-app --build-arg VERSION="$$VERSION" --build-arg REVISION="$$REVISION" --build-arg SOURCE_DATE_EPOCH="$$SOURCE_DATE_EPOCH" --tag "nexusrelay-app:$$IMAGE_TAG" .

image-build-web:
	test -n "$${VERSION-}" || { printf '%s\n' 'VERSION is required'; exit 2; }
	test -n "$${REVISION-}" || { printf '%s\n' 'REVISION is required'; exit 2; }
	test -n "$${SOURCE_DATE_EPOCH-}" || { printf '%s\n' 'SOURCE_DATE_EPOCH is required'; exit 2; }
	test -n "$${IMAGE_TAG-}" || { printf '%s\n' 'IMAGE_TAG is required'; exit 2; }
	case "$$IMAGE_TAG" in [A-Za-z0-9_]*) ;; *) printf '%s\n' 'IMAGE_TAG must start with an ASCII letter, digit, or underscore'; exit 2;; esac
	case "$$IMAGE_TAG" in *[!A-Za-z0-9_.-]*) printf '%s\n' 'IMAGE_TAG must match [A-Za-z0-9_][A-Za-z0-9_.-]{0,127}'; exit 2;; esac
	test "$${#IMAGE_TAG}" -le 128 || { printf '%s\n' 'IMAGE_TAG must be at most 128 characters'; exit 2; }
	docker buildx build --load --file deploy/docker/Dockerfile.web --build-arg VERSION="$$VERSION" --build-arg REVISION="$$REVISION" --build-arg SOURCE_DATE_EPOCH="$$SOURCE_DATE_EPOCH" --tag "nexusrelay-web:$$IMAGE_TAG" .

image-build-migrate:
	test -n "$${VERSION-}" || { printf '%s\n' 'VERSION is required'; exit 2; }
	test -n "$${REVISION-}" || { printf '%s\n' 'REVISION is required'; exit 2; }
	test -n "$${IMAGE_TAG-}" || { printf '%s\n' 'IMAGE_TAG is required'; exit 2; }
	case "$$IMAGE_TAG" in [A-Za-z0-9_]*) ;; *) printf '%s\n' 'IMAGE_TAG must start with an ASCII letter, digit, or underscore'; exit 2;; esac
	case "$$IMAGE_TAG" in *[!A-Za-z0-9_.-]*) printf '%s\n' 'IMAGE_TAG must match [A-Za-z0-9_][A-Za-z0-9_.-]{0,127}'; exit 2;; esac
	test "$${#IMAGE_TAG}" -le 128 || { printf '%s\n' 'IMAGE_TAG must be at most 128 characters'; exit 2; }
	deploy/migrate/atlas.sh build
	docker tag nexusrelay-migrate:local "nexusrelay-migrate:$$IMAGE_TAG"

test:
	$(MAKE) go-test
	$(MAKE) go-race
	$(MAKE) web-test
	$(MAKE) api-test
	$(MAKE) fixtures-validate
	$(MAKE) sdk-replay
	$(MAKE) atlas-test

lint:
	$(MAKE) fmt-check
	$(MAKE) go-vet
	$(MAKE) web-typecheck
	$(MAKE) atlas-validate

build:
	$(MAKE) go-build
	$(MAKE) web-build

generate:
	$(MAKE) api-generate

verify:
	$(MAKE) lint
	$(MAKE) api-drift
	$(MAKE) test
	$(MAKE) build
	$(MAKE) atlas-validate-semantic
