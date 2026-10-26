// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package sales serves aperio.sales.v1 -- leads, their activity, and quotes
// -- on the generated AIP server: the business logic of quoting.
//
//   - A quote (quotes.go) is priced line by line through the pricing service
//     when it is made, and again on every edit while it is a draft, so a
//     draft follows the rules as they change; its totals are the server's.
//   - SendQuote fixes the prices and starts the offer's clock; DeclineQuote
//     and ReviseQuote move it on (states.go); AcceptQuote makes its event and
//     stay through the event and stay services and wins its lead (accept.go).
package sales

import (
	"context"
	"time"

	"github.com/shokki-engineering/aperio/internal/database/repository/aperio/catalog"
	"github.com/shokki-engineering/aperio/internal/database/repository/aperio/guest"
	"github.com/shokki-engineering/aperio/internal/database/repository/aperio/venue"
	"github.com/shokki-engineering/aperio/internal/service/generated/aperio/sales/v1/salespbv1svc"
	"github.com/shokki-engineering/aperio/internal/service/generated/servicex"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/event/v1/eventpbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/pricing/v1/pricingpbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/stay/v1/staypbv1"
)

// offerTTL is how long a sent quote stands. Quote.ttl is INPUT_ONLY and not
// stored, so it cannot reach SendQuote; until SendQuoteRequest carries one,
// every offer stands this long.
const offerTTL = 14 * 24 * time.Hour

// Pricer prices an item for a date: the pricing service.
type Pricer interface {
	CalculatePrice(context.Context, *pricingpbv1.CalculatePriceRequest) (*pricingpbv1.CalculatePriceResponse, error)
}

// Events makes, finds and cancels the event an accepted quote becomes: the
// event service.
type Events interface {
	CreateEvent(context.Context, *eventpbv1.CreateEventRequest) (*eventpbv1.Event, error)
	GetEvent(context.Context, *eventpbv1.GetEventRequest) (*eventpbv1.Event, error)
	CancelEvent(context.Context, *eventpbv1.CancelEventRequest) (*eventpbv1.Event, error)
}

// Stays makes and finds the stay an accepted quote becomes: the stay
// service, whose hooks take the rooms from freebusy.
type Stays interface {
	CreateReservation(context.Context, *staypbv1.CreateReservationRequest) (*staypbv1.Reservation, error)
	GetReservation(context.Context, *staypbv1.GetReservationRequest) (*staypbv1.Reservation, error)
}

// Server serves aperio.sales.v1.SalesService: the generated standard methods
// with the pricing hooks, and the quote state methods.
type Server struct {
	*salespbv1svc.SalesServiceServer
	h      *hooks
	events Events // nil, with stays: AcceptQuote is Unimplemented
	stays  Stays
}

// New returns the Server over repos, reading each venue's currency from
// venues and a menu's fewest plates from menus, pricing through pricer, and
// making the bookings an accepted quote becomes through events and stays.
func New(repos salespbv1svc.SalesServiceRepositories, venues venue.VenueRepository, menus catalog.MenuRepository,
	pricer Pricer, events Events, stays Stays, opts ...servicex.Option) *Server {
	h := &hooks{repos: repos, venues: venues, menus: menus, pricer: pricer, now: time.Now}
	return &Server{SalesServiceServer: salespbv1svc.NewSalesServiceServer(repos, h, opts...), h: h, events: events, stays: stays}
}

// hooks is the business logic around the generated standard methods.
type hooks struct {
	salespbv1svc.NopSalesServiceHooks

	repos  salespbv1svc.SalesServiceRepositories
	venues venue.VenueRepository
	menus  catalog.MenuRepository
	pricer Pricer
	now    func() time.Time
	// contacts name the guests SummarizePipeline lists (pipeline.go); nil
	// until ReadContactsFrom, and then it lists their resource names.
	contacts guest.ContactRepository
}

// The resource types errors name.
const (
	quoteType = "aperio.sales.v1/Quote"
	venueType = "aperio.venue.v1/Venue"
)

// currency is the currency venue keeps its books in, which every price of it
// is stated in.
func (h *hooks) currency(ctx context.Context, venueName string) (string, error) {
	v, err := h.venues.Get(ctx, venueName)
	if err != nil {
		return "", servicex.NotFoundAs(err, venueType, venueName)
	}
	return v.GetCurrencyCode(), nil
}
