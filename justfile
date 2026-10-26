# Copyright 2026 Srikanth K.
# SPDX-License-Identifier: AGPL-3.0-or-later

default:
    @just --list

# Code generation from aperio's protos: Go stubs, Prisma, store, cache, services.
[group('codegen')]
mod gen 'tools/just/gen.just'

# The aperio database's Prisma migrations: migrate, deploy, status.
[group('db')]
mod db 'tools/just/db.just'

# Hasura DDN metadata regeneration after a database schema change.
[group('hasura')]
mod hasura 'tools/just/hasura.just'

# Application (server) -- run, build, test. Settings come from config: the
# embedded internal/config/aperio.release.toml defaults, overlaid by
# internal/config/aperio.dev.toml in a checkout. Any key can be overridden from
# the environment, APERIO_<SECTION>__<KEY> with double underscores between
# levels:
#   APERIO_SERVER__HTTP__PORT=8095 just run

# Run the server: gRPC :50061, REST :8090 (`just db up` and `just db deploy` first).
[group('app')]
run:
    go run .

# Print each bookable space's Google Calendar link (see tools/calendar).
[group('app')]
calendar-links:
    go run ./tools/calendar

# Compile everything, and the aperio binary into bin/.
[group('build')]
build:
    go build ./...
    go build -o bin/aperio .

# Format the hand-written Go; generated code follows its generator.
[group('build')]
fmt:
    gofmt -w $(grep -L '^// Code generated' $(git ls-files -co --exclude-standard '*.go'))

# Remove the Go build and test caches.
[group('build')]
clean:
    go clean -cache -testcache

# Vet everything.
[group('build')]
vet:
    go vet ./...

# Tidy module dependencies.
[group('build')]
gotidy:
    go mod tidy

# api-linter runs with the one rule api-linter.yaml disables; it exits 0 on
# findings, so a non-empty table fails the recipe.
# The protos: buf format and lint, then api-linter (API design).
[group('lint')]
lint:
    #!/usr/bin/env bash
    set -euo pipefail
    buf format --diff --exit-code
    buf lint
    d=$(mktemp -d); trap 'rm -rf "$d"' EXIT
    buf build -o "$d/desc.binpb" --as-file-descriptor-set
    (cd protobuf && api-linter --descriptor-set-in "$d/desc.binpb" --config ../api-linter.yaml \
        --output-format=summary $(find aperio -name '*.proto')) | tee "$d/out"
    if grep -q '| core::' "$d/out"; then echo "api-linter reported findings above" >&2; exit 1; fi

# The live suites skip here: they need the variables test-live sets.
# Unit tests with the race detector.
[group('test')]
test:
    go test -race -count=1 ./...

# Needs the dev Postgres and Hasura DDN engine (`just db up`, `just db deploy`,
# and the engine in deploy/hasura). A suite that cannot reach its backend
# fails rather than skips. One package at a time (-p 1): they share the database.
# Every test, live suites included, against Postgres and Hasura.
[group('test')]
test-live dsn="host=127.0.0.1 port=5433 user=postgres password=postgrespassword dbname=aperio sslmode=disable" url="http://localhost:3281/graphql":
    APERIO_TEST_REQUIRE_LIVE=1 APERIO_TEST_DATABASE_URL="{{ dsn }}" APERIO_TEST_GRAPHQL_URL="{{ url }}" \
        go test -count=1 -p 1 ./...

# Build, vet and test: what CI runs without a database.
[group('test')]
check: build vet test
