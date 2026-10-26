// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package booking_test

// Live tests of an event's sessions: each takes one of its space's resources
// for setup to teardown, a session past the space's capacity is refused, an
// event with sessions is not deleted without force, and cancelling the event
// gives every session's space back. Run as bookings_live_test.go says.

import (
	"context"
	"testing"
	"time"

	"github.com/shokki-engineering/aperio/protobuf/generated/go/booking/v1/bookingpbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/type/v1/typepbv1"
	"google.golang.org/genproto/googleapis/type/date"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestLiveEventSessions(t *testing.T) {
	f, ctx := setup(t), context.Background()

	ev, err := f.svc.CreateEvent(ctx, &bookingpbv1.CreateEventRequest{
		Parent: f.venue,
		Event: &bookingpbv1.Event{
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
	session := func() *bookingpbv1.CreateSessionRequest {
		return &bookingpbv1.CreateSessionRequest{Parent: ev.GetName(), Session: &bookingpbv1.Session{
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

	if _, err := f.svc.DeleteEvent(ctx, &bookingpbv1.DeleteEventRequest{Name: ev.GetName()}); err == nil {
		t.Error("an event with sessions was deleted without force")
	}

	ev, err = f.svc.CancelEvent(ctx, &bookingpbv1.CancelEventRequest{Name: ev.GetName(), Reason: "test"})
	if err != nil {
		t.Fatalf("cancel event: %v", err)
	}
	if ev.GetState() != bookingpbv1.Event_STATE_CANCELLED || f.fb.active() != 0 {
		t.Errorf("after cancel: state %v, %d still held; want CANCELLED, 0", ev.GetState(), f.fb.active())
	}
}
