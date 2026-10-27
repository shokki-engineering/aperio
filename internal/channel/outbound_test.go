// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package channel

// outbound_test.go checks what is sent to the channels: rates night by
// night from the pricer, nights alike run together, and the rooms left from
// freebusy (test/live's, in memory) counted per night.

import (
	"context"
	"testing"
	"time"

	"github.com/shokki-engineering/aperio/internal/engine"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/pricing/v1/pricingpbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/venue/v1/venuepbv1"
	"github.com/shokki-engineering/aperio/test/live"
	"google.golang.org/genproto/googleapis/type/date"
	"google.golang.org/genproto/googleapis/type/money"
)

// weekendPricer prices a room 8,000 a night, 9,600 on the 5th and 6th.
type weekendPricer struct{}

func (weekendPricer) CalculatePrice(_ context.Context, req *pricingpbv1.CalculatePriceRequest) (*pricingpbv1.CalculatePriceResponse, error) {
	units := int64(8000)
	if d := req.GetServiceDate().GetDay(); d == 5 || d == 6 {
		units = 9600
	}
	return &pricingpbv1.CalculatePriceResponse{UnitPrice: &money.Money{CurrencyCode: "INR", Units: units}}, nil
}

// twoRooms is a venue with one room type of two rooms.
type twoRooms struct{}

func (twoRooms) GetVenue(context.Context, *venuepbv1.GetVenueRequest) (*venuepbv1.Venue, error) {
	return &venuepbv1.Venue{Name: "venues/v", TimeZone: "Asia/Kolkata"}, nil
}

func (twoRooms) GetSpace(context.Context, *venuepbv1.GetSpaceRequest) (*venuepbv1.Space, error) {
	return &venuepbv1.Space{Name: "venues/v/spaces/sv", Resources: []string{"resources/a", "resources/b"}}, nil
}

var testHotel = Hotel{Code: "7777001", Venue: "venues/v", Rooms: []RoomType{{Code: "SV-DLX", Space: "venues/v/spaces/sv", RatePlan: "BAR"}}}

func march(d int32) *date.Date { return &date.Date{Year: 2027, Month: 3, Day: d} }

func TestRates(t *testing.T) {
	o := &Outbound{Prices: weekendPricer{}}
	rates, err := o.Rates(context.Background(), testHotel, march(1), 8)
	if err != nil {
		t.Fatal(err)
	}
	want := []struct {
		start, end int32
		units      int64
	}{{1, 4, 8000}, {5, 6, 9600}, {7, 8, 8000}}
	if len(rates) != len(want) {
		t.Fatalf("rates = %+v; want %d runs", rates, len(want))
	}
	for i, w := range want {
		r := rates[i]
		if r.Start.GetDay() != w.start || r.End.GetDay() != w.end || r.Amount.GetUnits() != w.units || r.RatePlanCode != "BAR" {
			t.Errorf("run %d = %v..%v %v; want %+v", i, r.Start, r.End, r.Amount, w)
		}
	}
}

func TestAvailability(t *testing.T) {
	fb := live.NewFreebusy()
	ctx := context.Background()
	o := &Outbound{Venues: twoRooms{}, Engine: fb}
	v, _ := twoRooms{}.GetVenue(ctx, nil)
	loc, _ := time.LoadLocation(v.GetTimeZone())
	// One room taken the nights of the 3rd and 4th, both on the 4th.
	for _, b := range []engine.Booking{
		{Key: "a", Resources: []string{"resources/a", "resources/b"}, Window: engine.Stay(march(3), march(5), loc), Quantity: 1},
		{Key: "b", Resources: []string{"resources/a", "resources/b"}, Window: engine.Stay(march(4), march(5), loc), Quantity: 1},
	} {
		if _, err := fb.Book(ctx, b); err != nil {
			t.Fatal(err)
		}
	}
	avail, err := o.Availability(ctx, testHotel, march(1), 6)
	if err != nil {
		t.Fatal(err)
	}
	want := []struct{ start, end, free int32 }{{1, 2, 2}, {3, 3, 1}, {4, 4, 0}, {5, 6, 2}}
	if len(avail) != len(want) {
		t.Fatalf("availability = %+v; want %d runs", avail, len(want))
	}
	for i, w := range want {
		a := avail[i]
		if a.Start.GetDay() != w.start || a.End.GetDay() != w.end || a.Free != w.free || a.TypeCode != "SV-DLX" {
			t.Errorf("run %d = %v..%v free %d; want %+v", i, a.Start, a.End, a.Free, w)
		}
	}
}
