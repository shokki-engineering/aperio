<!-- Copyright 2026 Srikanth K. -->
<!-- SPDX-License-Identifier: AGPL-3.0-or-later -->

# aperio

aperio is the backend for two venues: a beach resort and a wedding and event
hall. It keeps their stays, their events and the sessions within them, their
leads and quotes (plate menus and fixed packages), seasonal pricing, GST
invoices, payments in any currency (online through Razorpay), in-house
service requests, guests, building documents, and bookings that arrive from
sales channels such as Booking.com.

It's one deployable that serves the same API over **gRPC**, **REST** and
**MCP**. Staff screens and integrations use the resource APIs. Guests on the
website, AI agents and voice agents use the **concierge**, a small set of
flat, speakable task methods that ask for consent before anything is booked.

Availability belongs to [freebusy](https://github.com/shokki-engineering/freebusy),
a separate service with its own database. Every room and hall booked here is
first a freebusy hold, and then a freebusy reservation.

What's built and what's next: [docs/roadmap.md](docs/roadmap.md). Every RPC:
[docs/api.md](docs/api.md). The voice design: [docs/voice.md](docs/voice.md).
What changed: [CHANGELOG.md](CHANGELOG.md).

## Contents

- [How it works](#how-it-works)
  - [aperio and freebusy](#aperio-and-freebusy)
  - [Storage: Postgres, Hasura, Redis, read models](#storage-postgres-hasura-redis-read-models)
  - [Generated code and hand-written code](#generated-code-and-hand-written-code)
  - [Webhooks in, background jobs](#webhooks-in-background-jobs)
  - [Auth](#auth)
  - [A guest booking, end to end](#a-guest-booking-end-to-end)
- [The API](#the-api)
  - [gRPC](#grpc)
  - [REST over HTTP and HTTPS](#rest-over-http-and-https)
  - [MCP](#mcp)
  - [Webhook endpoints](#webhook-endpoints)
- [Local development](#local-development)
- [Configuration](#configuration)
- [Production deployment](#production-deployment)
- [Project layout](#project-layout)
- [Contributing](#contributing)
- [More](#more)

## How it works

```mermaid
flowchart LR
    subgraph callers["Who calls"]
        staff["staff screens<br/>and integrations"]
        web["the website<br/>(guests)"]
        agents["AI and voice agents"]
    end
    subgraph inbound["Who calls back"]
        rzp["Razorpay"]
        bcom["Booking.com"]
    end

    staff -->|"gRPC :50061<br/>REST :8090"| aperio
    web -->|"REST :8090<br/>(concierge)"| aperio
    agents -->|"MCP :8091/mcp"| aperio
    rzp -->|"POST /webhooks/razorpay"| aperio
    bcom -->|"POST /channels/bookingcom/reservations"| aperio

    subgraph aperio["aperio (one binary)"]
        direction TB
        auth["auth: OIDC roles,<br/>per-venue access, guest links"]
        hand["hand-written business logic<br/>internal/service/*, internal/booking"]
        gen["generated: AIP services, repositories,<br/>caches, Hasura client, MCP tools"]
        jobs["background jobs:<br/>quote expiry, reconciler"]
        auth --> hand --> gen
    end

    gen -->|"GORM"| pg[("aperio Postgres<br/>Prisma migrations")]
    gen -.->|"aperioql (optional)"| ddn["Hasura DDN :3281"]
    ddn --- pg
    gen -.->|"list caches (optional)"| redis[("Redis")]
    gen -.->|"read-model copy (optional)"| os[("OpenSearch")]
    hand -->|"freebusy Go SDK (gRPC)<br/>check, hold, reserve, cancel"| fb["freebusy<br/>own Postgres"]
    fb -->|"POST /webhooks/freebusy"| aperio
```

### aperio and freebusy

aperio owns the business: who is coming, what they eat, what was quoted,
invoiced and paid. freebusy owns time: whether a room or a hall is free, and
taking it.

- **Every booking is a freebusy hold first, then a reservation.** The
  concierge's `HoldStay` places a freebusy hold that expires after a few
  minutes. `ConfirmStay` turns it into a freebusy reservation and an aperio
  `Reservation`. Event sessions book their halls the same way.
- **Two databases, no shared tables.** aperio stores only freebusy's names
  (`Reservation.freebusy_reservations`, `Session.freebusy_reservation`,
  `Space.resources`). The two sides agree through the API: idempotent booking
  keys, compensation when a step fails, freebusy's webhooks, and a reconciler
  that repairs or flags drift.
- aperio talks to freebusy over gRPC with its Go SDK (`internal/engine`). In
  development freebusy runs from its own checkout on `:50051`.

### Storage: Postgres, Hasura, Redis, read models

- **Postgres** holds aperio's data, one schema per part of the business. The
  schema is written by Prisma from the protos (`just gen prisma`), and changes
  ship as Prisma migrations (`protobuf/generated/prisma/aperio/migrations`).
  OpenTravel structures (room stays, guest lists) and other value objects are
  stored whole, as JSONB documents. Anything that's searched or reported on
  gets a column of its own.
- **Hasura DDN** puts a typed GraphQL API over that database (connector
  `aperio_pgsql`, engine `:3281`). Set `[database] provider = "hasura"` to
  make the generated repositories read through it (`aperioql`). Reporting
  and frontends can query it directly. The default, `gorm`, talks to
  Postgres directly.
- **Redis** (or Dragonfly) backs the generated list caches over venues, the
  catalog and pricing. Every write moves them on. They're off by default
  (`[cache] enabled`).
- **Read models** are what summaries and agents read instead of scanning
  every record. The first is `stay.v1.StayNight`, one row per night of every
  live stay, written in the same transaction as the reservation. It lives in
  Postgres. Set `[search] provider = "opensearch"` to keep a copy in
  OpenSearch as well; any error or timeout there falls back to Postgres.
  Read models are rebuilt at startup when empty.

### Generated code and hand-written code

Everything that touches the database is generated from aperio's protos
(`protobuf/aperio`), which import their OpenTravel types from
[protobuf-ota](https://buf.build/shokki-engineering/protobuf-ota):

| Generated | Where |
|---|---|
| Go messages, gRPC, the REST gateway, MCP tools | `protobuf/generated/go` |
| GORM models, stores, converters, filters, repositories, caches | `internal/database/` |
| The Hasura client `aperioql` | `internal/database/hasura/aperioql` |
| The Prisma schema | `protobuf/generated/prisma/aperio` |
| The AIP service layer (standard methods and their hooks) | `internal/service/generated/` |

The hand-written code is business logic only: hooks and custom methods in
`internal/service/<domain>`, and the libraries beside them (`internal/booking`,
`internal/pricing`, `internal/auth`, and so on). Services call each other
through their gRPC methods, so each service's rules hold no matter who calls.

### Webhooks in, background jobs

Three plain HTTP endpoints sit on the REST port beside the gateway:

| Endpoint | From | Does |
|---|---|---|
| `POST /webhooks/razorpay` | Razorpay | settles payments made through payment links, and refunds |
| `POST /webhooks/freebusy` | freebusy | settles stays and events cancelled or moved in freebusy |
| `POST /channels/bookingcom/reservations` | Booking.com | books, changes and cancels stays from `OTA_HotelResNotifRQ` |

Details, signing and setup: [Webhook endpoints](#webhook-endpoints).

Background jobs run inside the server:

| Job | Default interval | Setting |
|---|---|---|
| Quote expiry: sent quotes past their `expire_time` are expired, and their holds released | 1 minute | `[freebusy_sync] quote_expiry_interval_seconds` |
| freebusy reconciler: walks live bookings and repairs or flags any that disagree with freebusy | 5 minutes | `[freebusy_sync] reconcile_interval_seconds` |
| Postgres pool monitor: reports the pool's health through telemetry | always on | none |

A negative interval turns a job off. Read-model rebuilds and OpenSearch
reindexing also run in the background at startup (`rebuild_on_start`,
`reindex_on_start`).

### Auth

Auth is **off until `[auth] issuer` is set**. While it's off, the server logs a
loud warning and lets every call through. Don't expose an aperio like that
to a network.

- **Staff** sign in with any OIDC issuer (Google Workspace, Keycloak, Clerk)
  and send `Authorization: Bearer <token>`. The token's `roles` claim maps
  to six roles: `owner`, `front_desk`, `sales`, `kitchen`, `housekeeping`
  and `accounts`. Every RPC has a rule in `internal/auth/policy_table.go`,
  and a test fails if one is missing. Who may call what:
  [docs/api.md](docs/api.md#who-may-call-what).
- **Per-venue access:** the `venues` claim (`venues/<id>`, or `*`) limits a
  person to the resources under their venues.
- **Guests** use the ConciergeService, which is public. A guest can also hold
  a **signed link** to one quote, reservation or invoice
  (`Authorization: Link <token>`), signed with `[auth] link_secret`.
- One gRPC interceptor enforces all of it, for gRPC, for REST (the gateway
  forwards `Authorization`) and for MCP (the header on each MCP HTTP request
  is lifted onto the call).

### A guest booking, end to end

John Doe books a sea-view room on the website or by phone. Every step that
takes a room or money is two steps: a reversible hold, then a confirm the
guest agrees to.

```mermaid
sequenceDiagram
    actor Guest as John Doe
    participant C as ConciergeService
    participant FB as freebusy
    participant Stay as StayService
    participant Desk as Front desk / website
    participant Fin as FinanceService
    participant RZP as Razorpay

    Guest->>C: FindStay {arrival 2027-12-12, 2 nights, 2 adults}
    C->>FB: availability for each room type
    C-->>Guest: options, cheapest first, with spoken text
    Guest->>C: HoldStay {space, request_id}
    C-->>Guest: elicitation: "May I have the guest's name and phone number?"
    Guest->>C: HoldStay again, with name and phone
    C->>FB: hold the rooms (expires in minutes)
    C-->>Guest: hold_token, total, expire_time
    Guest->>C: ConfirmStay {hold_token}
    C-->>Guest: elicitation (consent): "Book 1 room of Sea View Deluxe for 2 nights ... plus taxes?"
    Guest->>C: ConfirmStay {hold_token, confirmed: true}
    C->>Stay: CreateReservation (finds or adds the contact)
    Stay->>FB: the hold becomes a reservation
    C-->>Guest: reservation PENDING, reference "7F3A9C"
    Desk->>Fin: CreateInvoice (proforma for the booking), IssueInvoice
    Desk->>Fin: RequestPayment
    Fin->>RZP: create a payment link
    Fin-->>Desk: url, sent to the guest
    Guest->>RZP: pays the advance
    RZP->>Fin: POST /webhooks/razorpay (payment_link.paid, signed)
    Fin->>Fin: payment SUCCEEDED, receipt voucher, invoice paid and due
    Fin->>Stay: the advance confirms the reservation
    Stay-->>Guest: CONFIRMED
```

Today the payment link is requested by staff or the frontend after
`ConfirmStay`. The concierge doesn't send it yet, although its spoken reply
says payment details will follow ([roadmap](docs/roadmap.md), M9).

## The API

The API is one service per part of the business, **109 methods** in all.
Each one is served on gRPC, REST and MCP. The full tables, with every REST
route, are in **[docs/api.md](docs/api.md)**.

| Service | Package | Methods | Covers |
|---|---|---|---|
| VenueService | `aperio.venue.v1` | 8 | venues and their spaces (room types, halls) |
| CatalogService | `aperio.catalog.v1` | 12 | dishes, menus, fixed packages |
| PricingService | `aperio.pricing.v1` | 11 | seasons, price rules, `CalculatePrice` |
| GuestService | `aperio.guest.v1` | 5 | contacts, `SummarizeContact` |
| SalesService | `aperio.sales.v1` | 16 | leads, activities, quotes (Send, Accept, Decline, Revise), `SummarizePipeline` |
| StayService | `aperio.stay.v1` | 9 | reservations (Confirm, Cancel, Arrive, Depart), `SummarizeDay` |
| EventService | `aperio.event.v1` | 12 | events (Confirm, Cancel), sessions, `SummarizeSessions` |
| FinanceService | `aperio.finance.v1` | 16 | GST invoices (Issue, Cancel, Credit), payments (Refund, Void, Clear), `RequestPayment`, `SummarizeReceivables` |
| OperationsService | `aperio.operations.v1` | 8 | service requests (Acknowledge, Start, Complete, Cancel), `SummarizeServiceRequests` |
| DocumentService | `aperio.document.v1` | 5 | building documents (metadata) |
| ConciergeService | `aperio.concierge.v1` | 7 | the guest's tasks (below) |

A resource gets only the methods its lifecycle allows. Bookings, quotes,
invoices, payments and requests move through state methods and are never
deleted.

**The concierge**, for guests and agents:

| Method | Does | REST |
|---|---|---|
| FindStay | room types free for the dates, priced, cheapest first | `POST /v1/venues/{venue}:findStay` |
| HoldStay | holds rooms in freebusy for a few minutes; returns a `hold_token` | `POST /v1/venues/{venue}:holdStay` |
| ConfirmStay | books the hold, after the guest's consent | `POST /v1/venues/{venue}:confirmStay` |
| FindBooking | a guest's stays, by phone plus reference or arrival date | `POST /v1/venues/{venue}:findBooking` |
| CancelBooking | cancels a stay, after the guest's consent | `POST /v1/venues/{venue}:cancelBooking` |
| EnquireEvent | an event enquiry becomes a lead and a draft quote | `POST /v1/venues/{venue}:enquireEvent` |
| RequestService | "two black coffees to room 204" to the right department | `POST /v1/venues/{venue}:requestService` |

**Task tools**, for staff and their agents. Each one answers a whole question
in a few indexed reads: `SummarizeDay`, `SummarizeSessions`,
`SummarizeContact`, `SummarizePipeline`, `SummarizeReceivables` and
`SummarizeServiceRequests`.

### gRPC

- Port **50061**, plaintext unless `[server.tls]` is set.
- Service names are `aperio.<service>.v1.<Service>`, for example
  `aperio.stay.v1.StayService`. The protos are in `protobuf/aperio`.
- Server reflection is on in the `development` and `debug` environments
  (the dev profile is `debug`), so `grpcurl` works without the protos:

```sh
grpcurl -plaintext localhost:50061 list
grpcurl -plaintext -d '{"venue": "venues/sbr", "date": {"year": 2027, "month": 12, "day": 12}}' \
  localhost:50061 aperio.stay.v1.StayService/SummarizeDay
```

- Requests are validated (protovalidate) before any handler runs.
- Auth goes in the `authorization` metadata: `Bearer <token>`.

### REST over HTTP and HTTPS

- Base URL: `http://localhost:8090` in development. Every route is under
  `/v1/`, and `GET /health` answers `{"status":"ok", ...}`.
- JSON fields are **lowerCamelCase** (`displayName`, `arrivalDate`,
  `pageSize`), and unset fields are written out with their defaults. Dates
  are `google.type.Date` objects: `{"year": 2027, "month": 12, "day": 12}`.
- Resource names are paths: `venues/sbr`, `venues/sbr/reservations/r-7f3a9c`,
  `contacts/john-doe`.
- Custom methods use a colon: `POST /v1/venues/sbr/quotes/q-1:send`.

```sh
curl -s localhost:8090/health
curl -s 'localhost:8090/v1/venues?pageSize=10'

# Rooms for John Doe: 12 December 2027, two nights, two adults
curl -s -X POST localhost:8090/v1/venues/sbr:findStay \
  -H 'content-type: application/json' \
  -d '{"arrivalDate": {"year": 2027, "month": 12, "day": 12}, "nights": 2, "adults": 2}'

# Hold the first option (request_id makes a retry safe)
curl -s -X POST localhost:8090/v1/venues/sbr:holdStay \
  -H 'content-type: application/json' \
  -d '{"space": "venues/sbr/spaces/sea-view-deluxe",
       "arrivalDate": {"year": 2027, "month": 12, "day": 12}, "nights": 2, "adults": 2,
       "guestDisplayName": "John Doe", "phone": "+919800000000",
       "requestId": "3f0c9a52-6a8e-4c1e-9d1b-2f7c1e0b4a11"}'

# A staff call, once auth is on
curl -s localhost:8090/v1/venues/sbr/reservations:summarizeDay \
  -H "Authorization: Bearer $TOKEN"
```

**Errors** are a `google.rpc.Status` in JSON. `error.status` is the gRPC
code, and `details` carry an `ErrorInfo` with a stable `reason` (domain
`aperio.dev`), plus a `BadRequest` naming the field at fault, a
`PreconditionFailure` naming the resource whose state blocks the call, or a
`ResourceInfo` for a missing one. The message says what's wrong and then what
to do. This one came from a running server:

```json
{
  "error": {
    "code": 400,
    "status": "INVALID_ARGUMENT",
    "message": "The hold token is not one this venue gave. Hold the stay again with HoldStay, and pass the token it returns.",
    "details": [
      {"@type": "type.googleapis.com/google.rpc.ErrorInfo",
       "reason": "INVALID_HOLD_TOKEN", "domain": "aperio.dev", "metadata": {}},
      {"@type": "type.googleapis.com/google.rpc.BadRequest",
       "fieldViolations": [{"field": "hold_token", "reason": "INVALID_HOLD_TOKEN",
         "description": "the hold token is not one this venue gave. Hold the stay again with HoldStay, and pass the token it returns"}]}
    ]
  }
}
```

Branch on `reason`, not on the message. The generated reasons
(`NOT_FOUND`, `ALREADY_EXISTS`, `CONCURRENT_MODIFICATION`, `REQUIRED_FIELD`,
`INVALID_STATE`, and so on) are listed with what to do about each in
`internal/service/generated/servicex/errors.svc.md`.

When the concierge needs something from the guest, the error's `ErrorInfo`
has reason `ELICITATION_REQUIRED` (domain `mcp.the-protobuf-project.org`).
Its `metadata.message` is the question and `metadata.requested_schema` is a
JSON Schema of the fields to ask for (`guest_display_name`, `phone`, or
`confirmed`). A frontend shows it as a form and calls again with the answers.

**CORS:** off by default. To let a browser app on another origin call the
REST port:

```sh
APERIO_SERVER__CORS__ENABLED=true \
APERIO_SERVER__CORS__ALLOWED_ORIGINS=https://app.example.com \
just run
```

CORS covers the REST port only. The MCP port is for agents, not browser
pages.

**TLS:** set a certificate and key, and gRPC, REST and MCP are all served
over TLS. A bad path stops the server at startup with a clear error.

```sh
APERIO_SERVER__TLS__CERT_FILE=/etc/aperio/tls.crt \
APERIO_SERVER__TLS__KEY_FILE=/etc/aperio/tls.key \
./aperio
```

In Kubernetes, terminate TLS at the Ingress instead (see
[Helm](#the-helm-chart)).

**OpenAPI:** none is generated today. The protos and
[docs/api.md](docs/api.md) are the reference.

### MCP

- Endpoint: **`http://localhost:8091/mcp`**, streamable HTTP. All eleven
  services' tools are served together at that one path: **109 tools**,
  about 0.9 MB of schemas for `tools/list`.
- **Stateless** (`[server.mcp] stateless = true`). There's no session to
  keep, so every request stands alone and the server scales out behind a
  plain load balancer.
- **Tool names** are `<service>_service-<method in snake_case>_v1`, for
  example `concierge_service-find_stay_v1`, `stay_service-summarize_day_v1`
  and `finance_service-request_payment_v1`.
- **Arguments** use the proto field names (snake_case: `arrival_date`,
  `hold_token`), and every argument is validated in full on the server.
- Schemas are trimmed for an agent's context
  (`tools/protobuf/buf/go.buf.gen.yaml`): shared types appear once under
  `$defs`, OpenTravel code lists over 64 values become patterns, nested
  descriptions are short, and there are no output schemas.

**Connecting Claude Code:**

```sh
claude mcp add --transport http aperio http://localhost:8091/mcp
# with auth on, send a staff token on every request
claude mcp add --transport http aperio https://aperio.example.com/mcp \
  --header "Authorization: Bearer $TOKEN"
```

Any MCP client that speaks streamable HTTP connects the same way: point it
at the URL and, when auth is on, have it send `Authorization: Bearer <token>`
on each HTTP request. The concierge tools need no token. A quick check
without a client:

```sh
curl -s -X POST localhost:8091/mcp \
  -H 'content-type: application/json' -H 'accept: application/json, text/event-stream' \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}' | head -c 300
```

**Elicitation.** The concierge asks instead of failing:

- `HoldStay` and `EnquireEvent` ask only for whichever of the guest's name
  and phone is missing.
- `ConfirmStay` and `CancelBooking` ask for **consent**, with the real room,
  dates and amount in the question. Nothing is booked or cancelled without
  `confirmed: true`.

How the question arrives depends on the client:

| Client | Gets |
|---|---|
| MCP 2026-07-28 that declares `elicitation` in its request `_meta` | an `input_required` result; it calls again with `inputResponses` |
| older MCP that declares `elicitation` at initialize | an elicitation over its session |
| MCP without elicitation | a tool error with the question, which the model asks in its own words before calling again |
| REST or gRPC | the `ELICITATION_REQUIRED` error described above |

A declined answer ends the call without running the handler again. More in
[docs/voice.md](docs/voice.md).

### Webhook endpoints

All three are served on the **REST port (8090)**, so they go through the same
TLS and Ingress as the REST API. None of them uses OIDC: each one checks its
own signature or login.

#### `POST /webhooks/razorpay`

- **Does:** settles the payments behind `RequestPayment`'s links and the
  refunds behind `RefundPayment`. `payment_link.paid` and `payment.captured`
  succeed a payment; `payment_link.expired` and `payment_link.cancelled`
  fail one; `refund.processed` and `refund.failed` settle a refund. A
  settled payment updates its invoice's paid and due amounts and confirms a
  PENDING stay or a TENTATIVE event.
- **Signed:** `X-Razorpay-Signature` is the hex HMAC-SHA256 of the raw body
  under the webhook secret, checked in constant time. A bad signature gets
  401. Settling is idempotent, so a redelivery gets 200.
- **Enable:** set `[razorpay]` `key_id`, `key_secret` and `webhook_secret`.
  The endpoint is mounted only when `key_id` is set. In the Razorpay
  dashboard, add a webhook to `https://<your-host>/webhooks/razorpay` with
  the same secret and the events above.

```sh
APERIO_RAZORPAY__KEY_ID=rzp_test_xxxxxxxx \
APERIO_RAZORPAY__KEY_SECRET=... \
APERIO_RAZORPAY__WEBHOOK_SECRET=... \
just run
```

#### `POST /webhooks/freebusy`

- **Does:** when a reservation is cancelled or moved in freebusy (by an
  operator, or over CalDAV), the matching aperio stay or event is settled.
  Answers 204, or 500 so that freebusy retries.
- **Signed:** Standard Webhooks. `webhook-signature` is
  `v1,<base64 HMAC-SHA256>` of `<webhook-id>.<webhook-timestamp>.<body>`
  under the subscription's `whsec_` secret. Timestamps more than five
  minutes off are refused, and delivery ids are de-duplicated.
- **Enable:** set `[freebusy_sync] webhook_url` to a URL freebusy can reach,
  such as `https://aperio.example.com/webhooks/freebusy`. At startup aperio
  makes sure the subscription `webhooks/aperio` exists in freebusy. Put the
  `whsec_` secret that freebusy returned in
  `APERIO_FREEBUSY_SYNC__WEBHOOK_SECRET`, so that every replica verifies
  with the same one. With no URL, no subscription is made, but the
  reconciler still runs.

#### `POST /channels/bookingcom/reservations`

- **Does:** takes Booking.com's `OTA_HotelResNotifRQ` (new, modified or
  cancelled reservations) and books, changes or cancels the stay through
  the stay and guest services. It's idempotent on Booking.com's reference
  and answers `OTA_HotelResNotifRS`.
- **Authenticated:** HTTP basic auth. With either credential empty, every
  message is refused.
- **Enable:** set `[channels.bookingcom] enabled = true`, a username and
  password, and map each Booking.com hotel code to a venue and its room
  codes to spaces:

```toml
[channels.bookingcom]
enabled = true

[[channels.bookingcom.hotels]]
hotel_code = "7777001"
venue = "venues/sbr"

[[channels.bookingcom.hotels.rooms]]
code = "SV-DLX"
space = "venues/sbr/spaces/sea-view-deluxe"
rate_plan = "BAR"
```

The Booking.com adapter is built but not yet certified: there's no pull
poller or outbound sender yet ([roadmap](docs/roadmap.md), M7).

## Local development

### Prerequisites

| Tool | For |
|---|---|
| Go **1.26.10** (`go.mod`) | building and testing |
| [just](https://just.systems) | every recipe below |
| [buf](https://buf.build) and `api-linter` (v2) | `just lint`, `just gen` |
| `protoc-gen-mcp`, `protoc-gen-store`, `protoc-gen-cache`, `protoc-gen-service` on `PATH` | `just gen` (CI installs them with `go install`; see `.github/workflows/ci.yaml`) |
| Node 22 and npm | Prisma (`just db ...`) |
| Docker | Postgres, the Compose stack, Hasura |
| [ddn CLI](https://hasura.io/docs/3.0/cli/installation) | the local Hasura engine, `just hasura regen`, `just gen store` |
| Bazel via bazelisk (`.bazelversion`: 9.3.0) | images and `just bazel-check` |
| [k6](https://k6.io) | load tests |
| Helm | `just helm ...` |
| a freebusy checkout | booking: `just run` there serves gRPC on `:50051` |

`just` with no arguments lists every recipe, and `just --list <module>` lists
a module's (`db`, `gen`, `hasura`, `stack`, `helm`, `load`).

### 1. Database

```sh
just db up         # Postgres 18 on 127.0.0.1:5433, database aperio (password postgrespassword)
just db install    # the Prisma CLI, once
cp protobuf/generated/prisma/aperio/.env.example protobuf/generated/prisma/aperio/.env
# set APERIO_DATABASE_URL="postgresql://postgres:postgrespassword@localhost:5433/aperio" in that .env
just db deploy     # apply the migrations
just db status     # what has been applied
```

`just db reset` drops and recreates the dev database (it asks first).

### 2. Hasura DDN (optional)

You need Hasura only for `[database] provider = "hasura"`, the Hasura live
suites, and `just gen store`. Either run the stack's engine:

```sh
just stack up hasura
```

or run the DDN project in `deploy/hasura` with the ddn CLI:

```sh
cd deploy/hasura && cp .env.example .env
ddn supergraph build local && ddn run docker-start -- -d   # engine on :3281
```

After a migration that adds or renames tables, regenerate the metadata and
the `aperioql` client. Don't edit `.hml` files by hand.

```sh
just hasura regen            # full clean regen
just hasura regen stay       # one domain, for an additive change
```

### 3. freebusy

Booking needs freebusy. Run `just run` in its checkout (gRPC `:50051`, REST
`:8080`). Its dev server uses mkcert's certificate, so point aperio at the CA,
or skip TLS when freebusy runs without it:

```sh
export APERIO_FREEBUSY__CA_FILE="$(mkcert -CAROOT)/rootCA.pem"
# or: export APERIO_FREEBUSY__INSECURE=true
```

### 4. Run the server

```sh
just run      # go run . : gRPC :50061, REST :8090, MCP :8091/mcp
curl -s localhost:8090/health
curl -s 'localhost:8090/v1/venues?pageSize=10'
```

In a checkout, `internal/config/aperio.dev.toml` is layered over the release
defaults automatically: Postgres on 5433 without TLS, freebusy on
`localhost:50051`, and the `debug` environment. Override anything from the
environment:

```sh
APERIO_SERVER__HTTP__PORT=8095 just run
```

`just build` compiles everything and writes `bin/aperio`.

### 5. The whole stack in Compose

`deploy/compose/compose.yaml` runs aperio, freebusy, both Postgres
databases and their migrations. Profiles add Hasura (`hasura`), Redis
(`cache`) and OpenSearch (`search`).

```sh
cp deploy/compose/.env.example deploy/compose/.env   # replace every CHANGE_ME
just image                     # build aperio's images from this checkout (or let Compose pull them)
just stack up                  # the core stack, waits for health
just stack up cache search hasura
just stack ps
just stack logs aperio
just stack down                # stop, keeping volumes
just stack destroy             # DEV ONLY: also delete the volumes (asks first)
```

Every host port is bound to `127.0.0.1` and can be changed in `.env`: aperio
50061 / 8090 / 8091, its Postgres 5433, freebusy 50051 / 8080 / 8081, the
Hasura engines 3281 (aperio) and 3280 (freebusy), and OpenSearch 9200. The
example `.env` turns CORS on for `http://localhost:5173`. Auth stays off
until you set `APERIO_AUTH_ISSUER`.

### 6. Tests

```sh
just test        # unit tests with -race; live suites skip
just check       # build, vet and unit tests: what CI runs without a database
just test-live   # every suite against Postgres, Hasura and Redis
```

`just test-live` needs the dev Postgres with migrations applied
(`just db up`, `just db deploy`), the Hasura engine on `:3281`, and Redis on
`127.0.0.1:6379` (`just stack up cache`, or any Redis). Any suite that can't
reach its backend fails instead of skipping. The defaults can be overridden:

```sh
just test-live "host=127.0.0.1 port=5433 user=postgres password=postgrespassword dbname=aperio sslmode=disable" \
  http://localhost:3281/graphql 127.0.0.1:6379
```

The OpenSearch suite runs only when `APERIO_TEST_OPENSEARCH_URL` is set:

```sh
APERIO_TEST_OPENSEARCH_URL=http://localhost:9200 go test ./internal/service/stay/ -run OpenSearch
```

### 7. Lint and generate

```sh
just lint        # buf format and lint, then api-linter
just gen         # everything: go, prisma, store, service
just gen go      # or one target: go | prisma | store | service
just db migrate add_guest_notes   # after a schema change: write and apply the next migration
just fmt         # gofmt the hand-written Go
```

`just gen store` introspects the running Hasura engine, so bring it up
first. After adding or moving a Go package, regenerate the Bazel BUILD files
and check that everything builds there too:

```sh
bazel run //:gazelle    # or: just gazelle
just bazel-check        # bazel build //... and bazel test //...
```

### 8. Load tests

The k6 test (`test/load`, outside Bazel) runs staff reads, guest bookings
through real freebusy holds, in-house requests and MCP agents all at once,
against the Compose stack:

```sh
just stack up               # or: just stack up cache, to measure with Redis
just load smoke             # every endpoint once, with the busy-day thresholds
just load day               # a busy day, steady, for 3 minutes
just load stress            # ramping to five times a busy day
just load soak              # a busy day for 30 minutes
just load day 2 10m         # twice a busy day for 10 minutes
```

Point it elsewhere with `APERIO_URL`, `MCP_URL` and `FREEBUSY_URL`. Each
run's summary goes to `test/load/results/`. The thresholds (for example
`FindStay` p95 under 500 ms, `SummarizeDay` p95 under 200 ms) are in
`test/load/aperio.js`.

## Configuration

Settings come in three layers, each overriding the one before:

1. `internal/config/aperio.release.toml`, the production defaults, compiled
   into the binary. Secrets are blank.
2. `internal/config/aperio.dev.toml`, the development profile. It's read from
   the working directory in a checkout, or from the embedded copy with
   `APERIO_PROFILE=dev`. `APERIO_PROFILE=release` skips it even in a checkout.
3. Environment variables: `APERIO_<SECTION>__<KEY>`, with a double underscore
   between levels.

```sh
APERIO_DATABASE__POSTGRES__HOST=db \
APERIO_DATABASE__POSTGRES__PASSWORD=... \
APERIO_FREEBUSY__ENDPOINT=freebusy:50051 \
APERIO_AUTH__ISSUER=https://keycloak.example.com/realms/aperio \
./aperio
```

Every section, its main keys, and **which keys are secrets** (always set
secrets through the environment or a Kubernetes Secret, never in a file):

| Section | Key settings (default) | Secrets |
|---|---|---|
| `[app]` | `version` (must match the release tag) | |
| `[server]` | `environment` (`production`), `enable_http`, `enable_mcp`, `enable_health`, `shutdown_timeout_seconds` (20) | |
| `[server.grpc]` / `[server.http]` / `[server.mcp]` | ports 50061 / 8090 / 8091; MCP `path` `/mcp`, `stateless` true | |
| `[server.cors]` | `enabled` (false), `allowed_origins`, `allowed_methods`, `allowed_headers`, `allow_credentials`, `max_age_seconds` | |
| `[server.tls]` | `cert_file`, `key_file` (empty: no TLS) | the key file |
| `[database]` | `provider`: `gorm` (Postgres directly) or `hasura` | |
| `[database.postgres]` | `host`, `port` 5432, `user`, `dbname` `aperio`, `sslmode` `require`, pool sizes | `password` |
| `[database.hasura]` | `url` | `admin_secret` |
| `[cache]` | `enabled` (false), `address`, `database`, `prefix` | `password` |
| `[freebusy]` | `endpoint`, `insecure`, `ca_file`, `timeout_seconds`, `public_url` | `feeds_secret` |
| `[concierge]` | | `hold_secret`: seals hold tokens. Set the same value on every replica. |
| `[search]` | `provider` (`postgres` or `opensearch`), `rebuild_on_start` (`empty`) | |
| `[search.opensearch]` | `url`, `timeout_ms` (500), `reindex_on_start`, `index_queue` | `username`, `password` |
| `[notify]` | `provider`: `log`, `webhook` (signed `X-Aperio-Signature`) or `whatsapp`; `[notify.departments]` maps a department to a URL or number | `webhook.secret`, `whatsapp.token` |
| `[auth]` | `issuer` (empty: **auth off**), `audience`, `roles_claim`, `venues_claim`, `clock_skew_seconds`, `[auth.role_map]` | `link_secret` (32+ random bytes, the same on every replica) |
| `[channels.bookingcom]` | `enabled`, `hotels` (hotel code to venue, room codes to spaces) | `username`, `password` |
| `[razorpay]` | `base_url`, `timeout_seconds` (off while `key_id` is empty) | `key_id`, `key_secret`, `webhook_secret` |
| `[freebusy_sync]` | `webhook_url`, `webhook_id` (`aperio`), `reconcile_interval_seconds`, `quote_expiry_interval_seconds` | `webhook_secret` |

Telemetry is set through its own variables: `APERIO_OTLP_ENDPOINT` (an OTLP
gRPC collector, `host:port`), `APERIO_OTLP_SECURE`,
`APERIO_TELEMETRY_ENVIRONMENT`, `APERIO_SITE` and `APERIO_SITE_NAME`. aperio
exports logs, traces and metrics over OTLP. It doesn't serve `/metrics` yet.

## Production deployment

### Images

Images are built with **Bazel and rules_oci** (`deploy/image/BUILD.bazel`).
There's no Dockerfile. Each image is built for linux/amd64 and linux/arm64:

| Image | Contents |
|---|---|
| `ghcr.io/shokki-engineering/aperio` | the static server binary alone, on scratch (no shell) |
| `ghcr.io/shokki-engineering/aperio-migrate` | Prisma 7.10.1 and the migrations on distroless Node, running as uid 65532; its entrypoint runs `prisma migrate deploy` against `APERIO_DATABASE_URL` |

```sh
just image     # load both into Docker, for this machine's platform, as :latest
```

`just image` runs `bazel run //deploy/image:load` and
`//deploy/image:load_migrate`. CI and the release workflow push with
`bazel run --config=release //deploy/image:push` (and `:push_migrate`), which
stamps the binary with the tag and commit (`aperio --version`). The
migrate image's Prisma lockfile lives in `deploy/image/prisma`, and
Dependabot watches it there. Rewrite it with `just prisma-lock` after you
change its `package.json`.

### The Helm chart

The chart is `deploy/helm/aperio`. It runs the server and, as a
**pre-install and pre-upgrade hook**, a Job that applies the Prisma
migrations before any new pods start. Postgres and freebusy are yours to
provide: use a managed database, and deploy freebusy from its own chart.

Release tags publish the chart to `oci://ghcr.io/shokki-engineering/charts/aperio`.
Until the first release, install it from a checkout:

```sh
kubectl create namespace aperio
kubectl -n aperio create secret generic aperio \
  --from-literal=postgres-password="$(openssl rand -hex 24)" \
  --from-literal=concierge-hold-secret="$(openssl rand -hex 32)" \
  --from-literal=auth-link-secret="$(openssl rand -hex 32)"

cat > aperio-values.yaml <<'EOF'
database:
  postgres:
    host: postgres.db.svc
    user: aperio
secrets:
  existingSecret: aperio
freebusy:
  endpoint: freebusy.freebusy.svc:50051
  insecure: true
auth:
  issuer: https://keycloak.example.com/realms/aperio
  audience: aperio
EOF

helm install aperio deploy/helm/aperio -n aperio -f aperio-values.yaml
# after a release: helm install aperio oci://ghcr.io/shokki-engineering/charts/aperio --version <x.y.z> -n aperio -f aperio-values.yaml
helm test aperio -n aperio      # /health and a read through the Service
```

**Required values:** `database.postgres.host` and the Postgres password (the
`postgres-password` key). In practice you also need `freebusy.endpoint`,
since nothing can be booked without it, and `auth.issuer` before anything
faces a network. `just helm check` runs lint, a values-schema test and
kubeconform. `just helm template full` renders every optional part.

**Secrets.** Name an existing Secret (`secrets.existingSecret`, recommended),
or let the chart make one from `secrets.*`. The Secret's keys and what each
becomes:

| Key | Becomes | Needed |
|---|---|---|
| `postgres-password` | `APERIO_DATABASE__POSTGRES__PASSWORD` | always |
| `database-url` | the migration Job's `APERIO_DATABASE_URL` (else built from `database.postgres`) | optional |
| `concierge-hold-secret` | `APERIO_CONCIERGE__HOLD_SECRET` | with more than one replica (32+ bytes) |
| `auth-link-secret` | `APERIO_AUTH__LINK_SECRET` | for guest links (32+ bytes) |
| `hasura-admin-secret` | `APERIO_DATABASE__HASURA__ADMIN_SECRET` | with `database.provider: hasura` |
| `redis-password` | `APERIO_CACHE__PASSWORD` | with the cache |
| `opensearch-username`, `opensearch-password` | `APERIO_SEARCH__OPENSEARCH__USERNAME` / `__PASSWORD` | with OpenSearch |
| `freebusy-feeds-secret` | `APERIO_FREEBUSY__FEEDS_SECRET` | for calendar feed links |
| `freebusy-ca.crt` | mounted as `APERIO_FREEBUSY__CA_FILE` | with `freebusy.ca.enabled` |
| `notify-webhook-secret`, `whatsapp-token` | `APERIO_NOTIFY__WEBHOOK__SECRET`, `APERIO_NOTIFY__WHATSAPP__TOKEN` | with that notifier |
| `razorpay-key-id`, `razorpay-key-secret`, `razorpay-webhook-secret` | `APERIO_RAZORPAY__*` | for online payments |
| `bookingcom-username`, `bookingcom-password` | `APERIO_CHANNELS__BOOKINGCOM__*` | for Booking.com |

Any other setting goes in `extraEnv` as `APERIO_<SECTION>__<KEY>: value`.
`extraEnv` lands in a ConfigMap, so don't put secrets in it.

**Ingress.** The Service exposes `grpc` (50061), `http` (8090) and `mcp`
(8091). There are two Ingresses, both off by default:

```yaml
ingress:
  className: nginx
  http:                       # REST, /health and the webhooks at /, MCP at mcpPath
    enabled: true
    hosts: [aperio.example.com]
    mcpPath: /mcp
    tls: [{secretName: aperio-tls, hosts: [aperio.example.com]}]
  grpc:                       # HTTP/2 to the backend (ingress-nginx's annotation is the default)
    enabled: true
    hosts: [grpc.aperio.example.com]
    tls: [{secretName: aperio-grpc-tls, hosts: [grpc.aperio.example.com]}]
```

The chart also has an HPA, a PodDisruptionBudget, topology spread, a
NetworkPolicy and a ServiceMonitor (which stays off until aperio serves
`/metrics`). Its probes `GET /health` on the http port.

### Before facing the internet

- **Auth:** set `auth.issuer` and `auth.audience` (`[auth]`), and map your
  issuer's groups to roles if they differ (`APERIO_AUTH__ROLE_MAP__<GROUP>`).
  While the issuer is empty, every call is let through.
- **Secrets:** a real Postgres password, `concierge-hold-secret` and
  `auth-link-secret` (`openssl rand -hex 32`, the same on every replica),
  and the gateway and channel secrets for whatever you enable.
- **TLS:** terminate at the Ingress, or set `[server.tls]`. Use
  `sslmode: require` or stricter to reach Postgres.
- **CORS:** list your frontend's exact origins (`cors.allowedOrigins`). Don't
  combine `*` with credentials.
- **freebusy:** reach it over TLS (`freebusy.ca`) unless it's inside the
  cluster, and set `[freebusy_sync] webhook_url` and `webhook_secret` so
  that changes made in freebusy show up in aperio.

### Releasing

A release is a git tag `vX.Y.Z` on a commit that has already passed CI on
`main`:

1. Move **Unreleased** in `CHANGELOG.md` under `## [X.Y.Z] - YYYY-MM-DD`.
2. Set `[app] version = "vX.Y.Z"` in `internal/config/aperio.release.toml`.
3. Tag and push. `.github/workflows/release.yaml` checks that the tag
   matches the config and the changelog. It then builds and pushes both
   images with Bazel (`:X.Y.Z`, plus `:latest` unless it's a pre-release),
   attaches SLSA provenance and SPDX SBOMs, packages and pushes the chart to
   `oci://ghcr.io/shokki-engineering/charts/aperio`, signs everything
   keylessly with cosign, and creates a GitHub release from the changelog
   section.

CI (`.github/workflows/ci.yaml`) runs on pushes to `main` and on pull requests: gofmt,
vet and buf lint; buf breaking; a check that generated code hasn't drifted;
the Go build; Bazel build, tests and images (it runs the image); the Helm
chart check; unit tests; and the live suites against Postgres and Redis.
`api-lint.yaml` runs `just lint`.

## Project layout

aperio follows freebusy's layout.

| Path | What |
|---|---|
| `main.go`, `app.go`, `build.go` | the binary: telemetry, config, server lifecycle, version |
| `internal/config` | `aperio.release.toml`, `aperio.dev.toml`, `APERIO_` overrides |
| `internal/server` | the HybridServer: services assembled and registered on gRPC, REST and MCP; webhook routes; background jobs |
| `internal/service/<domain>` | business logic: `stay`, `event`, `guest`, `pricing`, `sales`, `finance`, `operations`, `concierge` |
| `internal/service/generated` | the generated AIP services |
| `internal/booking` | what the stay and event services share in taking rooms and halls from freebusy |
| `internal/database` | the connection (GORM or Hasura) and the generated models, repositories, caches and Hasura client |
| `internal/auth` | OIDC, the role policy, per-venue access, signed links |
| `internal/engine`, `internal/freebusysync` | freebusy through its SDK; its webhooks and the reconciler |
| `internal/gateway/razorpay`, `internal/channel`, `internal/notify` | payments, sales channels, notifications |
| `internal/search`, `internal/pricing`, `internal/calendar` | read models and OpenSearch; price rules, money and GST; calendar notes |
| `internal/telemetry` | logs, traces and metrics (OTLP) |
| `protobuf/aperio` | the protos; `protobuf/generated` holds their Go, Prisma and MCP output |
| `test/live`, `test/load` | what live suites share; the k6 load test |
| `tools/` | `just` modules, buf templates, Bazel helpers, the calendar-links tool (`just calendar-links`) |
| `deploy/` | `compose` (the whole stack), `hasura` (the DDN project), `helm/aperio`, `image` (Bazel images) |
| `docs/` | the roadmap, the API reference, the voice design |

## Contributing

[CLAUDE.md](CLAUDE.md) holds the rules the code follows. The ones you'll hit
first:

- **Generated code is never edited.** Change the proto, or extend the plugin
  (protoc-gen-store, protoc-gen-cache, protoc-gen-service, protokit), and run
  `just gen`. Schema changes go through `just db migrate <name>`; never edit
  a migration or Hasura's `.hml` metadata by hand.
- **Hand-written code is business logic only**, in hooks and custom methods.
  A new RPC needs a rule in `internal/auth/policy_table.go`. Webhooks are
  mounted with `Services.Route`, and background jobs plug into
  `internal/server/background.go`.
- **No import aliases.** Refer to a package by the name it declares. If two
  imports clash (every protobuf-ota package has a `types`), split the file.
- **Every hand-written file stays under 200 lines** and is documented.
- **Test data uses John Doe** (Jane Doe, "Doe wedding") for every person,
  never a real-sounding name.
- **Protos lint clean:** `just lint`. Only
  `core::0215::foreign-type-reference` may be disabled.
- **Diagrams are Mermaid**, never ASCII art.
- A new Go package needs `bazel run //:gazelle`. A new task tool or guest
  flow gets a step in `test/load/scenarios` and a budget in the thresholds.

## More

- [docs/roadmap.md](docs/roadmap.md): architecture, status, milestones, open decisions
- [docs/api.md](docs/api.md): every RPC and its REST route, and who may call it
- [docs/voice.md](docs/voice.md): aperio on a phone call, the concierge and elicitation
- [CHANGELOG.md](CHANGELOG.md): what changed, release by release

Licensed under AGPL-3.0-or-later ([LICENSE](LICENSE)).
