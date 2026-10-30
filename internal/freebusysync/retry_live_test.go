// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package freebusysync_test

// Live tests of John Doe's event written while another writer gets in
// first: accepting its halls, flagging one and moving its state each read
// it again and succeed; and a hold a failed session left live but off the
// event's list is still given back when the event is cancelled. Run as
// fixture_live_test.go says.

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/shokki-engineering/aperio/internal/booking"
	"github.com/shokki-engineering/aperio/internal/engine"
	"github.com/shokki-engineering/aperio/internal/service/event"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/event/v1/eventpbv1"
	"github.com/shokki-engineering/aperio/test/live"
)

func TestLiveEventWritesRetryConflicts(t *testing.T) {
	ctx := context.Background()
	// racing is f's event service, its next event write beaten by another.
	racing := func(f *fixture) *event.Server { return event.New(flaky(f.repos, 0, 1, nil), f.base) }
	t.Run("HoldHalls", func(t *testing.T) {
		f := setup(t)
		e, hold := f.heldEvent(t, racing(f))
		if got, _ := f.fb.Lookup(ctx, []string{hold}); got[hold].State != engine.Live || len(e.GetFreebusyHolds()) != 1 {
			t.Errorf("hold %s is %v, holds %v; want it live and listed", hold, got[hold].State, e.GetFreebusyHolds())
		}
	})
	t.Run("RelinkEvent", func(t *testing.T) {
		f := setup(t)
		e, hold := f.heldEvent(t, f.events)
		got, err := racing(f).RelinkEvent(ctx, e.GetName(), hold, "", "check the hall")
		if err != nil || !strings.Contains(got.GetNotes(), booking.Mark(hold)) {
			t.Errorf("relink past a conflict: notes %q, %v; want the flag", got.GetNotes(), err)
		}
	})
	t.Run("ConfirmEvent", func(t *testing.T) {
		f := setup(t)
		e, _ := f.heldEvent(t, f.events)
		got, err := racing(f).ConfirmEvent(ctx, &eventpbv1.ConfirmEventRequest{Name: e.GetName()})
		if err != nil || got.GetState() != eventpbv1.Event_STATE_CONFIRMED {
			t.Errorf("confirm past a conflict: %v, %v; want CONFIRMED", got.GetState(), err)
		}
	})
}

// stuck is freebusy refusing to cancel one reservation.
type stuck struct {
	*live.Freebusy
	reservation string
}

// Cancel refuses s.reservation, and cancels the rest.
func (s stuck) Cancel(ctx context.Context, reservation, reason string) error {
	if reservation == s.reservation {
		return errors.New("freebusy unavailable")
	}
	return s.Freebusy.Cancel(ctx, reservation, reason)
}

func TestLiveCancelEventGivesBackUnlistedHold(t *testing.T) {
	f, ctx := setup(t), context.Background()
	e, hold := f.heldEvent(t, f.events)
	// The session takes the hold off the list, cannot cancel it, and cannot
	// put it back: live in freebusy, and off the event's list.
	base := *f.base
	base.Engine = stuck{Freebusy: f.fb, reservation: hold}
	svc := event.New(flaky(f.repos, 1, -1, errors.New("database unavailable")), &base)
	if _, err := f.session(svc, e, "reception"); err == nil {
		t.Fatal("create session freebusy cannot replace the hold for: want an error")
	}
	got, err := f.events.GetEvent(ctx, &eventpbv1.GetEventRequest{Name: e.GetName()})
	if fb, _ := f.fb.Lookup(ctx, []string{hold}); err != nil || len(got.GetFreebusyHolds()) != 0 || fb[hold].State != engine.Live {
		t.Fatalf("hold %s is %v, holds %v, %v; want it live and unlisted", hold, fb[hold].State, got.GetFreebusyHolds(), err)
	}
	if _, err := f.events.CancelEvent(ctx, &eventpbv1.CancelEventRequest{Name: e.GetName()}); err != nil {
		t.Fatalf("cancel event: %v", err)
	}
	if fb, _ := f.fb.Lookup(ctx, []string{hold}); fb[hold].State != engine.Ended {
		t.Errorf("hold %s is %v after the event was cancelled; want it given back", hold, fb[hold].State)
	}
}
