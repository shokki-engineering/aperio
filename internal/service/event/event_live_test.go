// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package event_test

// Live tests of an event's sessions: each takes one of its space's resources
// for setup to teardown, a session past the space's capacity is refused, and
// cancelling the event gives every session's space back. Run as test/live
// says.

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/shokki-engineering/aperio/internal/booking"
	"github.com/shokki-engineering/aperio/internal/database/repository/aperio/guest"
	"github.com/shokki-engineering/aperio/internal/database/repository/aperio/venue"
	"github.com/shokki-engineering/aperio/internal/service/event"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/event/v1/eventpbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/guest/v1/guestpbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/type/v1/typepbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/venue/v1/venuepbv1"
	"github.com/shokki-engineering/aperio/test/live"
	"google.golang.org/genproto/googleapis/type/date"
	"google.golang.org/genproto/googleapis/type/postaladdress"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// fixture is a venue with a room type of two rooms, a contact, and the
// service over them.
type fixture struct {
	svc     *event.Server
	fb      *live.Freebusy
	venue   string
	rooms   string // a space of two rooms
	contact string
	run     string
}

func setup(t *testing.T) *fixture {
	t.Helper()
	ctx, conn := context.Background(), live.Conn(t)
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
	return &fixture{svc: event.New(live.EventRepositories(conn), booking.NewBase(fb, venues, guest.New(conn).Contacts)), fb: fb, venue: v.GetName(), rooms: sp.GetName(), contact: c.GetName(), run: run}
}

func TestLiveEventSessions(t *testing.T) {
	f, ctx := setup(t), context.Background()

	ev, err := f.svc.CreateEvent(ctx, &eventpbv1.CreateEventRequest{
		Parent: f.venue,
		Event: &eventpbv1.Event{
			DisplayName: "Offsite", Contact: f.contact, EventType: typepbv1.EventType_EVENT_TYPE_CONFERENCE,
			Dates:  &typepbv1.DateRange{StartDate: &date.Date{Year: 2026, Month: 12, Day: 5}, EndDate: &date.Date{Year: 2026, Month: 12, Day: 5}},
			Guests: 80,
		},
	})
	if err != nil {
		t.Fatalf("create event: %v", err)
	}

	// session is 10:00-13:00 on 5 Dec 2026, set up from 09:00.
	start := time.Date(2026, 12, 5, 10, 0, 0, 0, time.UTC)
	session := func() *eventpbv1.CreateSessionRequest {
		return &eventpbv1.CreateSessionRequest{Parent: ev.GetName(), Session: &eventpbv1.Session{
			DisplayName: "Keynote", Space: f.rooms, Guests: 80,
			SetupTime: timestamppb.New(start.Add(-time.Hour)),
			StartTime: timestamppb.New(start), EndTime: timestamppb.New(start.Add(3 * time.Hour)),
		}}
	}

	// The space has two resources: two sessions fit, a third does not.
	for i := 0; i < 2; i++ {
		s, err := f.svc.CreateSession(ctx, session())
		if err != nil {
			t.Fatalf("create session %d: %v", i, err)
		}
		if s.GetFreebusyReservation() == "" {
			t.Errorf("session %d has no freebusy reservation", i)
		}
	}
	if _, err := f.svc.CreateSession(ctx, session()); err == nil {
		t.Error("a session past the space's capacity was accepted")
	}

	ev, err = f.svc.CancelEvent(ctx, &eventpbv1.CancelEventRequest{Name: ev.GetName(), Reason: "test"})
	if err != nil {
		t.Fatalf("cancel event: %v", err)
	}
	if ev.GetState() != eventpbv1.Event_STATE_CANCELLED || f.fb.Active() != 0 {
		t.Errorf("after cancel: state %v, %d still held; want CANCELLED, 0", ev.GetState(), f.fb.Active())
	}
}
