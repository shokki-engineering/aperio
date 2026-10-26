# Copyright 2026 Srikanth K.
# SPDX-License-Identifier: AGPL-3.0-or-later

default:
    @just --list

# Code generation from protobuf-ota: Prisma, store, cache, bindings, services.
[group('codegen')]
mod gen 'tools/just/gen.just'

# The ota database's Prisma migrations: migrate, deploy, status.
[group('db')]
mod db 'tools/just/db.just'

# Hasura DDN metadata regeneration after a database schema change.
[group('hasura')]
mod hasura 'tools/just/hasura.just'

# Compile everything.
[group('build')]
build:
    go build ./...

# Vet everything.
[group('build')]
vet:
    go vet ./...

# Tidy module dependencies.
[group('build')]
gotidy:
    go mod tidy

# The protos: buf format and lint (structure), then api-linter (API design),
# with the one rule api-linter.yaml disables. api-linter exits 0 on findings,
# so a non-empty table fails the recipe.
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

# Unit tests with the race detector.
[group('test')]
test:
    go test -race -count=1 ./...

# Build, vet and test: what CI runs without a database.
[group('test')]
check: build vet test
