<!-- Copyright 2026 Srikanth K. -->
<!-- SPDX-License-Identifier: AGPL-3.0-or-later -->

# Changelog

Notable changes to aperio, newest first. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and versions follow
[Semantic Versioning](https://semver.org/): the API under `protobuf/` and the
`APERIO_*` configuration are what a version promises.

To release: move **Unreleased** under a new `## [x.y.z] - YYYY-MM-DD` heading
and set `[app] version` in `internal/config/aperio.release.toml` to `vx.y.z`.

## [Unreleased]

### Added

- Payments (M2): Razorpay payment links (`FinanceService.RequestPayment`),
  refunds and its signed webhook at `/webhooks/razorpay` (`[razorpay]`);
  `ClearPayment` for cheques; a receipt voucher for every advance.
- Staying in step with freebusy (M4): freebusy's webhooks at
  `/webhooks/freebusy` settle stays and events cancelled or moved there;
  the subscription is ensured at startup; a reconciler repairs or flags
  drift; leads hold their dates; an accepted event holds its hall.
  `SendQuote` takes a `ttl`; sent quotes past it expire (`[freebusy_sync]`).
- Auth (M5): OIDC staff sign-in from any issuer, six roles with a rule for
  every RPC, per-venue access and guests' signed links, on gRPC, REST and
  MCP (`[auth]`; off, with a warning, until an issuer is set).
- Operations (M6): service-request states; department and guest
  notifications through a log, a signed webhook or WhatsApp Business
  (`[notify]`); a completed request's charge posted to its booking's bill.
- Channels (M7): Booking.com OTA_HotelResNotif in at
  `/channels/bookingcom/reservations` (`[channels.bookingcom]`), rates and
  availability messages out.
- Platform (M3): Redis list caches (`[cache]`), query tracing, the pool
  monitor, background jobs, and webhooks served beside the REST gateway.
- A k6 load test (`test/load`, `just load smoke|day|stress|soak`): staff
  reads, guest bookings through real freebusy holds, in-house requests and
  MCP agents at once, with a busy day's latency budgets as thresholds.
- Deployment: the images built by Bazel with rules_oci (`just image`,
  `deploy/image`): the server on scratch and the Prisma migrations on
  distroless Node, each for linux/amd64 and linux/arm64; Compose for the
  whole stack (`just stack`), a Helm chart (`deploy/helm/aperio`, `just
  helm`), CI with a generated-code drift check and image and chart jobs, a
  release workflow, and Dependabot.

- The server: `main.go` on runtime-go's HybridServer, every service on gRPC
  (:50061) and REST (:8090); configuration (`internal/config`) and telemetry
  (`internal/telemetry`) as freebusy's.
- Bookings: stays and sessions booked in freebusy with compensation, state
  methods, calendar notes on the booked rooms' calendars.
- Pricing and quotes: seasons, price rules, `CalculatePrice`, quotes through
  Send, Decline, Revise and Accept (which books the event and stay).
- GST invoices: CGST/SGST/IGST, numbering per venue, kind and financial year
  (`Venue.invoice_prefix`), Issue, Cancel, Credit.
- Payments: any currency, invoice paid and due, an advance confirming its
  booking, gateway settle and refund, void.
- MCP: every service's tools at :8091/mcp, now that protoc-gen-mcp shares
  types through `$defs`.

- The concierge (`aperio.concierge.v1`): FindStay, HoldStay and ConfirmStay,
  the guest's side for the website and voice agents; a hold is a freebusy
  hold the booking then takes, sealed in a token. Contacts carry
  `phone_numbers`, by which a returning guest is found. FindBooking and
  CancelBooking find and cancel a guest's stay by their phone and its
  reference (ConfirmStay's), never by a phone alone. EnquireEvent takes an
  event enquiry as a lead and a draft quote of the cheapest menu that suits,
  with the menus priced and the halls free that day; RequestService sends a
  guest's ask -- two black coffees to room 204 -- to the department that
  does it.
- Read models, as freebusy's: `stay.v1.StayNight`, one row per night of
  every live stay, written in the same transaction as the reservation, with
  an optional OpenSearch copy (`[search] provider = "opensearch"`) kept by a
  change-feed indexer and read with Postgres behind it; rebuilt and
  reindexed at startup when empty (`rebuild_on_start`, `reindex_on_start`).
- `StayService.SummarizeDay`: a venue's day -- arrivals, departures, who
  stays the night, rooms taken -- for the front desk and agents, from the
  read model in four indexed reads. `Reservation.spaces`, the room types
  booked, so stays filter as `spaces:"venues/sbr/spaces/villa"`.
- `GuestService.SummarizeContact`: a guest's history in one call -- stays
  and nights by room type, the last and next visit, events, open leads and
  quotes, paid less refunds and still due, what they asked for before --
  counted from `start_date` when given ("this year").
- Task tools for the other services' staff and agents, each a few indexed
  reads: `EventService.SummarizeSessions` (the coming days' sessions with
  covers and plates by diet), `SalesService.SummarizePipeline` (leads by
  stage, won and lost lately, follow-ups due, uncontacted leads, quotes
  expiring), `FinanceService.SummarizeReceivables` (outstanding, overdue,
  aging, debtors, collected this month; credit notes left out) and
  `OperationsService.SummarizeServiceRequests` (open requests by
  department, overdue first). Operations gets a service package of its own.
- Elicitation from the concierge: a method missing the guest's name or phone
  asks for just those, and ConfirmStay and CancelBooking ask for consent with
  the real amount and dates (`confirmed`) before they book or cancel. The
  question is a gRPC error (`google.rpc.ErrorInfo`, reason
  `ELICITATION_REQUIRED`) that a REST or gRPC client shows as a form; over MCP
  it is an elicitation to a client that declares the capability, and a tool
  error the model asks in its own words to one that does not.

### Fixed

- An issued credit note now settles and reduces what its invoice is due;
  proformas are no longer counted beside the tax invoice that supersedes
  them; receipt vouchers can't be paid against.
- A second acceptance of the same hall on the same day fails instead of
  double-booking it.

### Changed

- **Breaking:** the API is one service per part of the business: venue,
  catalog, pricing, guest, sales, stay, event, finance, operations,
  document and concierge (97 methods, was 111). Resource names and URLs are
  unchanged; resource types move (`aperio.booking.v1/Reservation` is
  `aperio.stay.v1/Reservation`). Bookings, quotes and leads are cancelled or
  lost, never deleted. The database migrations are squashed into a fresh
  init.

- The layout follows freebusy's: service packages named after their domain
  (`internal/service/stay`, `event`, `sales`, ...), shared live-test setup in
  `test/live`, the calendar-links command in `tools/calendar`.
