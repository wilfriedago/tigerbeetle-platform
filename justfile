# Single entry point for every task in the repository (ADR-0016). `just --list` shows the recipes.
# Go recipes call the Go toolchain directly; TypeScript recipes go through pnpm and Turborepo,
# which keeps caching the TypeScript packages.

set shell := ["bash", "-euo", "pipefail", "-c"]

api := "apps/api"
compose := "docker compose --project-directory . -f deploy/compose/compose.yaml"

# Tools from scripts/install-tools.sh live in $(go env GOPATH)/bin; put them first on PATH, as the
# git hooks do. Without Go installed, fall back to the default GOPATH so the TypeScript recipes
# still run.
export PATH := `go env GOPATH 2>/dev/null || printf '%s' "$HOME/go"` + "/bin:" + env("PATH")

# List the recipes.
default:
    @just --list

# Order: 1. sqlc; 2. the Go commands that write the specs (from phase 2 of
# docs/plans/ddd-transports/plan.md); 3. Go code read from the specs; 4. the TypeScript generators.
[doc('Regenerate every generated file, in dependency order.')]
[group('generate')]
generate: generate-sql generate-servers generate-ts

# Step 1: the Postgres accessors.
[group('generate')]
generate-sql:
    cd {{ api }} && sqlc generate

# Step 3: the Go HTTP server from the hand-written spec (oapi-codegen, until phase 2).
[group('generate')]
generate-servers:
    cd {{ api }} && go tool oapi-codegen -config oapi-codegen.yaml ../../packages/openapi/openapi.yaml

# Step 4: the spec lint and TypeScript types, and the console's route tree, through Turborepo.
[group('generate')]
generate-ts:
    pnpm turbo run generate

# Regenerate, then fail when any file differs from the commit (CI's drift check).
[group('generate')]
generate-check: generate
    @if [ -n "$(git status --porcelain)" ]; then git status --short; echo "generated files are out of date: run just generate and commit the result" >&2; exit 1; fi

# Every linter: golangci-lint for Go; vp check, the packages' linters and Redocly for the rest.
[group('quality')]
lint: lint-go lint-ts

# golangci-lint, including the depguard dependency rules.
[group('quality')]
lint-go:
    cd {{ api }} && golangci-lint run ./...

# Oxfmt and Oxlint over the repository, then each package's lint task (Redocly for the spec).
[group('quality')]
lint-ts:
    pnpm exec vp check
    pnpm turbo run lint

# Redocly on the spec alone, for the pre-commit hook and the CI openapi job.
[group('quality')]
lint-spec:
    pnpm --filter @tigerbeetle-platform/openapi lint

# golangci-lint runs gofumpt and goimports on Go; vp check --fix runs Oxfmt and Oxlint fixes.
[doc('Format everything and write the changes.')]
[group('quality')]
format:
    cd {{ api }} && golangci-lint fmt ./...
    pnpm exec vp check --fix

# Format the given Go files; the pre-commit hook passes the staged ones.
[group('quality')]
fmt-go-files +files:
    cd {{ api }} && golangci-lint fmt {{ files }}

# TypeScript type checks. Go type-checks when it builds.
[group('quality')]
typecheck:
    pnpm turbo run typecheck

# Known vulnerabilities in the Go module and its dependencies.
[group('quality')]
vulncheck:
    cd {{ api }} && govulncheck ./...

# Every unit test: Go and TypeScript.
[group('test')]
test: test-go test-ts

# Go unit tests with the race detector.
[group('test')]
test-go:
    cd {{ api }} && go test -race ./...

# TypeScript unit tests of every package.
[group('test')]
test-ts:
    pnpm turbo run test

# Go integration tests against real Postgres and TigerBeetle containers (needs Docker).
[group('test')]
test-integration:
    cd {{ api }} && go test -race -tags integration -count=1 ./...

# Every build: the API binary and the console bundle.
[group('build')]
build: build-go build-ts

# The API binary in apps/api/bin (cgo, for the TigerBeetle client).
[group('build')]
build-go:
    cd {{ api }} && go build -trimpath -o bin/api ./cmd/api

# The TypeScript packages and the console bundle.
[group('build')]
build-ts:
    pnpm turbo run build

# Both need Postgres and TigerBeetle (just compose-up, or your own) and the variables of
# .env.example in the environment.
[doc('The API (go run) and the console (vp dev) side by side.')]
[group('dev')]
[parallel]
dev: dev-api dev-ts

[group('dev')]
dev-api:
    cd {{ api }} && go run ./cmd/api

[group('dev')]
dev-ts:
    pnpm turbo run dev

# Build and start the local stack (TigerBeetle, Postgres, API, console) and wait until healthy.
[group('stack')]
compose-up:
    {{ compose }} up --build --detach --wait

# Stop the local stack, keeping its data.
[group('stack')]
compose-down:
    {{ compose }} down

# Follow the logs of the local stack.
[group('stack')]
compose-logs:
    {{ compose }} logs --follow

# Stop the local stack and delete its volumes (TigerBeetle and Postgres data).
[group('stack')]
compose-reset:
    {{ compose }} down --volumes

# Install the pinned tools (golangci-lint, sqlc, govulncheck, just) into $(go env GOPATH)/bin.
[group('tools')]
tools-install:
    scripts/install-tools.sh

# Check that the TigerBeetle client in go.mod matches the server image in compose.
[group('tools')]
versions-check:
    scripts/check-versions.sh
