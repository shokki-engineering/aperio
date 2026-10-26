// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package server

// assembly.go builds aperio's services on one database connection and one
// freebusy client: the generated AIP servers, with the hand-written business
// logic where there is some (booking, sales, finance), and plain generated
// servers where the standard methods are all there is yet (venue, crm,
// document, operations). The sales service books an accepted quote through
// the booking service, and finance confirms a paid booking through it; the
// concierge -- the guest's side -- works through all of them.

import (
	"github.com/shokki-engineering/aperio/internal/database"
	"github.com/shokki-engineering/aperio/internal/database/repository/aperio/document"
	"github.com/shokki-engineering/aperio/internal/database/repository/aperio/operations"
	"github.com/shokki-engineering/aperio/internal/database/repository/aperio/venue"
	"github.com/shokki-engineering/aperio/internal/engine"
	"github.com/shokki-engineering/aperio/internal/service/booking"
	"github.com/shokki-engineering/aperio/internal/service/concierge"
	"github.com/shokki-engineering/aperio/internal/service/crm"
	"github.com/shokki-engineering/aperio/internal/service/finance"
	"github.com/shokki-engineering/aperio/internal/service/generated/aperio/crm/v1/crmpbv1svc"
	"github.com/shokki-engineering/aperio/internal/service/generated/aperio/document/v1/documentpbv1svc"
	"github.com/shokki-engineering/aperio/internal/service/generated/aperio/operations/v1/operationspbv1svc"
	"github.com/shokki-engineering/aperio/internal/service/generated/aperio/venue/v1/venuepbv1svc"
	"github.com/shokki-engineering/aperio/internal/service/sales"
	"github.com/the-protobuf-project/runtime-go/grpc/hints"
	"google.golang.org/grpc"
)

// Services are aperio's assembled services.
type Services struct {
	Conn   *database.Connection
	Engine engine.Engine

	Venues     *venuepbv1svc.VenueServiceServer
	CRM        *crmpbv1svc.CrmServiceServer
	Bookings   *booking.Server
	Sales      *sales.Server
	Finance    *finance.Server
	Documents  *documentpbv1svc.DocumentServiceServer
	Operations *operationspbv1svc.OperationsServiceServer
	Concierge  *concierge.Server
}

// New assembles the services on conn, booking through eng, sealing the
// concierge's hold tokens with holdSecret. gateways are the payment
// gateways' adapters (none yet: gateway payments are recorded, and refunded
// in the gateway's own dashboard).
func New(conn *database.Connection, eng engine.Engine, holdSecret []byte, gateways ...finance.Gateway) *Services {
	rc := conn.Repox()
	venues, people := venue.New(rc), crmRepositories(rc)
	bookings := booking.New(bookingRepositories(rc), venues, people.Contacts, eng)
	selling := sales.New(salesRepositories(rc), venues.Venues, people.Leads, bookings)
	guests := crm.New(people)
	return &Services{
		Conn:   conn,
		Engine: eng,
		Venues: venuepbv1svc.NewVenueServiceServer(
			venuepbv1svc.VenueServiceRepositories{Venues: venues.Venues, Spaces: venues.Spaces}, venuepbv1svc.NopVenueServiceHooks{}),
		CRM:      guests,
		Bookings: bookings,
		Sales:    selling,
		Finance:  finance.New(financeRepositories(rc), venues.Venues, bookings, gateways),
		Documents: documentpbv1svc.NewDocumentServiceServer(
			documentpbv1svc.DocumentServiceRepositories{Documents: document.New(rc).Documents}, documentpbv1svc.NopDocumentServiceHooks{}),
		Operations: operationspbv1svc.NewOperationsServiceServer(
			operationspbv1svc.OperationsServiceRepositories{ServiceRequests: operations.New(rc).ServiceRequests}, operationspbv1svc.NopOperationsServiceHooks{}),
		Concierge: concierge.New(venues, eng, selling, guests, bookings, holdSecret),
	}
}

// Interceptors are the unary interceptors every surface shares, after
// validation: hints that turn an error into the next step to try.
func (s *Services) Interceptors() []grpc.UnaryServerInterceptor {
	return []grpc.UnaryServerInterceptor{
		hints.New(hints.Options{Product: "aperio"}).Interceptor(),
	}
}
