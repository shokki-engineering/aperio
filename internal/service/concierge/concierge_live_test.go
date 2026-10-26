// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package concierge_test

// A live test of a guest's find, hold and book over the dev database, with
// freebusy faked (test/live.Freebusy): a resort with two Sea View Deluxe rooms at
// ₹14,000 a night. Run as test/live says.

import (
	"context"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shokki-engineering/aperio/internal/booking"
	"github.com/shokki-engineering/aperio/internal/database/repository/aperio/venue"
	"github.com/shokki-engineering/aperio/internal/service/concierge"
	"github.com/shokki-engineering/aperio/internal/service/guest"
	"github.com/shokki-engineering/aperio/internal/service/pricing"
	"github.com/shokki-engineering/aperio/internal/service/stay"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/concierge/v1/conciergepbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/guest/v1/guestpbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/pricing/v1/pricingpbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/stay/v1/staypbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/type/v1/typepbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/venue/v1/venuepbv1"
	"github.com/shokki-engineering/aperio/test/live"
	"google.golang.org/genproto/googleapis/type/date"
	"google.golang.org/genproto/googleapis/type/money"
	"google.golang.org/genproto/googleapis/type/postaladdress"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestLiveFindHoldConfirm(t *testing.T) {
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

	arrive := &date.Date{Year: int32(time.Now().Year()) + 1, Month: 12, Day: 12}
	found, err := svc.FindStay(ctx, &conciergepbv1.FindStayRequest{Venue: v.GetName(), ArrivalDate: arrive, Nights: 2, Adults: 2})
	must(err, "find")
	if len(found.GetOptions()) != 1 || found.GetOptions()[0].GetTotal().GetUnits() != 28000 || !strings.Contains(found.GetSpoken(), "28,000 rupees") {
		t.Fatalf("find = %v, %q; want Sea View Deluxe at 28,000 rupees", found.GetOptions(), found.GetSpoken())
	}
	if f, _ := svc.FindStay(ctx, &conciergepbv1.FindStayRequest{Venue: v.GetName(), ArrivalDate: arrive, Nights: 2, Adults: 5}); len(f.GetOptions()) != 0 {
		t.Errorf("a room for 2 was offered to 5 adults: %v", f.GetOptions())
	}

	hold := &conciergepbv1.HoldStayRequest{Venue: v.GetName(), Space: sp.GetName(), ArrivalDate: arrive, Nights: 2, Adults: 2, RequestId: uuid.NewString()}
	if _, err := svc.HoldStay(ctx, hold); status.Code(err) != codes.InvalidArgument || !strings.Contains(err.Error(), "name") {
		t.Errorf("hold with no guest = %v; want the guest's details asked for", err)
	}
	hold.GuestDisplayName, hold.Phone = "Priya Iyer", "98765 "+run[len(run)-5:]
	held, err := svc.HoldStay(ctx, hold)
	must(err, "hold")
	if !strings.Contains(held.GetSpoken(), "Shall I book it?") || fb.Active() != 1 {
		t.Errorf("hold: %q, %d claims; want a question and one claim", held.GetSpoken(), fb.Active())
	}
	if again, _ := svc.HoldStay(ctx, hold); again == nil || fb.Active() != 1 {
		t.Errorf("a retried hold took more rooms: %d claims", fb.Active())
	}

	if _, err := svc.ConfirmStay(ctx, &conciergepbv1.ConfirmStayRequest{Venue: v.GetName(), HoldToken: held.GetHoldToken() + "x"}); status.Code(err) != codes.InvalidArgument {
		t.Errorf("a tampered token = %v; want refused", err)
	}
	booked, err := svc.ConfirmStay(ctx, &conciergepbv1.ConfirmStayRequest{Venue: v.GetName(), HoldToken: held.GetHoldToken()})
	must(err, "confirm")
	if fb.Active() != 1 || fb.Booked() != 0 {
		t.Errorf("booking took %d claims, %d of its own; want the hold's one", fb.Active(), fb.Booked())
	}
	r, err := bookings.GetReservation(ctx, &staypbv1.GetReservationRequest{Name: booked.GetReservation()})
	must(err, "get stay")
	if r.GetState() != staypbv1.Reservation_STATE_PENDING || r.GetRooms()[0].GetSpace() != sp.GetName() || r.GetChannel() != typepbv1.Channel_CHANNEL_WEBSITE {
		t.Errorf("stay: %v, rooms %v, channel %v", r.GetState(), r.GetRooms(), r.GetChannel())
	}
	c, err := guests.GetContact(ctx, &guestpbv1.GetContactRequest{Name: r.GetContact()})
	must(err, "get guest")
	if want := "+9198765" + run[len(run)-5:]; len(c.GetPhoneNumbers()) != 1 || c.GetPhoneNumbers()[0] != want {
		t.Errorf("guest's phone numbers = %v; want [%s]", c.GetPhoneNumbers(), want)
	}
	if again, err := svc.ConfirmStay(ctx, &conciergepbv1.ConfirmStayRequest{Venue: v.GetName(), HoldToken: held.GetHoldToken()}); err != nil || again.GetReservation() != booked.GetReservation() {
		t.Errorf("a retried confirm = %v, %v; want the same stay", again.GetReservation(), err)
	}

	// The same guest, by phone, holding the second room: found, not added.
	second := &conciergepbv1.HoldStayRequest{Venue: v.GetName(), Space: sp.GetName(), ArrivalDate: arrive, Nights: 2, Adults: 2,
		GuestDisplayName: "Priya", Phone: hold.GetPhone(), RequestId: uuid.NewString()}
	held2, err := svc.HoldStay(ctx, second)
	must(err, "second hold")
	booked2, err := svc.ConfirmStay(ctx, &conciergepbv1.ConfirmStayRequest{Venue: v.GetName(), HoldToken: held2.GetHoldToken()})
	must(err, "second confirm")
	r2, _ := bookings.GetReservation(ctx, &staypbv1.GetReservationRequest{Name: booked2.GetReservation()})
	if r2.GetContact() != r.GetContact() {
		t.Errorf("a returning guest was added again: %q, then %q", r.GetContact(), r2.GetContact())
	}
	if _, err := svc.HoldStay(ctx, &conciergepbv1.HoldStayRequest{Venue: v.GetName(), Space: sp.GetName(), ArrivalDate: arrive, Nights: 2,
		Adults: 2, GuestDisplayName: "X", Phone: "9000000000", RequestId: uuid.NewString()}); status.Code(err) != codes.FailedPrecondition {
		t.Errorf("a third hold on two rooms = %v; want not available", err)
	}
}
