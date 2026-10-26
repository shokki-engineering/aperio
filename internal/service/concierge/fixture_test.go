// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package concierge_test

// fixture_test.go is what the concierge's live tests share: a resort with
// two Sea View Deluxe rooms at 14,000 rupees a night, and the concierge over
// it with freebusy faked (test/live.Freebusy).

import (
	"context"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/shokki-engineering/aperio/internal/booking"
	"github.com/shokki-engineering/aperio/internal/database/repository/aperio/venue"
	"github.com/shokki-engineering/aperio/internal/service/concierge"
	"github.com/shokki-engineering/aperio/internal/service/generated/aperio/guest/v1/guestpbv1svc"
	"github.com/shokki-engineering/aperio/internal/service/guest"
	"github.com/shokki-engineering/aperio/internal/service/pricing"
	"github.com/shokki-engineering/aperio/internal/service/stay"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/pricing/v1/pricingpbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/type/v1/typepbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/venue/v1/venuepbv1"
	"github.com/shokki-engineering/aperio/test/live"
	"google.golang.org/genproto/googleapis/type/money"
	"google.golang.org/genproto/googleapis/type/postaladdress"
)

// resort is the fixture.
type resort struct {
	svc    *concierge.Server
	fb     *live.Freebusy
	stays  *stay.Server
	guests *guestpbv1svc.GuestServiceServer
	venue  *venuepbv1.Venue
	space  *venuepbv1.Space
	run    string
}

func newResort(t *testing.T) *resort {
	t.Helper()
	ctx, conn := context.Background(), live.Conn(t)
	must := func(err error, what string) {
		t.Helper()
		if err != nil {
			t.Fatalf("%s: %v", what, err)
		}
	}
	venues, fb := venue.New(conn), live.NewFreebusy()
	guests := guest.New(live.GuestRepositories(conn))
	bookings := stay.New(live.StayRepositories(conn), booking.NewBase(fb, venues, live.GuestRepositories(conn).Contacts))
	pricer := pricing.New(live.PricingRepositories(conn), live.CatalogRepositories(conn), venues.Venues)
	svc := concierge.New(venues, fb, pricer, guests, bookings, []byte(strings.Repeat("k", 32)))

	run := strconv.FormatInt(time.Now().UnixNano(), 10)
	v, err := venues.Venues.Create(ctx, &venuepbv1.Venue{DisplayName: "Concierge Resort " + run, Kind: venuepbv1.VenueKind_VENUE_KIND_STAYS,
		TimeZone: "Asia/Kolkata", CurrencyCode: "INR", Address: &postaladdress.PostalAddress{RegionCode: "IN"},
		Gst: &typepbv1.Gst{Gstin: "29ABCDE1234F1Z5", LegalDisplayName: "Test", GstStateCode: "29"}})
	must(err, "create venue")
	sp, err := venues.Spaces.Create(ctx, v.GetName(), &venuepbv1.Space{DisplayName: "Sea View Deluxe", Kind: venuepbv1.SpaceKind_SPACE_KIND_ROOM_TYPE,
		Resources: []string{"resources/sv1-" + run, "resources/sv2-" + run}, Capacity: &venuepbv1.Capacity{MaxAdults: 2}})
	must(err, "create space")
	_, err = pricer.CreatePriceRule(ctx, &pricingpbv1.CreatePriceRuleRequest{Parent: v.GetName(), PriceRule: &pricingpbv1.PriceRule{
		DisplayName: "Sea View rate", Target: pricingpbv1.PriceTarget_PRICE_TARGET_SPACE, Items: []string{sp.GetName()}, Active: true,
		Adjustment: &pricingpbv1.PriceRule_FixedPrice{FixedPrice: &money.Money{CurrencyCode: "INR", Units: 14000}},
	}})
	must(err, "create rate")
	return &resort{svc: svc, fb: fb, stays: bookings, guests: guests, venue: v, space: sp, run: run}
}
