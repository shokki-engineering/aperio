<!-- Copyright 2026 Srikanth K. -->
<!-- SPDX-License-Identifier: AGPL-3.0-or-later -->

# aperio

The OpenTravel (OTA) hotel backend: it serves every resource of
[protobuf-ota](https://buf.build/shokki-engineering/protobuf-ota) on gRPC, REST
and MCP, persisted through a generated database layer.

## Go imports

- Never alias an import. Refer to a package by the name it declares:
  `hotelv1.Hotel` from
  `buf.build/gen/go/shokki-engineering/protobuf-ota/protocolbuffers/go/protobuf/opentravel/hotel/hotel/v1`,
  `hotelv1grpc.HotelsServer` from its `.../hotel/v1/hotelv1grpc` package.
- When two imports in one file declare the same package name (every resource
  has a `types`, a `codes` and an `enums`), split the code so each file needs
  only one of them. Don't rename either import.

## Generated code

- protobuf-ota is the schema, consumed from the BSR at the commit the
  templates in `tools/protobuf/buf/` pin. Its messages and gRPC stubs are the
  BSR SDK's; nothing here generates them. protobuf-ota carries no annotation
  but its own, so what aperio needs to say about it -- database layout, what
  is cached, where bindings live -- is in `tools/protobuf/buf/config/`.
- Everything that touches the database is generated: models, stores,
  converters, filters, repositories, caches (`internal/database/`), and the
  AIP service layer over them (`internal/service/generated/`). Hand-written
  code is business logic in hooks and custom methods, and nothing else.
- If the generated layer can't express something a service needs, extend the
  plugin (protoc-gen-store, protoc-gen-cache, protoc-gen-service, protokit)
  and regenerate. Don't edit generated output or write the query, converter or
  adapter by hand.
- `store.backend.yaml`, `cache.yaml` and `service.yaml` repeat one layout
  block; change all three together.
- `store.graphql.yaml` repeats it too, as a fourth copy.
- The database schema is Prisma's: `just gen prisma` writes it, and
  `just db migrate <name>` writes and applies the next migration. Never edit a
  migration by hand.
- Never edit Hasura metadata (`deploy/hasura/**/*.hml`) by hand. Regenerate it
  with `just hasura regen`, and only when a schema change requires it.
- Regenerate with `just gen` (or `just gen prisma|store|ota|service`); `store`
  needs the local DDN engine up, as its graphql target introspects it.

## Repository

- Don't commit or stage. The owner reviews the working tree and commits.
