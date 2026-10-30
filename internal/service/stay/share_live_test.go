// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package stay_test

// A live test of ShareReservation: Unimplemented with no link signer; with
// one, the guest site's page for the stay with a token that verifies to
// that reservation alone, for 14 days. Run as test/live says.

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/shokki-engineering/aperio/internal/auth"
	"github.com/shokki-engineering/aperio/internal/booking"
	"github.com/shokki-engineering/aperio/internal/database/repository/aperio/guest"
	"github.com/shokki-engineering/aperio/internal/database/repository/aperio/venue"
	"github.com/shokki-engineering/aperio/internal/service/stay"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/guest/v1/guestpbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/stay/v1/staypbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/type/v1/typepbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/venue/v1/venuepbv1"
	"github.com/shokki-engineering/aperio/test/live"
	"google.golang.org/genproto/googleapis/type/date"
	"google.golang.org/genproto/googleapis/type/postaladdress"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestLiveShareReservation(t *testing.T) {
	ctx, conn := context.Background(), live.Projected(live.Conn(t))
	must := func(err error, what string) {
		t.Helper()
		if err != nil {
			t.Fatalf("%s: %v", what, err)
		}
	}
	run, venues := fmt.Sprint(time.Now().UnixNano()), venue.New(conn)
	v, err := venues.Venues.Create(ctx, &venuepbv1.Venue{DisplayName: "Share Resort " + run, Kind: venuepbv1.VenueKind_VENUE_KIND_STAYS,
		TimeZone: "Asia/Kolkata", CurrencyCode: "INR", Address: &postaladdress.PostalAddress{RegionCode: "IN"},
		Gst: &typepbv1.Gst{Gstin: "29ABCDE1234F1Z5", LegalDisplayName: "Test", GstStateCode: "29"}})
	must(err, "create venue")
	sp, err := venues.Spaces.Create(ctx, v.GetName(), &venuepbv1.Space{DisplayName: "Garden", Kind: venuepbv1.SpaceKind_SPACE_KIND_ROOM_TYPE,
		Resources: []string{"resources/g-1-" + run}})
	must(err, "create space")
	c, err := guest.New(conn).Contacts.Create(ctx, &guestpbv1.Contact{Kind: guestpbv1.ContactKind_CONTACT_KIND_PERSON, DisplayName: "John Doe"})
	must(err, "create contact")
	base := booking.NewBase(live.NewFreebusy(), venues, guest.New(conn).Contacts)
	base.Search = conn
	svc := stay.New(live.StayRepositories(conn), base)
	r, err := svc.CreateReservation(ctx, &staypbv1.CreateReservationRequest{Parent: v.GetName(), ReservationId: "share-" + run,
		Reservation: &staypbv1.Reservation{Contact: c.GetName(), Channel: typepbv1.Channel_CHANNEL_WEBSITE, Adults: 2,
			ArrivalDate: &date.Date{Year: 2027, Month: 1, Day: 4}, DepartureDate: &date.Date{Year: 2027, Month: 1, Day: 6},
			Rooms: []*staypbv1.RoomLine{{Space: sp.GetName(), Rooms: 1}}}})
	must(err, "create reservation")

	req := &staypbv1.ShareReservationRequest{Name: r.GetName()}
	if _, err := svc.ShareReservation(ctx, req); status.Code(err) != codes.Unimplemented {
		t.Errorf("share with no signer = %v; want Unimplemented", err)
	}
	links := auth.NewLinks([]byte(strings.Repeat("s", 32)), nil)
	svc.ShareLinksWith(links, "https://guest.example")
	got, err := svc.ShareReservation(ctx, req)
	must(err, "share")
	page, token, ok := strings.Cut(got.GetUrl(), "?link=")
	if !ok || page != "https://guest.example/"+r.GetName() {
		t.Fatalf("url %q; want the guest site's page for %s with ?link=", got.GetUrl(), r.GetName())
	}
	if granted, err := links.VerifyLink(token); err != nil || granted != r.GetName() {
		t.Errorf("the token grants %q, %v; want %s", granted, err, r.GetName())
	}
	if left := time.Until(got.GetExpireTime().AsTime()); left < 14*24*time.Hour-time.Minute || left > 14*24*time.Hour {
		t.Errorf("the link lapses in %v; want 14 days", left)
	}
	if _, err := svc.ShareReservation(ctx, &staypbv1.ShareReservationRequest{Name: v.GetName() + "/reservations/missing"}); status.Code(err) != codes.NotFound {
		t.Errorf("sharing a missing reservation = %v; want NotFound", err)
	}
}
