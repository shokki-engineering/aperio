// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package stay_test

// Live tests of the stay service over the dev database, with freebusy faked
// (test/live.Freebusy): rooms booked on create, a full date refused with no
// row left, a failed second line compensated, and cancel giving rooms back.
//
//	APERIO_TEST_DATABASE_URL="host=127.0.0.1 port=5433 user=postgres password=postgrespassword dbname=aperio sslmode=disable" \
//	  go test ./internal/service/stay/ -v

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

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
)

// fixture is a venue with a room type of two rooms, a contact, and the
// service over them.
type fixture struct {
	svc     *stay.Server
	fb      *live.Freebusy
	venue   string
	rooms   string // a space of two rooms
	contact string
	run     string
}

func setup(t *testing.T) *fixture {
	t.Helper()
	ctx, conn := context.Background(), live.Projected(live.Conn(t))
	run := fmt.Sprint(time.Now().UnixNano())
	venues := venue.New(conn)
	v, err := venues.Venues.Create(ctx, &venuepbv1.Venue{
		DisplayName: "Test Resort " + run, Kind: venuepbv1.VenueKind_VENUE_KIND_STAYS_AND_EVENTS,
		TimeZone: "Asia/Kolkata", CurrencyCode: "INR", Address: &postaladdress.PostalAddress{RegionCode: "IN"},
		Gst: &typepbv1.Gst{Gstin: "29ABCDE1234F1Z5", LegalDisplayName: "Test Hospitality", GstStateCode: "29"},
	})
	if err != nil {
		t.Fatalf("create venue: %v", err)
	}
	sp, err := venues.Spaces.Create(ctx, v.GetName(), &venuepbv1.Space{
		DisplayName: "Sea View", Kind: venuepbv1.SpaceKind_SPACE_KIND_ROOM_TYPE,
		Resources: []string{"resources/sv-101-" + run, "resources/sv-102-" + run},
	})
	if err != nil {
		t.Fatalf("create space: %v", err)
	}
	c, err := guest.New(conn).Contacts.Create(ctx, &guestpbv1.Contact{
		Kind: guestpbv1.ContactKind_CONTACT_KIND_PERSON, DisplayName: "Test Guest",
	})
	if err != nil {
		t.Fatalf("create contact: %v", err)
	}
	fb := live.NewFreebusy()
	base := booking.NewBase(fb, venues, guest.New(conn).Contacts)
	base.Search = conn
	return &fixture{svc: stay.New(live.StayRepositories(conn), base), fb: fb, venue: v.GetName(), rooms: sp.GetName(), contact: c.GetName(), run: run}
}

// stay asks for lines of rooms in the space, 1-3 Dec 2026.
func (f *fixture) stay(id string, lines ...int32) *staypbv1.CreateReservationRequest {
	r := &staypbv1.Reservation{
		Contact: f.contact, Channel: typepbv1.Channel_CHANNEL_WEBSITE, Adults: 2,
		ArrivalDate:   &date.Date{Year: 2026, Month: 12, Day: 1},
		DepartureDate: &date.Date{Year: 2026, Month: 12, Day: 3},
	}
	for _, n := range lines {
		r.Rooms = append(r.Rooms, &staypbv1.RoomLine{Space: f.rooms, Rooms: n})
	}
	return &staypbv1.CreateReservationRequest{Parent: f.venue, Reservation: r, ReservationId: id + "-" + f.run}
}

// gone fails t unless the reservation named by req is not stored.
func (f *fixture) gone(t *testing.T, req *staypbv1.CreateReservationRequest) {
	t.Helper()
	name := f.venue + "/reservations/" + req.GetReservationId()
	if _, err := f.svc.GetReservation(context.Background(), &staypbv1.GetReservationRequest{Name: name}); err == nil {
		t.Errorf("%s was left behind", name)
	}
}

func TestLiveReservationLifecycle(t *testing.T) {
	f, ctx := setup(t), context.Background()

	r, err := f.svc.CreateReservation(ctx, f.stay("first", 2))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if len(r.GetFreebusyReservations()) != 1 || f.fb.Active() != 1 {
		t.Fatalf("freebusy reservations = %v, active %d; want 1", r.GetFreebusyReservations(), f.fb.Active())
	}
	held := r.GetFreebusyReservations()[0]
	if n := f.fb.NoteOf(held); !n.AllDay || !strings.Contains(n.Summary, "Test Guest") || !strings.Contains(n.Summary, "2 rooms") {
		t.Errorf("calendar note = %+v; want an all-day note naming the guest and 2 rooms", n)
	}

	full := f.stay("full", 1)
	if _, err := f.svc.CreateReservation(ctx, full); err == nil {
		t.Fatal("a second stay in a full space was accepted")
	}
	f.gone(t, full)

	if r, err = f.svc.ConfirmReservation(ctx, &staypbv1.ConfirmReservationRequest{Name: r.GetName(), Etag: r.GetEtag()}); err != nil {
		t.Fatalf("confirm: %v", err)
	}
	if _, err := f.svc.DepartReservation(ctx, &staypbv1.DepartReservationRequest{Name: r.GetName()}); err == nil {
		t.Error("depart before arrive was accepted")
	}
	r, err = f.svc.CancelReservation(ctx, &staypbv1.CancelReservationRequest{Name: r.GetName(), Reason: "test"})
	if err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if r.GetState() != staypbv1.Reservation_STATE_CANCELLED || r.GetCancelTime() == nil || f.fb.Active() != 0 {
		t.Errorf("after cancel: state %v, cancel_time %v, %d still held", r.GetState(), r.GetCancelTime(), f.fb.Active())
	}
	if n := f.fb.NoteOf(held); n.Summary != "" {
		t.Errorf("the calendar note outlived the cancel: %+v", n)
	}
	if _, err := f.svc.CreateReservation(ctx, f.stay("again", 2)); err != nil {
		t.Errorf("the rooms were not freed by cancel: %v", err)
	}
}

func TestLiveReservationCompensates(t *testing.T) {
	f, ctx := setup(t), context.Background()
	f.fb.FailOn = "#rooms/1"

	req := f.stay("split", 1, 1)
	if _, err := f.svc.CreateReservation(ctx, req); err == nil {
		t.Fatal("create succeeded with a failing second line")
	}
	if n := f.fb.Active(); n != 0 {
		t.Errorf("%d freebusy reservations left after the failure; want 0", n)
	}
	f.gone(t, req)
}
