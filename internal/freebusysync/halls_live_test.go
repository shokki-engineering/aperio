// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package freebusysync_test

// A live test of the reconciler over the halls accepted events hold
// (Event.freebusy_holds): one an operator cancelled in freebusy and one
// freebusy no longer has are flagged on John Doe's events, once, and a
// moved one is followed. Run as fixture_live_test.go says.

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/shokki-engineering/aperio/internal/booking"
	"github.com/shokki-engineering/aperio/internal/engine"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/event/v1/eventpbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/type/v1/typepbv1"
	"google.golang.org/genproto/googleapis/type/date"
)

func TestLiveReconcilerFlagsHallHolds(t *testing.T) {
	f, ctx := setup(t), context.Background()
	held := func(day int32) *eventpbv1.Event {
		t.Helper()
		e, err := f.events.CreateEvent(ctx, &eventpbv1.CreateEventRequest{Parent: f.venue, EventId: "doe-" + f.run + "-" + string(rune('a'+day-20)), Event: &eventpbv1.Event{
			DisplayName: "Doe wedding", Contact: f.contact, EventType: typepbv1.EventType_EVENT_TYPE_WEDDING, Guests: 100,
			Dates: &typepbv1.DateRange{StartDate: &date.Date{Year: 2027, Month: 12, Day: day}, EndDate: &date.Date{Year: 2027, Month: 12, Day: day}},
		}})
		if err != nil {
			t.Fatalf("create event: %v", err)
		}
		if e, err = f.events.HoldHalls(ctx, e.GetName(), []booking.Hall{{Field: "quote.lines[0].item", Space: f.hall}}); err != nil || len(e.GetFreebusyHolds()) != 1 {
			t.Fatalf("hold the hall: %v, %v", e.GetFreebusyHolds(), err)
		}
		return e
	}
	cancelled, missing, moved := held(20), held(21), held(22)
	f.fb.CancelOutside(cancelled.GetFreebusyHolds()[0])
	f.fb.Forget(missing.GetFreebusyHolds()[0])
	from := moved.GetFreebusyHolds()[0]
	to := f.fb.MoveOutside(from, engine.Window{Start: time.Date(2027, 12, 23, 0, 0, 0, 0, time.UTC), End: time.Date(2027, 12, 24, 0, 0, 0, 0, time.UTC)})

	for round := range 2 { // the second flags nothing twice
		if err := f.sync.Reconciler().At(f.venue).Round(ctx); err != nil {
			t.Fatalf("round %d: %v", round, err)
		}
	}
	get := func(e *eventpbv1.Event) *eventpbv1.Event {
		t.Helper()
		got, err := f.events.GetEvent(ctx, &eventpbv1.GetEventRequest{Name: e.GetName()})
		if err != nil {
			t.Fatalf("get %s: %v", e.GetName(), err)
		}
		return got
	}
	for _, e := range []*eventpbv1.Event{cancelled, missing} {
		hold, got := e.GetFreebusyHolds()[0], get(e)
		if strings.Count(got.GetNotes(), booking.Mark(hold)) != 1 || got.GetState() == eventpbv1.Event_STATE_CANCELLED {
			t.Errorf("event %s after its hall hold went: %v, notes %q; want it flagged once, and left to a person", e.GetName(), got.GetState(), got.GetNotes())
		}
	}
	m := get(moved)
	if len(m.GetFreebusyHolds()) != 1 || m.GetFreebusyHolds()[0] != to || !strings.Contains(m.GetNotes(), "moved") {
		t.Errorf("moved hall hold: holds %v, notes %q; want %s and a flag", m.GetFreebusyHolds(), m.GetNotes(), to)
	}
}
