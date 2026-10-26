# Copyright 2026 Srikanth K.
# SPDX-License-Identifier: AGPL-3.0-or-later

# Code generation from aperio's protos: Go stubs, Prisma schema, database
# layer + cache, service layer. Invoked as `just gen <recipe>` (e.g.
# `just gen store`). `just gen` runs `all`. Recipes run from the repo root so
# buf/template paths resolve unchanged.
#
# The OpenTravel types aperio's protos import are the BSR SDK's (go.mod),
# never generated here. Requires buf, protoc-gen-mcp, protoc-gen-store,
# protoc-gen-cache and protoc-gen-service on PATH.
set working-directory := '../..'

# Regenerate everything, in dependency order.
[group('codegen')]
all: go prisma store service

# The Go stubs: messages, gRPC, the HTTP gateway and the MCP bindings.
[group('codegen')]
go:
    buf generate --template tools/protobuf/buf/go.buf.gen.yaml
    go mod tidy

# The Prisma schema of the aperio database (protobuf/generated/prisma/aperio).
# The template does not clean its output -- migrations/ is Prisma's history --
# so the generated files are deleted here first, keeping migrations/,
# node_modules/ and .env. After a change, write the migration:
# `just db migrate <name>`.
[group('codegen')]
prisma:
    mkdir -p protobuf/generated/prisma/aperio
    find protobuf/generated/prisma/aperio -mindepth 1 -maxdepth 1 ! -name migrations ! -name node_modules ! -name .env -exec rm -rf {} +
    buf generate --template tools/protobuf/buf/prisma.buf.gen.yaml

# The database layer and its cache decorators: the gorm models, stores,
# converters and filters, the proto-facing repositories (over gorm and over
# Hasura), the typed Hasura client aperioql, and the read-through caches the
# protos' (cache.v1.cache) annotations select.
#
# The graphql target introspects the live DDN engine, so bring it up first
# (`just hasura regen`) or this recipe fails at that step.
[group('codegen')]
store:
    buf generate --template tools/protobuf/buf/store.buf.gen.yaml
    go mod tidy

# The service layer over the repositories: the AIP standard-method servers and
# their hooks, RegisterAll for gRPC, REST and MCP. Run after `go` and `store`.
[group('codegen')]
service:
    buf generate --template tools/protobuf/buf/service.buf.gen.yaml
    go mod tidy
