// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package freebusysync

// halls.go settles the halls accepted events hold (Event.freebusy_holds)
// between their acceptance and their sessions. They are not rebooked: a
// hold that freebusy cancelled or no longer has may be a hall sold to
// someone else, so it is flagged on the event's notes, as other drift is,
// for a person to settle; one freebusy moved is followed (the event names
// the replacement, so cancelling it frees the right one) and flagged.

import (
	"context"
	"errors"
	"log/slog"

	"github.com/shokki-engineering/aperio/internal/engine"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/event/v1/eventpbv1"
)

// settleHalls checks the hall holds of the events not cancelled, in one
// Lookup.
func (s *Syncer) settleHalls(ctx context.Context, events []*eventpbv1.Event) error {
	var names []string
	for _, e := range events {
		if e.GetState() != eventpbv1.Event_STATE_CANCELLED {
			names = append(names, e.GetFreebusyHolds()...)
		}
	}
	if len(names) == 0 {
		return nil
	}
	got, err := s.engine.Lookup(ctx, names)
	if err != nil {
		return err
	}
	if err := suspect(names, got); err != nil {
		return err
	}
	var errs []error
	for _, e := range events {
		if e.GetState() == eventpbv1.Event_STATE_CANCELLED {
			continue
		}
		for _, hold := range e.GetFreebusyHolds() {
			fb, ok := got[hold]
			switch {
			case !ok:
				errs = append(errs, s.flagEvent(ctx, e, hold, "",
					"freebusy has no reservation for a hall this event holds: check the hall is still the event's"))
			case fb.State == engine.Ended:
				errs = append(errs, s.flagEvent(ctx, e, hold, "",
					"freebusy cancelled a hall this event holds: book it again, or settle the event"))
			case fb.State == engine.Moved:
				errs = append(errs, s.flagEvent(ctx, e, hold, fb.Replacement,
					"freebusy moved a hall this event holds to "+fb.Replacement+"; the event's dates were not changed: check them"))
			}
		}
	}
	return errors.Join(errs...)
}

// flagEvent relinks and flags event e about its hall hold reservation.
func (s *Syncer) flagEvent(ctx context.Context, e *eventpbv1.Event, reservation, to, note string) error {
	slog.WarnContext(ctx, "freebusysync: event's hall hold out of step with freebusy", "event", e.GetName(), "reservation", reservation, "note", note)
	_, err := s.events.RelinkEvent(ctx, e.GetName(), reservation, to, note)
	return err
}
