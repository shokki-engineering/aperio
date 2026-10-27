<!-- Copyright 2026 Srikanth K. -->
<!-- SPDX-License-Identifier: AGPL-3.0-or-later -->

# aperio

The venue backend: stays, events, leads, quotes, GST invoices, payments,
documents and service requests for the venues aperio sells, on gRPC, REST and
MCP. Availability is freebusy's: every booking is a freebusy hold and
reservation first. The schema is aperio's own protos (`protobuf/aperio`),
written in the OpenTravel vocabulary of
[protobuf-ota](https://buf.build/shokki-engineering/protobuf-ota), whose types
they import.

## Layout

- The repository follows freebusy's layout (`../freebusy`): the binary at the
  root (`main.go`, `app.go`, `build.go`), `internal/config` (koanf, embedded
  `aperio.release.toml` / `aperio.dev.toml`, `APERIO_<SECTION>__<KEY>`
  overrides), `internal/telemetry`, `internal/server` (assembly, register,
  runtime), `internal/database` (connection and the generated layers),
  `test/live` for what live suites share, and command-line tools under
  `tools/`, not `cmd/`. When adding a piece freebusy already has, shape it
  as freebusy's.
- The API is one service per part of the business (`protobuf/aperio`):
  venue, catalog (dishes, menus, packages), pricing (seasons, rules,
  CalculatePrice), guest (contacts), sales (leads, quotes), stay, event,
  finance, operations, document, and concierge (the guest's tasks). A
  resource gets only the methods its lifecycle allows: bookings, quotes,
  invoices and requests move by state methods and are never deleted.
- Hand-written services live in `internal/service/<domain>`, named after the
  domain as the repository package is; services call each other through
  their gRPC methods, so each one's rules hold whoever asks. What the stay
  and event services share is `internal/booking`, a library. A file that needs both -- a test, the
  server's assembly -- takes the repositories from a helper in a file of its
  own (`test/live/<domain>.go`, `internal/server/repositories_<domain>.go`).
- What summaries, reports and agents read is a read model, as freebusy's
  availability is: a `(store.v1.projection)` message (e.g.
  `stay.v1.StayNight`) whose rows the generated layer writes in the same
  transaction as its sources, from a hand-written writer in the owning
  service (`nights.go`). Services read it through `internal/search` --
  Postgres, or its generated OpenSearch copy with Postgres behind it
  (`[search] provider`) -- never by scanning the source tables. Register a
  new one's projector in `internal/server/assembly.go` and `test/live`, and
  its rebuild and copy in `internal/server/prime.go` and `search.go`.
- Beside the services: `internal/auth` (OIDC, roles for every RPC,
  per-venue access, signed links), `internal/notify` (notifiers),
  `internal/gateway/razorpay` (payments), `internal/freebusysync`
  (freebusy's webhooks, the reconciler), `internal/channel` (sales
  channels), `internal/search` (read models and their OpenSearch copy). A
  new RPC needs a rule in `internal/auth/policy_table.go`, or its test
  fails. Webhooks are mounted with `Services.Route` (internal/server
  routes.go), beside the REST gateway -- never with runtime-go's
  `WithHTTPHandler`, which replaces it; background jobs plug into
  `internal/server/background.go`; each new `internal/config` section is
  added to `AppConfig`, `aperio.release.toml` and, when it validates,
  `load.go`.
- Deployment: `Dockerfile` (targets `runtime` and `migrate`),
  `deploy/compose` (`just stack`), `deploy/helm/aperio` (`just helm
  check`). A new secret or setting goes into the chart's values, schema
  and templates too.
- Every hand-written file stays under 200 lines and is documented.

## Go imports

- Never alias an import. Refer to a package by the name it declares:
  `bookingpbv1.Reservation` from
  `github.com/shokki-engineering/aperio/protobuf/generated/go/booking/v1/bookingpbv1`,
  `types.RoomStay` from
  `buf.build/gen/go/shokki-engineering/protobuf-ota/protocolbuffers/go/protobuf/opentravel/hotel/booking/reservation/v1/types`.
- When two imports in one file declare the same package name (every
  protobuf-ota package has a `types`, a `codes` and an `enums`), split the code
  so each file needs only one of them. Don't rename either import.

## API rules

- `just lint` runs buf format and lint, then api-linter. The only rule that may
  be disabled is `core::0215::foreign-type-reference` (api-linter.yaml), which
  importing OpenTravel's types requires; fix every other finding.

## Generated code

- Everything that touches the database is generated, from aperio's protos:
  models, stores, converters, filters, repositories, caches
  (`internal/database/`), the Hasura client aperioql, the Prisma schema, and
  the AIP service layer (`internal/service/generated/`). Hand-written code is
  business logic in hooks and custom methods, and nothing else.
- The OpenTravel types aperio's protos import are the BSR SDK's; nothing here
  generates them.
- A field whose message type a `documents` rule names is one JSONB column (the
  message's JSON form), not a table: the OpenTravel vocabulary, aperio's own
  value objects and `google.type` messages. What is searched or reported on is
  a top-level field of its resource, and so a column of its own.
- If the generated layer can't express something a service needs, extend the
  plugin (protoc-gen-store, protoc-gen-cache, protoc-gen-service, protokit)
  and regenerate. Don't edit generated output or write the query, converter or
  adapter by hand.
- `store.backend.yaml`, `cache.yaml`, `service.yaml` and `store.graphql.yaml`
  repeat one layout block; change all four together.
- The database schema is Prisma's: `just gen prisma` writes it, and
  `just db migrate <name>` writes and applies the next migration. Never edit a
  migration by hand.
- Never edit Hasura metadata (`deploy/hasura/**/*.hml`) by hand. Regenerate it
  with `just hasura regen`, and only when a schema change requires it.
- Regenerate with `just gen` (or `just gen go|prisma|store|service`); `store`
  needs the local DDN engine up, as its graphql target introspects it.

## Docs

- Every diagram is Mermaid, in a ```` ```mermaid ```` block -- architecture,
  flows, sequences, state machines -- never ASCII art: GitHub renders it, and
  it stays editable as text.

## Tests

- Test data never uses real-sounding names: John Doe (Jane Doe, "Doe
  wedding") for every person, guest, caller and title.
- `just test-live` needs Postgres, Hasura and Redis; the OpenSearch suite
  runs only when `APERIO_TEST_OPENSEARCH_URL` is set.
- Load: `just load smoke` (every endpoint once, CI-sized) and `just load
  day` (a busy day for 3 minutes) against `just stack up` -- k6, outside
  Bazel. A new task tool or guest flow gets a read or a step in
  `test/load/scenarios`, and a budget in `test/load/aperio.js` thresholds.

## Repository

- Don't commit or stage. The owner reviews the working tree and commits.
