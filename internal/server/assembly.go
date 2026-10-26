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
	"context"
	"log/slog"

	"github.com/shokki-engineering/aperio/internal/booking"
	"github.com/shokki-engineering/aperio/internal/database"
	"github.com/shokki-engineering/aperio/internal/database/repository/aperio/catalog"
	"github.com/shokki-engineering/aperio/internal/database/repository/aperio/document"
	"github.com/shokki-engineering/aperio/internal/database/repository/aperio/venue"
	"github.com/shokki-engineering/aperio/internal/database/repository/repox"
	"github.com/shokki-engineering/aperio/internal/engine"
	"github.com/shokki-engineering/aperio/internal/service/concierge"
	"github.com/shokki-engineering/aperio/internal/service/event"
	"github.com/shokki-engineering/aperio/internal/service/finance"
	"github.com/shokki-engineering/aperio/internal/service/generated/aperio/catalog/v1/catalogpbv1svc"
	"github.com/shokki-engineering/aperio/internal/service/generated/aperio/document/v1/documentpbv1svc"
	"github.com/shokki-engineering/aperio/internal/service/generated/aperio/venue/v1/venuepbv1svc"
	"github.com/shokki-engineering/aperio/internal/service/guest"
	"github.com/shokki-engineering/aperio/internal/service/operations"
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
	Guests     *guest.Server
	Sales      *sales.Server
	Stays      *stay.Server
	Events     *event.Server
	Finance    *finance.Server
	Documents  *documentpbv1svc.DocumentServiceServer
	Operations *operations.Server
	Concierge  *concierge.Server

	// copies are the read models' OpenSearch copies, by read model; none
	// unless [search] provider = "opensearch" (search.go).
	copies map[string]searchIndex
}

// New assembles the services on conn, booking through eng, sealing the
// concierge's hold tokens with holdSecret. gateways are the payment
// gateways' adapters (none yet: gateway payments are recorded, and refunded
// in the gateway's own dashboard).
func New(conn *database.Connection, eng engine.Engine, holdSecret []byte, gateways ...finance.Gateway) *Services {
	// The read models' writers must be on the connection before anything
	// builds a repository over it: each takes the projections it carries.
	if conn.Projections == nil {
		conn.Projections = repox.NewProjections(stay.NightsProjector())
		conn.Projections.OnError = func(ctx context.Context, projection string, err error) {
			// A Hasura write landed but its projection did not (it is sent
			// in a request of its own): a summary misses what it changed
			// until the next write to it, or a rebuild.
			slog.WarnContext(ctx, "read model not updated after a write", "projection", projection, "err", err)
		}
	}
	rc := conn.Repox()
	venues, catalogRepos, guestRepos := venue.New(rc), catalogRepositories(rc), guestRepositories(rc)
	base := booking.NewBase(eng, venues, guestRepos.Contacts)
	base.Search = conn.SearchConn()
	stays, events := stay.New(stayRepositories(rc), base), event.New(eventRepositories(rc), base)
	events.ReadMenusFrom(catalogRepos.Menus)
	prices := pricing.New(pricingRepositories(rc), catalogRepos, venues.Venues)
	guests := guest.New(guestRepos)
	catalogSvc := catalogpbv1svc.NewCatalogServiceServer(catalogRepos, catalogpbv1svc.NopCatalogServiceHooks{})
	selling := sales.New(salesRepositories(rc), venues.Venues, catalogRepos.Menus, prices, events, stays)
	selling.ReadContactsFrom(guestRepos.Contacts)
	ops := operations.New(operationsRepositories(rc), venues.Venues)
	money := finance.New(financeRepositories(rc), venues.Venues, bookings{stays: stays, events: events}, gateways)
	money.ReadContactsFrom(guestRepos.Contacts)
	guests.ReadHistoryFrom(guest.History{Stays: stays, Events: events, Sales: selling, Finance: money, Spaces: venues.Spaces})
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
		Finance: money,
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
