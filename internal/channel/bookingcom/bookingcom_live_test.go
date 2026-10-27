// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package bookingcom_test

// Live tests of Booking.com's inbound endpoint over the dev database, with
// freebusy faked (test/live.Freebusy): the sample messages posted to the
// Handler as Booking.com would, through the real stay and guest services.
//
//	APERIO_TEST_DATABASE_URL="host=127.0.0.1 port=5433 user=postgres password=postgrespassword dbname=aperio sslmode=disable" \
//	  go test ./internal/channel/bookingcom/ -v

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/shokki-engineering/aperio/internal/booking"
	"github.com/shokki-engineering/aperio/internal/channel"
	"github.com/shokki-engineering/aperio/internal/channel/bookingcom"
	"github.com/shokki-engineering/aperio/internal/database/repository/aperio/venue"
	"github.com/shokki-engineering/aperio/internal/service/guest"
	"github.com/shokki-engineering/aperio/internal/service/stay"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/guest/v1/guestpbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/stay/v1/staypbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/type/v1/typepbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/venue/v1/venuepbv1"
	"github.com/shokki-engineering/aperio/test/live"
	"google.golang.org/genproto/googleapis/type/postaladdress"
)

// fixture is a venue with a room type of two rooms, mapped as Booking.com's
// hotel 7777001 and room type SV-DLX, and the endpoint over the services.
type fixture struct {
	t       *testing.T
	handler *bookingcom.Handler
	stays   *stay.Server
	guests  *guest.Server
	fb      *live.Freebusy
	venue   string
	phone   string // the guest's, this run's own, as is their email
}

func setup(t *testing.T) *fixture {
	t.Helper()
	ctx, conn := context.Background(), live.Projected(live.Conn(t))
	run := fmt.Sprint(time.Now().UnixNano())
	venues := venue.New(conn)
	v, err := venues.Venues.Create(ctx, &venuepbv1.Venue{
		DisplayName: "Channel Resort " + run, Kind: venuepbv1.VenueKind_VENUE_KIND_STAYS_AND_EVENTS,
		TimeZone: "Asia/Kolkata", CurrencyCode: "INR", Address: &postaladdress.PostalAddress{RegionCode: "IN"},
		Gst: &typepbv1.Gst{Gstin: "29ABCDE1234F1Z5", LegalDisplayName: "Test Hospitality", GstStateCode: "29"},
	})
	if err != nil {
		t.Fatalf("create venue: %v", err)
	}
	sp, err := venues.Spaces.Create(ctx, v.GetName(), &venuepbv1.Space{
		DisplayName: "Sea View Deluxe", Kind: venuepbv1.SpaceKind_SPACE_KIND_ROOM_TYPE,
		Resources: []string{"resources/sv-101-" + run, "resources/sv-102-" + run},
	})
	if err != nil {
		t.Fatalf("create space: %v", err)
	}
	guestRepos := live.GuestRepositories(conn)
	fb := live.NewFreebusy()
	base := booking.NewBase(fb, venues, guestRepos.Contacts)
	base.Search = conn
	f := &fixture{t: t, stays: stay.New(live.StayRepositories(conn), base), guests: guest.New(guestRepos), fb: fb,
		venue: v.GetName(), phone: "+9197" + run[len(run)-8:]}
	hotels := []channel.Hotel{{Code: "7777001", Venue: v.GetName(), Rooms: []channel.RoomType{{Code: "SV-DLX", Space: sp.GetName(), RatePlan: "BAR"}}}}
	f.handler = bookingcom.NewHandler(bookingcom.NewSync(hotels, f.stays, f.guests), "bcom-user", "bcom-secret")
	return f
}

// post posts testdata/file as Booking.com would and returns the answer.
func (f *fixture) post(file string) string {
	f.t.Helper()
	body, err := os.ReadFile(filepath.Join("testdata", file))
	if err != nil {
		f.t.Fatal(err)
	}
	msg := strings.NewReplacer("+91 98000 00001", f.phone, "john.doe@", "john.doe."+f.phone[1:]+"@").Replace(string(body))
	req := httptest.NewRequest(http.MethodPost, bookingcom.Path, strings.NewReader(msg))
	req.SetBasicAuth("bcom-user", "bcom-secret")
	w := httptest.NewRecorder()
	f.handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		f.t.Fatalf("%s: HTTP %d %s", file, w.Code, w.Body)
	}
	return w.Body.String()
}

// stays are the venue's stays, newest first.
func (f *fixture) list() []*staypbv1.Reservation {
	f.t.Helper()
	out, err := f.stays.ListReservations(context.Background(), &staypbv1.ListReservationsRequest{Parent: f.venue, OrderBy: "create_time desc"})
	if err != nil {
		f.t.Fatal(err)
	}
	return out.GetReservations()
}

func TestLiveInboundReservation(t *testing.T) {
	f := setup(t)

	rs := f.post("new.xml")
	if !strings.Contains(rs, "<Success>") || !strings.Contains(rs, `ResID_Value="4105349872"`) {
		t.Fatalf("answer to new:\n%s", rs)
	}
	got := f.list()
	if len(got) != 1 {
		t.Fatalf("%d stays after new; want 1", len(got))
	}
	r := got[0]
	if r.GetChannel().String() != "CHANNEL_BOOKING_COM" || r.GetChannelReference() != "4105349872" ||
		r.GetState() != staypbv1.Reservation_STATE_CONFIRMED || r.GetAdults() != 2 || r.GetTotal().GetUnits() != 18000 ||
		len(r.GetRoomStays()) != 1 || r.GetRoomStays()[0].GetRoomTypes()[0].GetRoomTypeCode() != "SV-DLX" || len(r.GetResGuests()) != 1 {
		t.Errorf("stay = %v", r)
	}
	if f.fb.Active() != 1 {
		t.Errorf("%d rooms held in freebusy; want 1", f.fb.Active())
	}
	c, err := f.guests.GetContact(context.Background(), &guestpbv1.GetContactRequest{Name: r.GetContact()})
	if err != nil || c.GetDisplayName() != "John Doe" || len(c.GetPhoneNumbers()) != 1 || c.GetPhoneNumbers()[0] != f.phone {
		t.Errorf("guest = %v, %v", c, err)
	}

	// Booking.com delivers again: the same stay, no second booking.
	if rs := f.post("new.xml"); !strings.Contains(rs, "<Success>") {
		t.Fatalf("answer to redelivery:\n%s", rs)
	}
	if n := len(f.list()); n != 1 || f.fb.Active() != 1 {
		t.Fatalf("after redelivery: %d stays, %d held; want 1 and 1", n, f.fb.Active())
	}

	// A third guest: changed in place.
	f.post("modify.xml")
	got = f.list()
	if len(got) != 1 || got[0].GetAdults() != 3 || got[0].GetTotal().GetUnits() != 21000 || got[0].GetName() != r.GetName() {
		t.Fatalf("after modify: %d stays, first %v", len(got), got[0])
	}

	// A night more: rebooked, the old stay cancelled and its rooms given back.
	f.post("modify_dates.xml")
	f.post("modify_dates.xml")
	got = f.list()
	if len(got) != 2 || got[0].GetDepartureDate().GetDay() != 13 || got[0].GetState() != staypbv1.Reservation_STATE_CONFIRMED ||
		got[1].GetState() != staypbv1.Reservation_STATE_CANCELLED || f.fb.Active() != 1 {
		t.Fatalf("after modify_dates: %d stays, %d held: %v", len(got), f.fb.Active(), got)
	}

	// Cancelled, twice.
	for range 2 {
		if rs := f.post("cancel.xml"); !strings.Contains(rs, "<Success>") {
			t.Fatalf("answer to cancel:\n%s", rs)
		}
	}
	got = f.list()
	if got[0].GetState() != staypbv1.Reservation_STATE_CANCELLED || f.fb.Active() != 0 {
		t.Errorf("after cancel: %v, %d held", got[0].GetState(), f.fb.Active())
	}
}

func TestLiveInboundRefusesBadLogin(t *testing.T) {
	f := setup(t)
	req := httptest.NewRequest(http.MethodPost, bookingcom.Path, strings.NewReader("<x/>"))
	req.SetBasicAuth("bcom-user", "wrong")
	w := httptest.NewRecorder()
	f.handler.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized || len(f.list()) != 0 {
		t.Errorf("bad login: HTTP %d, %d stays", w.Code, len(f.list()))
	}
}
