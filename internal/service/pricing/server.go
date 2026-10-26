// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package pricing serves aperio.pricing.v1 -- seasons, price rules, and the
// price of anything a venue sells on a date -- on the generated AIP server.
// CalculatePrice (price.go) is the one way aperio prices anything: quotes,
// the concierge and agents all ask it. The rules themselves -- which rule
// wins, what a season covers, exact money -- are internal/pricing's.
package pricing

import (
	"context"

	"github.com/shokki-engineering/aperio/internal/database/repository/aperio/venue"
	"github.com/shokki-engineering/aperio/internal/database/repository/repox"
	"github.com/shokki-engineering/aperio/internal/service/generated/aperio/catalog/v1/catalogpbv1svc"
	"github.com/shokki-engineering/aperio/internal/service/generated/aperio/pricing/v1/pricingpbv1svc"
	"github.com/shokki-engineering/aperio/internal/service/generated/servicex"
)

// Server serves aperio.pricing.v1.PricingService: the generated standard
// methods for seasons and price rules, and CalculatePrice.
type Server struct {
	*pricingpbv1svc.PricingServiceServer
	catalog catalogpbv1svc.CatalogServiceRepositories // what is priced
	venues  venue.VenueRepository                     // the currency it is priced in
}

// New returns the Server over repos, pricing what catalog sells in each
// venue's currency, read from venues.
func New(repos pricingpbv1svc.PricingServiceRepositories, catalog catalogpbv1svc.CatalogServiceRepositories,
	venues venue.VenueRepository, opts ...servicex.Option) *Server {
	return &Server{
		PricingServiceServer: pricingpbv1svc.NewPricingServiceServer(repos, pricingpbv1svc.NopPricingServiceHooks{}, opts...),
		catalog:              catalog,
		venues:               venues,
	}
}

// venueType is the resource type errors name.
const venueType = "aperio.venue.v1/Venue"

// currency is the currency venueName keeps its books in, which every price
// of it is stated in.
func (s *Server) currency(ctx context.Context, venueName string) (string, error) {
	v, err := s.venues.Get(ctx, venueName)
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
