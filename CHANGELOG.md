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
  reference (ConfirmStay's), never by a phone alone.

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
