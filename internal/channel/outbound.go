// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package channel

// outbound.go is what aperio tells the channels: each sold room type's price
// night by night, from the PricingService's CalculatePrice -- the one way
// aperio prices anything, so a season or a rule changes every channel at once
// -- and the rooms of it still free each night, from freebusy. Consecutive
// nights alike are sent as one run, as channels take them.

import (
	"context"
	"sort"
	"time"

	"github.com/shokki-engineering/aperio/internal/engine"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/pricing/v1/pricingpbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/venue/v1/venuepbv1"
	"google.golang.org/genproto/googleapis/type/date"
	"google.golang.org/protobuf/proto"
)

// Outbound computes the rates and availability a channel is sent.
type Outbound struct {
	Prices Prices
	Venues Venues
	Engine engine.Engine
}

// Rates are h's room types' prices for the nights nights from from, on each
// room type's rate plan.
func (o *Outbound) Rates(ctx context.Context, h Hotel, from *date.Date, nights int) ([]Rate, error) {
	var out []Rate
	for _, room := range h.Rooms {
		for i := range nights {
			d := engine.AddDays(from, i)
			p, err := o.Prices.CalculatePrice(ctx, &pricingpbv1.CalculatePriceRequest{
				Venue: h.Venue, Item: room.Space, ServiceDate: d, Quantity: 1,
			})
			if err != nil {
				return nil, err
			}
			if n := len(out); n > 0 && out[n-1].TypeCode == room.Code && proto.Equal(out[n-1].Amount, p.GetUnitPrice()) &&
				proto.Equal(engine.AddDays(out[n-1].End, 1), d) {
				out[n-1].End = d
				continue
			}
			out = append(out, Rate{TypeCode: room.Code, RatePlanCode: room.RatePlan, Start: d, End: d, Amount: p.GetUnitPrice()})
		}
	}
	return out, nil
}

// Availability is the rooms of each of h's room types free each of the
// nights nights from from, in freebusy.
func (o *Outbound) Availability(ctx context.Context, h Hotel, from *date.Date, nights int) ([]Availability, error) {
	v, err := o.Venues.GetVenue(ctx, &venuepbv1.GetVenueRequest{Name: h.Venue})
	if err != nil {
		return nil, err
	}
	loc, err := time.LoadLocation(v.GetTimeZone())
	if err != nil {
		loc = time.UTC
	}
	var out []Availability
	for _, room := range h.Rooms {
		sp, err := o.Venues.GetSpace(ctx, &venuepbv1.GetSpaceRequest{Name: room.Space})
		if err != nil {
			return nil, err
		}
		for i := range nights {
			d := engine.AddDays(from, i)
			free, err := o.free(ctx, sp.GetResources(), engine.Stay(d, engine.AddDays(d, 1), loc))
			if err != nil {
				return nil, err
			}
			if n := len(out); n > 0 && out[n-1].TypeCode == room.Code && out[n-1].Free == free &&
				proto.Equal(engine.AddDays(out[n-1].End, 1), d) {
				out[n-1].End = d
				continue
			}
			out = append(out, Availability{TypeCode: room.Code, Start: d, End: d, Free: free})
		}
	}
	return out, nil
}

// free is how many of resources are free for all of w: the most freebusy
// says are, found by halving (freebusy answers whether n are, and if n are,
// fewer are too).
func (o *Outbound) free(ctx context.Context, resources []string, w engine.Window) (int32, error) {
	var ferr error
	n := sort.Search(len(resources), func(i int) bool {
		if ferr != nil {
			return true
		}
		ok, err := o.Engine.Free(ctx, resources, w, int32(i+1))
		ferr = err
		return !ok
	})
	return int32(n), ferr
}
