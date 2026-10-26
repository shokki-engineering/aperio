// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package concierge

// find.go is FindStay -- the room types with the rooms asked for free for
// every night, priced, cheapest first -- and what HoldStay shares with it:
// the venue's clock, the stay's window in freebusy, and its price, night by
// night through the sales service, so a season or a weekend rule prices each
// night as a quote would.

import (
	"context"
	"fmt"
	"math/big"
	"slices"
	"strings"
	"time"

	"github.com/shokki-engineering/aperio/internal/database/repository/repox"
	"github.com/shokki-engineering/aperio/internal/engine"
	"github.com/shokki-engineering/aperio/internal/pricing"
	"github.com/shokki-engineering/aperio/internal/service/generated/servicex"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/concierge/v1/conciergepbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/sales/v1/salespbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/venue/v1/venuepbv1"
	"google.golang.org/genproto/googleapis/type/date"
	"google.golang.org/genproto/googleapis/type/money"
)

// shown is how many options FindStay offers: as many as a guest weighs at once.
const shown = 3

// FindStay offers the room types free for the stay, cheapest first.
func (s *Server) FindStay(ctx context.Context, req *conciergepbv1.FindStayRequest) (*conciergepbv1.FindStayResponse, error) {
	return run(ctx, s, "FindStay", func(ctx context.Context) (*conciergepbv1.FindStayResponse, error) { return s.findStay(ctx, req) })
}

func (s *Server) findStay(ctx context.Context, req *conciergepbv1.FindStayRequest) (*conciergepbv1.FindStayResponse, error) {
	v, w, err := s.window(ctx, req.GetVenue(), req.GetArrivalDate(), req.GetNights())
	if err != nil {
		return nil, err
	}
	rooms := max(req.GetRooms(), 1)
	spaces, _, err := s.spaces.List(ctx, v.GetName(), repox.ListInput{PageSize: 200})
	if err != nil {
		return nil, err
	}
	out := &conciergepbv1.FindStayResponse{}
	for _, sp := range spaces {
		if !sleeps(sp, rooms, req.GetAdults(), req.GetChildren()) {
			continue
		}
		free, err := s.engine.Free(ctx, sp.GetResources(), w, rooms)
		if err != nil || !free {
			continue // a room type freebusy cannot answer for is not offered
		}
		total, err := s.price(ctx, v, sp.GetName(), req.GetArrivalDate(), req.GetNights(), rooms, req.GetAdults()+req.GetChildren())
		if err != nil {
			continue // a room type with no price for the dates is not offered
		}
		out.Options = append(out.Options, &conciergepbv1.StayOption{
			Space: sp.GetName(), DisplayName: sp.GetDisplayName(), Total: total,
			Spoken: fmt.Sprintf("%s, %s for %s", sp.GetDisplayName(), amount(total), count(req.GetNights(), "night")),
		})
	}
	slices.SortFunc(out.Options, func(a, b *conciergepbv1.StayOption) int {
		return pricing.Rat(a.GetTotal()).Cmp(pricing.Rat(b.GetTotal()))
	})
	out.Options = out.Options[:min(len(out.Options), shown)]
	out.Spoken = findSpoken(out.Options, rooms, req.GetArrivalDate(), req.GetNights())
	return out, nil
}

// findSpoken words FindStay's answer.
func findSpoken(opts []*conciergepbv1.StayOption, rooms int32, arrive *date.Date, nights int32) string {
	when := fmt.Sprintf("%s from %s", count(nights, "night"), day(arrive))
	if len(opts) == 0 {
		return fmt.Sprintf("Sorry, nothing is free for %s for %s.", count(rooms, "room"), when)
	}
	var said []string
	for _, o := range opts {
		said = append(said, o.GetSpoken())
	}
	return fmt.Sprintf("For %s, we have: %s, before taxes.", when, strings.Join(said, "; "))
}

// window is venueName and the window a stay of nights from arrive takes
// there, in the venue's time zone -- as the booking service reckons it, so a
// hold and the reservation made from it ask freebusy for the same window.
func (s *Server) window(ctx context.Context, venueName string, arrive *date.Date, nights int32) (*venuepbv1.Venue, engine.Window, error) {
	v, err := s.venues.Get(ctx, venueName)
	if err != nil {
		return nil, engine.Window{}, servicex.NotFoundAs(err, venueType, venueName)
	}
	loc, err := time.LoadLocation(v.GetTimeZone())
	if err != nil {
		loc = time.UTC
	}
	if engine.Midnight(arrive, loc).Before(engine.Midnight(today(s.now(), loc), loc)) {
		return nil, engine.Window{}, servicex.BadField("PAST_DATE", "arrival_date", "the arrival date has passed", "Ask for a date from today on")
	}
	return v, engine.Stay(arrive, engine.AddDays(arrive, int(nights)), loc), nil
}

// price is a stay in space -- rooms rooms, nights nights from arrive -- before
// tax: each night priced for its own date.
func (s *Server) price(ctx context.Context, v *venuepbv1.Venue, space string, arrive *date.Date, nights, rooms, guests int32) (*money.Money, error) {
	total := new(big.Rat)
	for i := range nights {
		p, err := s.prices.CalculatePrice(ctx, &salespbv1.CalculatePriceRequest{
			Venue: v.GetName(), Item: space, ServiceDate: engine.AddDays(arrive, int(i)), Quantity: rooms, Guests: guests,
		})
		if err != nil {
			return nil, err
		}
		total.Add(total, pricing.Rat(p.GetAmount()))
	}
	return pricing.Money(total, v.GetCurrencyCode()), nil
}

// sleeps reports whether rooms rooms of sp sleep the party: a room type with
// freebusy resources whose capacity, when stated, covers them.
func sleeps(sp *venuepbv1.Space, rooms, adults, children int32) bool {
	if len(sp.GetResources()) == 0 || sp.GetKind() == venuepbv1.SpaceKind_SPACE_KIND_HALL {
		return false
	}
	c := sp.GetCapacity()
	if c.GetMaxAdults() > 0 && c.GetMaxAdults()*rooms < adults {
		return false
	}
	if c.GetMaxAdults() > 0 && (c.GetMaxAdults()+c.GetMaxChildren())*rooms < adults+children {
		return false
	}
	return true
}

// today is now's date in loc.
func today(now time.Time, loc *time.Location) *date.Date {
	n := now.In(loc)
	return &date.Date{Year: int32(n.Year()), Month: int32(n.Month()), Day: int32(n.Day())}
}
