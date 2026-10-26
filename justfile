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

# Unit tests with the race detector.
[group('test')]
test:
    go test -race -count=1 ./...

# Build, vet and test: what CI runs without a database.
[group('test')]
check: build vet test
