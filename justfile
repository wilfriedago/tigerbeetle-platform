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

# Order: 1. sqlc; 2. the Go commands that write the specs; 3. Go code read from the specs; 4. the
# TypeScript generators.
[doc('Regenerate every generated file, in dependency order.')]
[group('generate')]
generate: generate-sql generate-contracts generate-servers generate-ts

# Step 1: the Postgres accessors.
[group('generate')]
generate-sql:
    cd {{ api }} && sqlc generate

# Step 2: the REST spec and the GraphQL schema from the Go registrations, built without cgo
# (ADR-0018, ADR-0020).
[group('generate')]
generate-contracts:
    cd {{ api }} && CGO_ENABLED=0 go run ./cmd/openapi -out ../../packages/openapi/openapi.yaml
    cd {{ api }} && CGO_ENABLED=0 go run ./cmd/graphqlschema -out ../../packages/graphql/schema.graphql

# Step 3: the GraphQL executor, from the schema step 2 wrote, and the protobuf code from
# packages/proto (buf runs the pinned protoc plugins from PATH).
[group('generate')]
generate-servers:
    cd {{ api }} && go tool gqlgen generate --config gqlgen.yml
    cd packages/proto && buf generate

# Step 4: the spec lint and TypeScript types, and the console's route tree, through Turborepo.
[group('generate')]
generate-ts:
    pnpm turbo run generate

# Regenerate, then fail when any file differs from the commit (CI's drift check).
[group('generate')]
generate-check: generate
    @if [ -n "$(git status --porcelain)" ]; then git status --short; echo "generated files are out of date: run just generate and commit the result" >&2; exit 1; fi

# golangci-lint for Go; vp check, the packages' linters and Redocly for the rest; buf for the protos.
[doc('Every linter.')]
[group('quality')]
lint: lint-go lint-ts lint-proto

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

# buf lint with the STANDARD rules over packages/proto (ADR-0021).
[group('quality')]
lint-proto:
    cd packages/proto && buf lint

# Each fixture in apps/api/testdata/depguard must trip exactly the rule it names (plan 14.2).
[doc('Prove that every depguard rule fires on its fixture.')]
[group('quality')]
lint-depguard-probe:
    scripts/depguard-probe.sh

# Path parameters are flattened: the generated spec declares them on each operation, the
# hand-written one declared them once per path (ADR-0018). packages/openapi/oasdiff-ignore.txt
# lists the breaking changes made on purpose.
[doc('Breaking changes, then the changelog, of the REST spec against a git revision (CI: the base branch).')]
[group('quality')]
spec-diff base="main":
    oasdiff breaking "{{ base }}:packages/openapi/openapi.yaml" packages/openapi/openapi.yaml --flatten-params --fail-on ERR --err-ignore packages/openapi/oasdiff-ignore.txt
    oasdiff changelog "{{ base }}:packages/openapi/openapi.yaml" packages/openapi/openapi.yaml --flatten-params

# FILE rules: a field or message removed, renumbered or retyped in a file breaks generated clients.
# A base without packages/proto has nothing to compare.
[doc('Breaking changes of the protos against a git revision (CI: the base branch).')]
[group('quality')]
proto-breaking base="main":
    #!/usr/bin/env bash
    set -euo pipefail
    if ! git cat-file -e "{{ base }}:packages/proto/buf.yaml" 2>/dev/null; then
      echo "{{ base }} has no packages/proto: nothing to compare"
      exit 0
    fi
    cd packages/proto && buf breaking --against "../../.git#ref={{ base }},subdir=packages/proto"

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

# Every transport of the compose stack from the host (needs `just compose-up-brokers`). The E2E_*
# variables override the addresses; see apps/api/e2e.
[group('test')]
test-e2e:
    cd {{ api }} && go test -race -tags e2e -count=1 ./e2e/...

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

# The stack plus RabbitMQ, NATS, Kafka and the TigerBeetle CDC job, with gRPC, AMQP RPC, NATS RPC
# and the Kafka relay turned on (ADR-0021, ADR-0022).
[group('stack')]
compose-up-brokers:
    GRPC_ENABLED=true AMQP_RPC_ENABLED=true NATS_RPC_ENABLED=true KAFKA_EVENTS_ENABLED=true {{ compose }} --profile brokers up --build --detach --wait

# Stop the local stack, keeping its data. The brokers profile is named so its services stop too.
[group('stack')]
compose-down:
    {{ compose }} --profile brokers down

# Follow the logs of the local stack.
[group('stack')]
compose-logs:
    {{ compose }} --profile brokers logs --follow

# Stop the local stack and delete its volumes (TigerBeetle and Postgres data).
[group('stack')]
compose-reset:
    {{ compose }} --profile brokers down --volumes

# golangci-lint, sqlc, govulncheck, just, oasdiff, buf, protoc-gen-go and protoc-gen-go-grpc.
[doc('Install the pinned tools into $(go env GOPATH)/bin.')]
[group('tools')]
tools-install:
    scripts/install-tools.sh

# Check that the TigerBeetle client in go.mod matches the server image in compose.
[group('tools')]
versions-check:
    scripts/check-versions.sh
