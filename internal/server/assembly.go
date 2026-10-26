// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package server

// assembly.go builds aperio's services on one database connection and one
// freebusy client: each a generated AIP server, with hand-written business
// logic where it has some. They call each other through their gRPC methods,
// so each one's rules hold whoever asks: pricing prices what the catalog
// sells; sales prices quotes through pricing and books accepted ones through
// the stay and event services; finance confirms a paid booking through them;
// the concierge, the guest's side, works through guest, pricing, stay,
// sales, catalog and operations.

import (
	"github.com/shokki-engineering/aperio/internal/booking"
	"github.com/shokki-engineering/aperio/internal/database"
	"github.com/shokki-engineering/aperio/internal/database/repository/aperio/catalog"
	"github.com/shokki-engineering/aperio/internal/database/repository/aperio/document"
	"github.com/shokki-engineering/aperio/internal/database/repository/aperio/operations"
	"github.com/shokki-engineering/aperio/internal/database/repository/aperio/venue"
	"github.com/shokki-engineering/aperio/internal/database/repository/repox"
	"github.com/shokki-engineering/aperio/internal/engine"
	"github.com/shokki-engineering/aperio/internal/service/concierge"
	"github.com/shokki-engineering/aperio/internal/service/event"
	"github.com/shokki-engineering/aperio/internal/service/finance"
	"github.com/shokki-engineering/aperio/internal/service/generated/aperio/catalog/v1/catalogpbv1svc"
	"github.com/shokki-engineering/aperio/internal/service/generated/aperio/document/v1/documentpbv1svc"
	"github.com/shokki-engineering/aperio/internal/service/generated/aperio/guest/v1/guestpbv1svc"
	"github.com/shokki-engineering/aperio/internal/service/generated/aperio/operations/v1/operationspbv1svc"
	"github.com/shokki-engineering/aperio/internal/service/generated/aperio/venue/v1/venuepbv1svc"
	"github.com/shokki-engineering/aperio/internal/service/guest"
	"github.com/shokki-engineering/aperio/internal/service/pricing"
	"github.com/shokki-engineering/aperio/internal/service/sales"
	"github.com/shokki-engineering/aperio/internal/service/stay"
	"github.com/the-protobuf-project/runtime-go/grpc/hints"
	"google.golang.org/grpc"
)

// Services are aperio's assembled services.
type Services struct {
	Conn   *database.Connection
	Engine engine.Engine

	Venues     *venuepbv1svc.VenueServiceServer
	Catalog    *catalogpbv1svc.CatalogServiceServer
	Pricing    *pricing.Server
	Guests     *guestpbv1svc.GuestServiceServer
	Sales      *sales.Server
	Stays      *stay.Server
	Events     *event.Server
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
	venues, catalogRepos, guestRepos := venue.New(rc), catalogRepositories(rc), guestRepositories(rc)
	base := booking.NewBase(eng, venues, guestRepos.Contacts)
	stays, events := stay.New(stayRepositories(rc), base), event.New(eventRepositories(rc), base)
	prices := pricing.New(pricingRepositories(rc), catalogRepos, venues.Venues)
	guests := guest.New(guestRepos)
	catalogSvc := catalogpbv1svc.NewCatalogServiceServer(catalogRepos, catalogpbv1svc.NopCatalogServiceHooks{})
	selling := sales.New(salesRepositories(rc), venues.Venues, catalogRepos.Menus, prices, events, stays)
	ops := operationspbv1svc.NewOperationsServiceServer(
		operationspbv1svc.OperationsServiceRepositories{ServiceRequests: operations.New(rc).ServiceRequests}, operationspbv1svc.NopOperationsServiceHooks{})
	return &Services{
		Conn:   conn,
		Engine: eng,
		Venues: venuepbv1svc.NewVenueServiceServer(
			venuepbv1svc.VenueServiceRepositories{Venues: venues.Venues, Spaces: venues.Spaces}, venuepbv1svc.NopVenueServiceHooks{}),
		Catalog: catalogSvc,
		Pricing: prices,
		Guests:  guests,
		Sales:   selling,
		Stays:   stays,
		Events:  events,
		Finance: finance.New(financeRepositories(rc), venues.Venues, bookings{stays: stays, events: events}, gateways),
		Documents: documentpbv1svc.NewDocumentServiceServer(
			documentpbv1svc.DocumentServiceRepositories{Documents: document.New(rc).Documents}, documentpbv1svc.NopDocumentServiceHooks{}),
		Operations: ops,
		Concierge: concierge.New(venues, eng, concierge.Services{
			Pricing: prices, Guests: guests, Stays: stays, Sales: selling, Catalog: catalogSvc, Operations: ops,
		}, holdSecret),
	}
}

// catalogRepositories are the CatalogService's repositories over conn.
func catalogRepositories(conn repox.Conn) catalogpbv1svc.CatalogServiceRepositories {
	r := catalog.New(conn)
	return catalogpbv1svc.CatalogServiceRepositories{Dishes: r.Dishes, Menus: r.Menus, FixedPackages: r.FixedPackages}
}

// Interceptors are the unary interceptors every surface shares, after
// validation: hints that turn an error into the next step to try.
func (s *Services) Interceptors() []grpc.UnaryServerInterceptor {
	return []grpc.UnaryServerInterceptor{
		hints.New(hints.Options{Product: "aperio"}).Interceptor(),
	}
}
