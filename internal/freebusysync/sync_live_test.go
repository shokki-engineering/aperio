// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package freebusysync_test

// Live tests of staying in step with freebusy: a reservation cancelled in
// freebusy cancels John Doe's stay through the signed webhook, a delivery
// with a bad signature is refused, and the reconciler catches a broken
// link, a move and a hall cancelled under an event. Run as
// fixture_live_test.go says.

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/shokki-engineering/aperio/internal/engine"
	"github.com/shokki-engineering/aperio/internal/freebusysync"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/event/v1/eventpbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/stay/v1/staypbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/type/v1/typepbv1"
	"google.golang.org/genproto/googleapis/type/date"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestLiveWebhookCancelsStay(t *testing.T) {
	f := setup(t)
	r := f.stay(t, "webhook", 1)
	link := r.GetFreebusyReservations()[0]
	h := freebusysync.NewHandler(f.sync, secret)

	f.fb.CancelOutside(link)
	forged := "whsec_" + strings.Repeat("A", 44)
	if code := deliver(h, forged, "evt-bad-"+f.run, "freebusy.reservation.cancelled", link); code != http.StatusUnauthorized {
		t.Errorf("a delivery signed with another key answered %d; want 401", code)
	}
	if got := f.get(t, r.GetName()); got.GetState() == staypbv1.Reservation_STATE_CANCELLED {
		t.Fatal("a delivery with a bad signature cancelled the stay")
	}

	if code := deliver(h, secret, "evt-"+f.run, "freebusy.reservation.cancelled", link); code != http.StatusNoContent {
		t.Fatalf("the signed delivery answered %d; want 204", code)
	}
	got := f.get(t, r.GetName())
	if got.GetState() != staypbv1.Reservation_STATE_CANCELLED || got.GetCancelReason() != "cancelled in freebusy" {
		t.Errorf("stay after the webhook: %v, %q; want CANCELLED in freebusy", got.GetState(), got.GetCancelReason())
	}
	if code := deliver(h, secret, "evt-"+f.run, "freebusy.reservation.cancelled", link); code != http.StatusNoContent {
		t.Errorf("a redelivery answered %d; want 204", code)
	}
}

func TestLiveReconcilerRepairsAndFlags(t *testing.T) {
	f, ctx := setup(t), context.Background()
	broken := f.stay(t, "broken", 1)
	moved := f.stay(t, "moved", 1)
	lost := broken.GetFreebusyReservations()[0]
	f.fb.Forget(lost) // the link breaks: freebusy no longer has it

	from := moved.GetFreebusyReservations()[0]
	w := engine.Window{Start: time.Date(2027, 12, 11, 0, 0, 0, 0, time.UTC), End: time.Date(2027, 12, 13, 0, 0, 0, 0, time.UTC)}
	to := f.fb.MoveOutside(from, w)

	if err := f.sync.Reconciler().At(f.venue).Round(ctx); err != nil {
		t.Fatalf("round: %v", err)
	}
	b := f.get(t, broken.GetName())
	if b.GetFreebusyReservations()[0] == lost || !strings.Contains(b.GetNotes(), "[freebusy "+lost+"]") {
		t.Errorf("broken link: reservations %v, notes %q; want booked again and flagged", b.GetFreebusyReservations(), b.GetNotes())
	}
	m := f.get(t, moved.GetName())
	if m.GetFreebusyReservations()[0] != to || !strings.Contains(m.GetNotes(), "moved") {
		t.Errorf("moved: reservations %v, notes %q; want %s and a flag", m.GetFreebusyReservations(), m.GetNotes(), to)
	}

	// A second round flags nothing twice.
	if err := f.sync.Reconciler().At(f.venue).Round(ctx); err != nil {
		t.Fatalf("second round: %v", err)
	}
	again := f.get(t, broken.GetName())
	if strings.Count(again.GetNotes(), "[freebusy "+lost+"]") != 1 {
		t.Errorf("notes after a second round: %q; want the flag once", again.GetNotes())
	}

	// Cancelling the stay in aperio frees the replacement, not the old one.
	if _, err := f.stays.CancelReservation(ctx, &staypbv1.CancelReservationRequest{Name: m.GetName()}); err != nil {
		t.Fatalf("cancel the moved stay: %v", err)
	}
	if got, _ := f.fb.Lookup(ctx, []string{to}); got[to].State != engine.Ended {
		t.Errorf("the replacement %s is %v after cancel; want ended", to, got[to].State)
	}
}

func TestLiveReconcilerCancelsEventWhoseHallWent(t *testing.T) {
	f, ctx := setup(t), context.Background()
	e, err := f.events.CreateEvent(ctx, &eventpbv1.CreateEventRequest{Parent: f.venue, EventId: "doe-" + f.run, Event: &eventpbv1.Event{
		DisplayName: "John Doe's reception", Contact: f.contact, EventType: typepbv1.EventType_EVENT_TYPE_WEDDING, Guests: 100,
		Dates: &typepbv1.DateRange{StartDate: &date.Date{Year: 2027, Month: 12, Day: 20}, EndDate: &date.Date{Year: 2027, Month: 12, Day: 20}},
	}})
	if err != nil {
		t.Fatalf("create event: %v", err)
	}
	start := time.Date(2027, 12, 20, 13, 0, 0, 0, time.UTC)
	s, err := f.events.CreateSession(ctx, &eventpbv1.CreateSessionRequest{Parent: e.GetName(), Session: &eventpbv1.Session{
		DisplayName: "Reception", Space: f.hall, Guests: 100,
		StartTime: timestamppb.New(start), EndTime: timestamppb.New(start.Add(4 * time.Hour)),
	}})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	f.fb.CancelOutside(s.GetFreebusyReservation())
	if err := f.sync.Reconciler().At(f.venue).Round(ctx); err != nil {
		t.Fatalf("round: %v", err)
	}
	got, err := f.events.GetEvent(ctx, &eventpbv1.GetEventRequest{Name: e.GetName()})
	if err != nil || got.GetState() != eventpbv1.Event_STATE_CANCELLED {
		t.Errorf("event after its only hall was cancelled in freebusy: %v, %v; want CANCELLED", got.GetState(), err)
	}
}
