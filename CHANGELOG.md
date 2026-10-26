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

### Changed

- The layout follows freebusy's: service packages named after their domain
  (`internal/service/booking`, `sales`, `finance`), shared live-test setup in
  `test/live`, the calendar-links command in `tools/calendar`.
