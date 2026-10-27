<!-- Copyright 2026 Srikanth K. -->
<!-- SPDX-License-Identifier: AGPL-3.0-or-later -->

# aperio API reference

Every method of aperio's eleven services, with its REST route. These tables
are taken from the protos (`protobuf/aperio/*/v1/*_service.proto`, their
`google.api.http` options). When a proto changes, the proto is right and this
page is out of date. How to call the API over gRPC, REST and MCP is in the
[README](../README.md#the-api).

**109 methods**, each served three ways:

- **gRPC** on `:50061`, as `aperio.<service>.v1.<Service>/<Method>`, for
  example `aperio.stay.v1.StayService/SummarizeDay`.
- **REST** on `:8090`, at the route in the tables below.
- **MCP** at `:8091/mcp`, as the tool `<service>_service-<method>_v1`, for
  example `stay_service-summarize_day_v1`.

Every `Update*` method is a `PATCH` that writes only the fields named in
`update_mask` (AIP-134). Every `List*` method takes `pageSize`, `pageToken`
and `filter`. Bookings, quotes, invoices, payments and service requests are
never deleted. They move through their state methods instead.

## Contents

- [VenueService](#venueservice-aperiovenuev1): venues and their spaces
- [CatalogService](#catalogservice-aperiocatalogv1): dishes, menus, fixed packages
- [PricingService](#pricingservice-aperiopricingv1): seasons, price rules, CalculatePrice
- [GuestService](#guestservice-aperioguestv1): contacts
- [SalesService](#salesservice-aperiosalesv1): leads, activities, quotes
- [StayService](#stayservice-aperiostayv1): room reservations
- [EventService](#eventservice-aperioeventv1): events and their sessions
- [FinanceService](#financeservice-aperiofinancev1): GST invoices and payments
- [OperationsService](#operationsservice-aperiooperationsv1): service requests
- [DocumentService](#documentservice-aperiodocumentv1): building documents
- [ConciergeService](#conciergeservice-aperioconciergev1): the guest's tasks
- [Task tools](#task-tools): the Summarize methods

## VenueService (`aperio.venue.v1`)

| Method | What it does | REST |
|---|---|---|
| GetVenue | Retrieves one venue. | `GET /v1/{name=venues/*}` |
| ListVenues | Lists venues. | `GET /v1/venues` |
| CreateVenue | Creates a venue. | `POST /v1/venues` |
| UpdateVenue | Updates a venue. | `PATCH /v1/{venue.name=venues/*}` |
| GetSpace | Retrieves one space (a room type or a hall). | `GET /v1/{name=venues/*/spaces/*}` |
| ListSpaces | Lists spaces. | `GET /v1/{parent=venues/*}/spaces` |
| CreateSpace | Creates a space. | `POST /v1/{parent=venues/*}/spaces` |
| UpdateSpace | Updates a space. | `PATCH /v1/{space.name=venues/*/spaces/*}` |

## CatalogService (`aperio.catalog.v1`)

| Method | What it does | REST |
|---|---|---|
| GetDish | Retrieves one dish. | `GET /v1/{name=venues/*/dishes/*}` |
| ListDishes | Lists dishes. | `GET /v1/{parent=venues/*}/dishes` |
| CreateDish | Creates a dish. | `POST /v1/{parent=venues/*}/dishes` |
| UpdateDish | Updates a dish. | `PATCH /v1/{dish.name=venues/*/dishes/*}` |
| GetMenu | Retrieves one menu. | `GET /v1/{name=venues/*/menus/*}` |
| ListMenus | Lists menus. | `GET /v1/{parent=venues/*}/menus` |
| CreateMenu | Creates a menu. | `POST /v1/{parent=venues/*}/menus` |
| UpdateMenu | Updates a menu. | `PATCH /v1/{menu.name=venues/*/menus/*}` |
| GetFixedPackage | Retrieves one fixed package. | `GET /v1/{name=venues/*/fixedPackages/*}` |
| ListFixedPackages | Lists fixed packages. | `GET /v1/{parent=venues/*}/fixedPackages` |
| CreateFixedPackage | Creates a fixed package. | `POST /v1/{parent=venues/*}/fixedPackages` |
| UpdateFixedPackage | Updates a fixed package. | `PATCH /v1/{fixed_package.name=venues/*/fixedPackages/*}` |

## PricingService (`aperio.pricing.v1`)

| Method | What it does | REST |
|---|---|---|
| GetSeason | Retrieves one season. | `GET /v1/{name=venues/*/seasons/*}` |
| ListSeasons | Lists seasons. | `GET /v1/{parent=venues/*}/seasons` |
| CreateSeason | Creates a season. | `POST /v1/{parent=venues/*}/seasons` |
| UpdateSeason | Updates a season. | `PATCH /v1/{season.name=venues/*/seasons/*}` |
| DeleteSeason | Deletes a season. | `DELETE /v1/{name=venues/*/seasons/*}` |
| GetPriceRule | Retrieves one price rule. | `GET /v1/{name=venues/*/priceRules/*}` |
| ListPriceRules | Lists price rules. | `GET /v1/{parent=venues/*}/priceRules` |
| CreatePriceRule | Creates a price rule. | `POST /v1/{parent=venues/*}/priceRules` |
| UpdatePriceRule | Updates a price rule. | `PATCH /v1/{price_rule.name=venues/*/priceRules/*}` |
| DeletePriceRule | Deletes a price rule. | `DELETE /v1/{name=venues/*/priceRules/*}` |
| CalculatePrice | Prices a menu, fixed package, space or dish for a date: its base price with the highest-priority matching rule applied. Quotes, the website and agents all price through it. | `POST /v1/{venue=venues/*}:calculatePrice` |

## GuestService (`aperio.guest.v1`)

| Method | What it does | REST |
|---|---|---|
| GetContact | Retrieves one contact. | `GET /v1/{name=contacts/*}` |
| ListContacts | Lists contacts. | `GET /v1/contacts` |
| CreateContact | Creates a contact. | `POST /v1/contacts` |
| UpdateContact | Updates a contact. | `PATCH /v1/{contact.name=contacts/*}` |
| SummarizeContact | A guest's history: stays and nights by room type, the last and next visit, events, open leads and quotes, what they paid and still owe, past requests. | `GET /v1/{name=contacts/*}:summarize` |

## SalesService (`aperio.sales.v1`)

| Method | What it does | REST |
|---|---|---|
| GetLead | Retrieves one lead. | `GET /v1/{name=leads/*}` |
| ListLeads | Lists leads. | `GET /v1/leads` |
| CreateLead | Creates a lead. | `POST /v1/leads` |
| UpdateLead | Updates a lead. | `PATCH /v1/{lead.name=leads/*}` |
| GetActivity | Retrieves one activity (a call, a visit, a note). | `GET /v1/{name=leads/*/activities/*}` |
| ListActivities | Lists a lead's activities. | `GET /v1/{parent=leads/*}/activities` |
| CreateActivity | Creates an activity. | `POST /v1/{parent=leads/*}/activities` |
| GetQuote | Retrieves one quote. | `GET /v1/{name=venues/*/quotes/*}` |
| ListQuotes | Lists quotes. | `GET /v1/{parent=venues/*}/quotes` |
| CreateQuote | Creates a draft quote. | `POST /v1/{parent=venues/*}/quotes` |
| UpdateQuote | Updates a draft quote. | `PATCH /v1/{quote.name=venues/*/quotes/*}` |
| SendQuote | Sends a draft quote to the guest and fixes its prices. Takes a `ttl` (14 days when unset). | `POST /v1/{name=venues/*/quotes/*}:send` |
| AcceptQuote | Accepts a sent quote: makes the reservation or event it offers, and wins its lead. | `POST /v1/{name=venues/*/quotes/*}:accept` |
| DeclineQuote | Records that the guest declined a sent quote. | `POST /v1/{name=venues/*/quotes/*}:decline` |
| ReviseQuote | Makes a new draft from a sent or declined quote, which supersedes it. | `POST /v1/{name=venues/*/quotes/*}:revise` |
| SummarizePipeline | A venue's sales pipeline: open leads by stage, recent wins and losses, follow-ups due, uncontacted leads, quotes about to expire. | `GET /v1/{venue=venues/*}:summarizePipeline` |

## StayService (`aperio.stay.v1`)

| Method | What it does | REST |
|---|---|---|
| GetReservation | Retrieves one reservation. | `GET /v1/{name=venues/*/reservations/*}` |
| ListReservations | Lists reservations. | `GET /v1/{parent=venues/*}/reservations` |
| CreateReservation | Creates a reservation. Its rooms are held, then reserved, in freebusy. | `POST /v1/{parent=venues/*}/reservations` |
| UpdateReservation | Updates a reservation. | `PATCH /v1/{reservation.name=venues/*/reservations/*}` |
| ConfirmReservation | Confirms a pending reservation. Its rooms stay taken. | `POST /v1/{name=venues/*/reservations/*}:confirm` |
| CancelReservation | Cancels a reservation and releases its rooms. | `POST /v1/{name=venues/*/reservations/*}:cancel` |
| ArriveReservation | Records the guests' arrival. | `POST /v1/{name=venues/*/reservations/*}:arrive` |
| DepartReservation | Records the guests' departure. | `POST /v1/{name=venues/*/reservations/*}:depart` |
| SummarizeDay | A day at a venue: arrivals, departures, who stays the night, rooms taken. | `GET /v1/{venue=venues/*}/reservations:summarizeDay` |

## EventService (`aperio.event.v1`)

| Method | What it does | REST |
|---|---|---|
| GetEvent | Retrieves one event. | `GET /v1/{name=venues/*/events/*}` |
| ListEvents | Lists events. | `GET /v1/{parent=venues/*}/events` |
| CreateEvent | Creates an event. | `POST /v1/{parent=venues/*}/events` |
| UpdateEvent | Updates an event. | `PATCH /v1/{event.name=venues/*/events/*}` |
| ConfirmEvent | Confirms a tentative event. Its spaces stay taken. | `POST /v1/{name=venues/*/events/*}:confirm` |
| CancelEvent | Cancels an event and releases the spaces of its sessions. | `POST /v1/{name=venues/*/events/*}:cancel` |
| GetSession | Retrieves one session. | `GET /v1/{name=venues/*/events/*/sessions/*}` |
| ListSessions | Lists an event's sessions. | `GET /v1/{parent=venues/*/events/*}/sessions` |
| CreateSession | Creates a session, which books its space in freebusy. | `POST /v1/{parent=venues/*/events/*}/sessions` |
| UpdateSession | Updates a session. | `PATCH /v1/{session.name=venues/*/events/*/sessions/*}` |
| DeleteSession | Deletes a session. | `DELETE /v1/{name=venues/*/events/*/sessions/*}` |
| SummarizeSessions | The next few days' sessions for the kitchen and banquet team: space, times, covers, menu, plates by diet. | `GET /v1/{venue=venues/*}/events:summarizeSessions` |

## FinanceService (`aperio.finance.v1`)

| Method | What it does | REST |
|---|---|---|
| GetInvoice | Retrieves one invoice. | `GET /v1/{name=venues/*/invoices/*}` |
| ListInvoices | Lists invoices. | `GET /v1/{parent=venues/*}/invoices` |
| CreateInvoice | Creates a draft invoice, priced with CGST/SGST or IGST. | `POST /v1/{parent=venues/*}/invoices` |
| UpdateInvoice | Updates a draft invoice. | `PATCH /v1/{invoice.name=venues/*/invoices/*}` |
| IssueInvoice | Issues a draft: gives it the next number in the venue's series, dates it and fixes it. | `POST /v1/{name=venues/*/invoices/*}:issue` |
| CancelInvoice | Cancels an issued invoice before it is reported. Its number stays used. | `POST /v1/{name=venues/*/invoices/*}:cancel` |
| CreditInvoice | Makes a draft credit note that corrects an issued tax invoice. | `POST /v1/{name=venues/*/invoices/*}:credit` |
| GetPayment | Retrieves one payment. | `GET /v1/{name=venues/*/payments/*}` |
| ListPayments | Lists payments. | `GET /v1/{parent=venues/*}/payments` |
| CreatePayment | Records a payment, in any currency. | `POST /v1/{parent=venues/*}/payments` |
| UpdatePayment | Updates a payment. | `PATCH /v1/{payment.name=venues/*/payments/*}` |
| RefundPayment | Refunds a succeeded payment, as a payment of its own (through the gateway when it came through one). | `POST /v1/{name=venues/*/payments/*}:refund` |
| VoidPayment | Voids a payment recorded in error. | `POST /v1/{name=venues/*/payments/*}:void` |
| ClearPayment | Clears a pending cheque or bank transfer once the bank has credited it. | `POST /v1/{name=venues/*/payments/*}:clear` |
| RequestPayment | Asks the guest to pay an issued invoice online: a Razorpay payment link, settled by Razorpay's webhook. | `POST /v1/{name=venues/*/invoices/*}:requestPayment` |
| SummarizeReceivables | What a venue is owed: outstanding, overdue, aging, top debtors, collected this month. | `GET /v1/{venue=venues/*}/invoices:summarizeReceivables` |

## OperationsService (`aperio.operations.v1`)

| Method | What it does | REST |
|---|---|---|
| GetServiceRequest | Retrieves one service request. | `GET /v1/{name=venues/*/serviceRequests/*}` |
| ListServiceRequests | Lists service requests. | `GET /v1/{parent=venues/*}/serviceRequests` |
| CreateServiceRequest | Creates a service request; its department is notified. | `POST /v1/{parent=venues/*}/serviceRequests` |
| AcknowledgeServiceRequest | Takes up an open request for its department. | `POST /v1/{name=venues/*/serviceRequests/*}:acknowledge` |
| StartServiceRequest | Marks an acknowledged request as on its way. | `POST /v1/{name=venues/*/serviceRequests/*}:start` |
| CompleteServiceRequest | Marks a request done. The requester is told, and any charge is posted to the booking's bill. | `POST /v1/{name=venues/*/serviceRequests/*}:complete` |
| CancelServiceRequest | Calls off a request. | `POST /v1/{name=venues/*/serviceRequests/*}:cancel` |
| SummarizeServiceRequests | Open requests by department: what is waiting, what is overdue (oldest first), what was done today. | `GET /v1/{venue=venues/*}/serviceRequests:summarize` |

## DocumentService (`aperio.document.v1`)

Metadata only for now (category, uri, sha256, expiry). Uploads and expiry
reminders are still to be designed ([roadmap](roadmap.md), M6).

| Method | What it does | REST |
|---|---|---|
| GetDocument | Retrieves one document. | `GET /v1/{name=venues/*/documents/*}` |
| ListDocuments | Lists documents. | `GET /v1/{parent=venues/*}/documents` |
| CreateDocument | Creates a document. | `POST /v1/{parent=venues/*}/documents` |
| UpdateDocument | Updates a document. | `PATCH /v1/{document.name=venues/*/documents/*}` |
| DeleteDocument | Deletes a document. | `DELETE /v1/{name=venues/*/documents/*}` |

## ConciergeService (`aperio.concierge.v1`)

The guest's side, for the website and for voice agents. The inputs are flat
and few, and every reply carries a `spoken` sentence an agent can say as it
is. The concierge is public: it needs no staff token. The design is in
[voice.md](voice.md).

| Method | What it does | Asks (elicitation) | REST |
|---|---|---|---|
| FindStay | Room types free for the dates and guests, priced, cheapest first (at most three). | nothing | `POST /v1/{venue=venues/*}:findStay` |
| HoldStay | Holds rooms of one type in freebusy for a few minutes and prices them. Returns a `hold_token`. | the guest's name and phone, if missing | `POST /v1/{venue=venues/*}:holdStay` |
| ConfirmStay | Books the held stay. The booking stays PENDING until its advance is paid. | consent, with the room, dates and amount | `POST /v1/{venue=venues/*}:confirmStay` |
| FindBooking | A guest's stays, found by their phone plus the booking's reference or arrival date. | nothing | `POST /v1/{venue=venues/*}:findBooking` |
| CancelBooking | Cancels a guest's stay, found by phone and reference, and gives its rooms back. | consent, with the dates and reference | `POST /v1/{venue=venues/*}:cancelBooking` |
| EnquireEvent | Takes an event enquiry: a lead for sales, a draft quote with the cheapest menu that fits, the menus that suit, the halls free that day. | the guest's name and phone, if missing | `POST /v1/{venue=venues/*}:enquireEvent` |
| RequestService | Sends a guest's request to the department that handles it ("two black coffees to room 204"). | nothing | `POST /v1/{venue=venues/*}:requestService` |

## Task tools

The `Summarize*` methods are built for staff and their agents. Each one
answers a whole question in one call, from a few indexed reads (the stay
summary reads the `stay.v1.StayNight` read model), so an agent doesn't need to
page through every record.

| Tool | Service | For |
|---|---|---|
| `stay_service-summarize_day_v1` | StayService.SummarizeDay | the front desk |
| `event_service-summarize_sessions_v1` | EventService.SummarizeSessions | the kitchen and banquet team |
| `guest_service-summarize_contact_v1` | GuestService.SummarizeContact | anyone serving a returning guest |
| `sales_service-summarize_pipeline_v1` | SalesService.SummarizePipeline | sales |
| `finance_service-summarize_receivables_v1` | FinanceService.SummarizeReceivables | accounts |
| `operations_service-summarize_service_requests_v1` | OperationsService.SummarizeServiceRequests | the departments |

## Who may call what

When `[auth] issuer` is set, each method is open only to the roles listed in
`internal/auth/policy_table.go`. The owner may call every method. The
ConciergeService is public. `GetQuote`, `GetReservation` and `GetInvoice` also
accept a guest's signed link (`Authorization: Link <token>`) for the one
resource the link names. In short:

| Role | Reaches |
|---|---|
| `owner` | everything; alone changes venues, spaces, the catalog, pricing, and deletes documents |
| `front_desk` | runs stays (create, confirm, cancel, arrive, depart); contacts and leads; reads quotes, events, invoices and prices; records payments and sends payment links; works service requests |
| `sales` | leads, quotes (create, send, accept, decline, revise), events and sessions, the pipeline; reads stays, contacts and prices |
| `kitchen` | reads events, sessions and menus; works service requests; the day summary. No invoices. |
| `housekeeping` | works service requests; the day summary |
| `accounts` | invoices, payments, refunds, documents, receivables, the pipeline; reads stays, events, quotes, contacts and prices |
