// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package freebusysync_test

// Live tests of a session replacing the hall John Doe's accepted event
// holds: freebusy's webhook for the replaced hold landing before the
// session is done, and the event's list failing to change, flag nothing on
// the event; a session failing after the hold was given back puts it back
// on the list, to be flagged. Run as fixture_live_test.go says.

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/shokki-engineering/aperio/internal/booking"
	"github.com/shokki-engineering/aperio/internal/engine"
	"github.com/shokki-engineering/aperio/internal/service/event"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/event/v1/eventpbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/type/v1/typepbv1"
	"google.golang.org/genproto/googleapis/type/date"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// heldEvent is John Doe's event on 20 Dec 2027, holding the hall, served
// by svc, and its hold.
func (f *fixture) heldEvent(t *testing.T, svc *event.Server) (*eventpbv1.Event, string) {
	t.Helper()
	ctx := context.Background()
	e, err := svc.CreateEvent(ctx, &eventpbv1.CreateEventRequest{Parent: f.venue, EventId: "doe-" + f.run, Event: &eventpbv1.Event{
		DisplayName: "Doe wedding", Contact: f.contact, EventType: typepbv1.EventType_EVENT_TYPE_WEDDING, Guests: 100,
		Dates: &typepbv1.DateRange{StartDate: &date.Date{Year: 2027, Month: 12, Day: 20}, EndDate: &date.Date{Year: 2027, Month: 12, Day: 20}},
	}})
	if err != nil {
		t.Fatalf("create event: %v", err)
	}
	if e, err = svc.HoldHalls(ctx, e.GetName(), []booking.Hall{{Field: "quote.lines[0].item", Space: f.hall}}); err != nil || len(e.GetFreebusyHolds()) != 1 {
		t.Fatalf("hold the hall: %v, %v", e.GetFreebusyHolds(), err)
	}
	return e, e.GetFreebusyHolds()[0]
}

// session creates the reception, session id, in the hall through svc.
func (f *fixture) session(svc *event.Server, e *eventpbv1.Event, id string) (*eventpbv1.Session, error) {
	start := time.Date(2027, 12, 20, 13, 0, 0, 0, time.UTC)
	return svc.CreateSession(context.Background(), &eventpbv1.CreateSessionRequest{Parent: e.GetName(), SessionId: id, Session: &eventpbv1.Session{
		DisplayName: "Reception", Space: f.hall, Guests: 100,
		StartTime: timestamppb.New(start), EndTime: timestamppb.New(start.Add(4 * time.Hour)),
	}})
}

// after reconciles once and returns e as it stands, failing t when e's
// notes flag hold other than want times.
func (f *fixture) after(t *testing.T, e *eventpbv1.Event, hold string, want int) *eventpbv1.Event {
	t.Helper()
	ctx := context.Background()
	if err := f.sync.Reconciler().At(f.venue).Round(ctx); err != nil {
		t.Fatalf("round: %v", err)
	}
	got, err := f.events.GetEvent(ctx, &eventpbv1.GetEventRequest{Name: e.GetName()})
	if err != nil {
		t.Fatalf("get event: %v", err)
	}
	if n := strings.Count(got.GetNotes(), booking.Mark(hold)); n != want || got.GetState() == eventpbv1.Event_STATE_CANCELLED {
		t.Errorf("event %v, notes %q: hold %s flagged %d times; want %d, and the event kept", got.GetState(), got.GetNotes(), hold, n, want)
	}
	return got
}

func TestLiveWebhookDuringSessionFlagsNothing(t *testing.T) {
	f, ctx := setup(t), context.Background()
	e, hold := f.heldEvent(t, f.events)
	var webhook error
	f.fb.OnCancel = func(r string) { webhook = errors.Join(webhook, f.sync.Reservation(ctx, r)) }
	if _, err := f.session(f.events, e, "reception"); err != nil {
		t.Fatalf("create session: %v", err)
	}
	if webhook != nil {
		t.Fatalf("webhook: %v", webhook)
	}
	if got := f.after(t, e, hold, 0); len(got.GetFreebusyHolds()) != 0 {
		t.Errorf("holds %v after the session replaced the hold; want none", got.GetFreebusyHolds())
	}
}

func TestLiveHoldsConflictRetried(t *testing.T) {
	f := setup(t)
	e, hold := f.heldEvent(t, f.events)
	svc := event.New(flaky(f.repos, 0, 1, nil), f.base)
	if _, err := f.session(svc, e, "reception"); err != nil {
		t.Fatalf("create session past one conflict: %v", err)
	}
	if got := f.after(t, e, hold, 0); len(got.GetFreebusyHolds()) != 0 {
		t.Errorf("holds %v after the session replaced the hold; want none", got.GetFreebusyHolds())
	}
}

func TestLiveForgetFailureKeepsHold(t *testing.T) {
	f, ctx := setup(t), context.Background()
	e, hold := f.heldEvent(t, f.events)
	svc := event.New(flaky(f.repos, 0, -1, errors.New("database unavailable")), f.base)
	if _, err := f.session(svc, e, "reception"); err == nil {
		t.Fatal("create session while the event's holds cannot be written: want an error")
	}
	if got, _ := f.fb.Lookup(ctx, []string{hold}); got[hold].State != engine.Live {
		t.Errorf("hold %s is %v; want it kept live", hold, got[hold].State)
	}
	if got := f.after(t, e, hold, 0); len(got.GetFreebusyHolds()) != 1 {
		t.Errorf("holds %v; want the hold still listed", got.GetFreebusyHolds())
	}
}

func TestLiveFailedSessionRestoresHold(t *testing.T) {
	f := setup(t)
	e, hold := f.heldEvent(t, f.events)
	f.fb.FailOn = "/sessions/reception"
	if _, err := f.session(f.events, e, "reception"); err == nil {
		t.Fatal("create session freebusy refuses: want an error")
	}
	// The hold was given back for nothing: listed again, and flagged once.
	got := f.after(t, e, hold, 1)
	if len(got.GetFreebusyHolds()) != 1 || got.GetFreebusyHolds()[0] != hold {
		t.Errorf("holds %v; want %s back", got.GetFreebusyHolds(), hold)
	}
}
