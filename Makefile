SHELL := /bin/sh

.DEFAULT_GOAL := help
.NOTPARALLEL: fmt-check sdk-replay test lint build generate verify atlas-test postgres-init-test backup-restore-test redis-startup-test traefik-config-test cloudflare-config-test private-admin-feasibility-test secret-publication-test local-core-test observability-test

.PHONY: help fmt fmt-check go-fmt sdk-go-fmt go-fmt-check sdk-go-fmt-check \
	go-test go-race go-vet go-build observability-test \
	web-install web-lint web-typecheck web-test web-build \
	api-install api-validate api-generate api-drift api-test \
	fixtures-validate shell-syntax-check python-syntax-check javascript-syntax-check whitespace-check \
	sdk-replay-js sdk-replay-go sdk-replay-python sdk-replay \
	atlas-hash atlas-validate atlas-validate-semantic atlas-test \
	compose-config postgres-init-test backup-verify backup-restore-test redis-startup-test traefik-config-test cloudflare-config-test private-admin-feasibility-test \
	secret-publication-test local-core-init local-core-up local-core-check local-core-down local-core-test \
	image-build-go image-build-web image-build-migrate image-pull-cloudflared \
	test lint build generate verify

help:
	@printf '%s\n' \
		'Formatting:' \
		'  fmt             Autoformat root-module and SDK replay Go files with gofmt.' \
		'  fmt-check       Check Go formatting, authored syntax/validation, fixture canonical form, and whitespace.' \
		'' \
		'Go: go-fmt sdk-go-fmt go-fmt-check sdk-go-fmt-check go-test go-race go-vet go-build observability-test' \
		'Web: web-install web-lint web-typecheck web-test web-build' \
		'API: api-install api-validate api-generate api-drift api-test' \
		'SDK replay: sdk-replay-js sdk-replay-go sdk-replay-python sdk-replay' \
		'Fixtures: fixtures-validate' \
		'Atlas: atlas-hash atlas-validate atlas-validate-semantic atlas-test' \
		'Deployment: compose-config postgres-init-test backup-verify backup-restore-test redis-startup-test traefik-config-test cloudflare-config-test private-admin-feasibility-test secret-publication-test' \
		'Local core: local-core-init local-core-up local-core-check local-core-down local-core-test' \
		'Images: image-build-go image-build-web image-build-migrate image-pull-cloudflared' \
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
	sh -n deploy/cloudflare/generate-config.sh deploy/cloudflare/publish-secrets.sh deploy/cloudflare/test-config.sh deploy/private-admin/generate-corefile.sh deploy/private-admin/test-feasibility.sh deploy/local-core.sh deploy/test-core-startup.sh deploy/migrate/atlas.sh deploy/migrate/entrypoint.sh deploy/migrate/test-validation.sh deploy/postgres/init/10-nexusrelay-roles.sh deploy/postgres/apply-login-passwords.sh deploy/postgres/verify-role-graph.sh deploy/postgres/test-initialization.sh deploy/operations/backup.sh deploy/operations/database-backup-container.sh deploy/operations/crypto-backup-container.sh deploy/operations/verify-backup.sh deploy/operations/restore.sh deploy/operations/restore-container.sh deploy/operations/restore-crypto.sh deploy/operations/restore-crypto-container.sh deploy/operations/test-restore.sh deploy/operations/graph-upgrade.sh deploy/operations/graph-upgrade-container.sh deploy/redis/entrypoint.sh deploy/redis/test-startup.sh deploy/secrets/publish.sh deploy/secrets/test-publication.sh deploy/traefik/entrypoint.sh deploy/traefik/test-config.sh scripts/gofmt.sh

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

observability-test:
	go test -race ./internal/observability ./internal/httpserver ./internal/dependency

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

compose-config:
	docker compose -f deploy/compose.yaml --profile core config >/dev/null
	docker compose -f deploy/compose.yaml --profile core --profile cloudflare config >/dev/null

postgres-init-test:
	deploy/postgres/test-initialization.sh

backup-verify:
	test -n "$${DATABASE_BACKUP_ARTIFACT-}" || { printf '%s\n' 'DATABASE_BACKUP_ARTIFACT is required'; exit 2; }
	test -n "$${CRYPTO_BACKUP_ARTIFACT-}" || { printf '%s\n' 'CRYPTO_BACKUP_ARTIFACT is required'; exit 2; }
	deploy/operations/verify-backup.sh "$$DATABASE_BACKUP_ARTIFACT" "$$CRYPTO_BACKUP_ARTIFACT"

backup-restore-test:
	deploy/operations/test-restore.sh

redis-startup-test:
	deploy/redis/test-startup.sh

traefik-config-test:
	deploy/traefik/test-config.sh

cloudflare-config-test:
	deploy/cloudflare/test-config.sh

private-admin-feasibility-test:
	deploy/private-admin/test-feasibility.sh

secret-publication-test:
	deploy/secrets/test-publication.sh

local-core-init:
	deploy/local-core.sh init

local-core-up:
	deploy/local-core.sh up

local-core-check:
	deploy/local-core.sh check

local-core-down:
	deploy/local-core.sh down

local-core-test:
	deploy/test-core-startup.sh

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

image-pull-cloudflared:
	docker pull cloudflare/cloudflared:2026.7.3@sha256:e39ee8da81ad5e05d77f38d2f51c60ca51bf2a8450ac3abab50c17fdb91d91bf
	docker tag cloudflare/cloudflared:2026.7.3@sha256:e39ee8da81ad5e05d77f38d2f51c60ca51bf2a8450ac3abab50c17fdb91d91bf cloudflare/cloudflared:2026.7.3

test:
	$(MAKE) go-test
	$(MAKE) go-race
	$(MAKE) web-test
	$(MAKE) api-test
	$(MAKE) fixtures-validate
	$(MAKE) sdk-replay
	$(MAKE) atlas-test
	$(MAKE) postgres-init-test
	$(MAKE) backup-restore-test
	$(MAKE) redis-startup-test
	$(MAKE) traefik-config-test
	$(MAKE) cloudflare-config-test
	$(MAKE) secret-publication-test

lint:
	$(MAKE) fmt-check
	$(MAKE) go-vet
	$(MAKE) web-typecheck
	$(MAKE) atlas-validate
	$(MAKE) compose-config

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
