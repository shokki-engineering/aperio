// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package sales_test

// Live tests of what a lead's spaces and an accepted event's hall holds
// take in freebusy: a lead naming the ballroom holds it, so a second lead
// for it that day holds nothing, and a lead naming the room type holds as
// many of its rooms as it asks; an accepted wedding records its hall hold
// on the event (freebusy_holds), cancelling the event gives back exactly
// that, and the first session in the hall takes it off the list. Run as
// sales_live_test.go says.

import (
	"context"
	"testing"
	"time"

	"github.com/shokki-engineering/aperio/protobuf/generated/go/event/v1/eventpbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/sales/v1/salespbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/type/v1/typepbv1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestLiveLeadHoldsItsSpaces(t *testing.T) {
	f, ctx := holdSetup(t), context.Background()
	lead := func(name string, spaces []string, rooms int32) *salespbv1.Lead {
		t.Helper()
		l, err := f.svc.CreateLead(ctx, &salespbv1.CreateLeadRequest{Lead: &salespbv1.Lead{
			DisplayName: name, Contact: f.contact, Venue: f.venue, Kind: salespbv1.LeadKind_LEAD_KIND_EVENT,
			Dates: []*typepbv1.DateRange{dateRange(2027, 12, 20, 20)}, Spaces: spaces, Rooms: rooms,
		}})
		if err != nil {
			t.Fatalf("create lead %s: %v", name, err)
		}
		return l
	}
	hall := lead("Doe wedding", []string{f.hall}, 0)
	if _, n, held := f.fb.HoldOf(hall.GetHold()); hall.GetHold() == "" || !held || n != 1 {
		t.Fatalf("hall lead hold %q: %d, active %v; want the ballroom held", hall.GetHold(), n, held)
	}
	if again := lead("Doe reception", []string{f.hall}, 0); again.GetHold() != "" {
		t.Errorf("a second lead for the held ballroom holds %q; want nothing, as the hall is taken", again.GetHold())
	}
	rooms := lead("Doe guests", []string{f.rooms}, 2)
	if _, n, held := f.fb.HoldOf(rooms.GetHold()); !held || n != 2 {
		t.Errorf("room-type lead hold %q: %d, active %v; want 2 of its rooms", rooms.GetHold(), n, held)
	}
	if n := f.fb.Active(); n != 2 {
		t.Errorf("%d claims; want the ballroom and the two rooms' holds", n)
	}
}

func TestLiveEventRecordsItsHallHolds(t *testing.T) {
	f, ctx := holdSetup(t), context.Background()
	accept := func() *eventpbv1.Event {
		t.Helper()
		q, err := f.svc.AcceptQuote(ctx, &salespbv1.AcceptQuoteRequest{Name: f.wedding(t, ctx).GetName()})
		if err != nil {
			t.Fatalf("accept the wedding: %v", err)
		}
		e, err := f.events.GetEvent(ctx, &eventpbv1.GetEventRequest{Name: q.GetBooking()})
		if err != nil || len(e.GetFreebusyHolds()) != 1 || f.fb.Active() != 1 {
			t.Fatalf("accepted event holds %v (%d claims), %v; want its hall's hold recorded", e.GetFreebusyHolds(), f.fb.Active(), err)
		}
		return e
	}

	// Cancelling gives back exactly the holds it records.
	cancelled, err := f.events.CancelEvent(ctx, &eventpbv1.CancelEventRequest{Name: accept().GetName(), Reason: "John Doe called it off"})
	if err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if n := f.fb.Active(); n != 0 || len(cancelled.GetFreebusyHolds()) != 1 || cancelled.GetCancelReason() == "" {
		t.Errorf("after cancel: %d claims, holds %v, reason %q; want the hall given back and the record kept",
			n, cancelled.GetFreebusyHolds(), cancelled.GetCancelReason())
	}

	// The first session in the hall replaces the hold and takes it off.
	e := accept()
	start := time.Date(2027, 12, 18, 12, 30, 0, 0, time.UTC)
	if _, err := f.events.CreateSession(ctx, &eventpbv1.CreateSessionRequest{Parent: e.GetName(), Session: &eventpbv1.Session{
		DisplayName: "Pheras", Space: f.hall, Guests: 200, StartTime: timestamppb.New(start), EndTime: timestamppb.New(start.Add(5 * time.Hour)),
	}}); err != nil {
		t.Fatalf("session: %v", err)
	}
	got, err := f.events.GetEvent(ctx, &eventpbv1.GetEventRequest{Name: e.GetName()})
	if err != nil || len(got.GetFreebusyHolds()) != 0 || f.fb.Active() != 1 {
		t.Errorf("after the session: holds %v, %d claims, %v; want none recorded, the session's booking alone", got.GetFreebusyHolds(), f.fb.Active(), err)
	}
}
