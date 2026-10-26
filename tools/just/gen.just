# Copyright 2026 Srikanth K.
# SPDX-License-Identifier: AGPL-3.0-or-later

# Code generation: protobuf-ota -> database layer + cache, HTTP/MCP bindings,
# service layer. Invoked as `just gen <recipe>` (e.g. `just gen store`).
# `just gen` runs `all`. Recipes run from the repo root so buf/template paths
# resolve unchanged.
#
# Every input is buf.build/shokki-engineering/protobuf-ota at the commit the
# templates pin; the messages and gRPC stubs are the BSR SDK's (go.mod), never
# generated here. Requires buf, protoc-gen-store, protoc-gen-cache,
# protoc-gen-mcp and protoc-gen-service on PATH.
set working-directory := '../..'

# Regenerate everything, in dependency order.
[group('codegen')]
all: prisma store ota service

# The Prisma schema of the ota database (protobuf/generated/prisma/ota). The
# template does not clean its output -- migrations/ is Prisma's history -- so
# the generated files are deleted here first, keeping migrations/, node_modules/
# and .env. After a change, write the migration: `just db migrate <name>`.
[group('codegen')]
prisma:
    mkdir -p protobuf/generated/prisma/ota
    find protobuf/generated/prisma/ota -mindepth 1 -maxdepth 1 ! -name migrations ! -name node_modules ! -name .env -exec rm -rf {} +
    buf generate --template tools/protobuf/buf/prisma.buf.gen.yaml

# The database layer and its cache decorators: the gorm models, stores,
# converters and filters, the proto-facing repositories (over gorm and over
# Hasura), the typed Hasura client otaql, and the read-through caches
# cache.yaml's resources rules select.
#
# The graphql target introspects the live DDN engine, so bring it up first
# (`just hasura regen`) or this recipe fails at that step.
[group('codegen')]
store:
    buf generate --template tools/protobuf/buf/store.buf.gen.yaml
    go mod tidy

# The HTTP gateway and MCP bindings for every protobuf-ota service, then
# otagw points each gateway at the SDK's gRPC package and moves it into its
# <pkg>gw package.
[group('codegen')]
ota:
    buf generate --template tools/protobuf/buf/ota.buf.gen.yaml
    go run ./tools/protobuf/otagw
    go mod tidy

# The service layer over the repositories: the AIP standard-method servers and
# their hooks, RegisterAll for gRPC, REST and MCP. Run after `store`, whose
# repositories the servers persist through, and `ota`, whose bindings they
# register.
[group('codegen')]
service:
    buf generate --template tools/protobuf/buf/service.buf.gen.yaml
    go mod tidy
