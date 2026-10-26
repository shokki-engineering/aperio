// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package selling serves aperio.sales.v1 (named so beside the repository
// package sales) on the generated AIP server
// (protoc-gen-service): the business logic of pricing and quoting.
//
//   - CalculatePrice (price.go) is the one way aperio prices anything: an
//     item's base price with the venue's matching price rule applied
//     (internal/pricing), and the GST on it.
//   - A quote (quotes.go) is priced line by line through it when it is made,
//     and again on every edit while it is a draft, so a draft follows the
//     rules as they change; its totals are the server's, never the caller's.
//   - SendQuote fixes the prices and starts the offer's clock; DeclineQuote
//     and ReviseQuote move it on (states.go); AcceptQuote makes its bookings
//     through the booking service and wins its lead (accept.go).
package selling

import (
	"context"
	"time"

	"github.com/shokki-engineering/aperio/internal/database/repository/aperio/crm"
	"github.com/shokki-engineering/aperio/internal/database/repository/aperio/venue"
	"github.com/shokki-engineering/aperio/internal/database/repository/repox"
	"github.com/shokki-engineering/aperio/internal/service/generated/aperio/sales/v1/salespbv1svc"
	"github.com/shokki-engineering/aperio/internal/service/generated/servicex"
)

// offerTTL is how long a sent quote stands. Quote.ttl is INPUT_ONLY and not
// stored, so it cannot reach SendQuote; until SendQuoteRequest carries one,
// every offer stands this long.
const offerTTL = 14 * 24 * time.Hour

// Server serves aperio.sales.v1.SalesService: the generated standard methods
// with the pricing hooks, and the custom methods.
type Server struct {
	*salespbv1svc.SalesServiceServer
	h      *hooks
	leads  crm.LeadRepository
	booker Booker // nil: AcceptQuote is Unimplemented
}

// New returns the Server over the sales repositories, reading each venue's
// currency from venues, winning leads in leads, and making the bookings an
// accepted quote becomes through booker.
func New(repos salespbv1svc.SalesServiceRepositories, venues venue.VenueRepository, leads crm.LeadRepository,
	booker Booker, opts ...servicex.Option) *Server {
	h := &hooks{repos: repos, venues: venues, now: time.Now}
	return &Server{SalesServiceServer: salespbv1svc.NewSalesServiceServer(repos, h, opts...), h: h, leads: leads, booker: booker}
}

// hooks is the business logic around the generated standard methods.
type hooks struct {
	salespbv1svc.NopSalesServiceHooks

	repos  salespbv1svc.SalesServiceRepositories
	venues venue.VenueRepository
	now    func() time.Time
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

// all reads every page of a List.
func all[T any](list func(repox.ListInput) ([]T, string, error)) ([]T, error) {
	var out []T
	in := repox.ListInput{PageSize: 500}
	for {
		page, next, err := list(in)
		if err != nil {
			return nil, err
		}
		out = append(out, page...)
		if next == "" {
			return out, nil
		}
		in.PageToken = next
	}
}
