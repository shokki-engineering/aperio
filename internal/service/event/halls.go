// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package event

// halls.go is the record of the halls an accepted event holds
// (Event.freebusy_holds): HoldHalls takes them for the event's days when its
// quote is accepted (booking.HoldHall) and records each freebusy
// reservation; the first session in a hall replaces its hold, taking it
// off the list before cancelling it (replaceHold, sessions.go); CancelEvent
// gives back what is on it, and the reconciler checks it against freebusy.
// The list is output only, so it is written here (editEvent), never through
// UpdateEvent.

import (
	"context"
	"errors"
	"log/slog"
	"slices"

	"github.com/shokki-engineering/aperio/internal/booking"
	"github.com/shokki-engineering/aperio/internal/service/generated/servicex"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/event/v1/eventpbv1"
)

// HoldHalls holds halls for event name's days and records them on its
// freebusy_holds. It is idempotent: a hall held already (a retried accept)
// is held under the same reservation, recorded once. A hall that is not
// free fails it -- after recording the halls held before it, so that
// cancelling the event gives them back. The record is written through
// editEvent, so a session or webhook writing the event at the same moment
// does not fail the accept and give back what it took. It is a custom
// method, not an RPC: AcceptQuote calls it.
func (s *Server) HoldHalls(ctx context.Context, name string, halls []booking.Hall) (*eventpbv1.Event, error) {
	var out *eventpbv1.Event
	err := s.Options().Run(ctx, "EventService", "HoldHalls", func(ctx context.Context) error {
		e, err := s.Repositories().Events.Get(ctx, name)
		if err != nil {
			return servicex.NotFoundAs(err, eventType, name)
		}
		if e.GetState() == eventpbv1.Event_STATE_CANCELLED {
			return booking.WrongState(eventType, name, "HoldHalls", e.GetState().String())
		}
		out = e
		var held []string // what this call holds, recorded or not
		var holdErr error
		for _, h := range halls {
			r, err := s.h.HoldHall(ctx, servicex.ParentOf(name), name, h.Field, h.Space, e.GetDates())
			if err != nil {
				holdErr = err
				break
			}
			if !slices.Contains(held, r) {
				held = append(held, r)
			}
		}
		// taken is what this call held and the event did not list: as first
		// read, until editEvent reads it again.
		taken := slices.DeleteFunc(slices.Clone(held), func(r string) bool { return slices.Contains(e.GetFreebusyHolds(), r) })
		got, err := s.h.editEvent(ctx, name, func(e *eventpbv1.Event) ([]string, []string, error) {
			taken = slices.DeleteFunc(slices.Clone(held), func(r string) bool { return slices.Contains(e.GetFreebusyHolds(), r) })
			if e.GetState() == eventpbv1.Event_STATE_CANCELLED {
				return nil, nil, booking.WrongState(eventType, name, "HoldHalls", e.GetState().String())
			}
			if len(taken) == 0 {
				return nil, nil, nil
			}
			e.FreebusyHolds = append(e.FreebusyHolds, taken...)
			return nil, []string{"freebusy_holds"}, nil
		})
		if err != nil {
			// Unrecorded, nothing would give them back: give them back now.
			release := s.h.ReleaseHalls(context.WithoutCancel(ctx), taken, "aperio: the event's halls could not be recorded")
			return errors.Join(holdErr, err, release)
		}
		out = got
		return holdErr
	})
	return out, err
}

// replaceHold gives back hold, the event's hold on a hall a session is
// about to book, recording the replacement first: it takes hold off
// event's freebusy_holds, and only then cancels it in freebusy. freebusy
// sends a webhook for every cancel, and the Syncer flags a listed hold it
// finds cancelled as freebusy cancelling a hall the event still keeps; were
// the hold cancelled first, that webhook (or a reconciler round) could land
// before the list changed and flag it falsely, and a failed forget would
// leave it flagged for good. Done in this order, a hold freebusy reports
// cancelled while still listed is one aperio never gave back. The
// freebusy_holds list cannot say which space a hold is for, so the Syncer
// could not tell a replaced hold from a lost one after the fact: the order
// is what keeps it right. A forget that fails fails the session, with the
// hold kept and listed; a cancel that fails puts the hold back on the list.
// It reports whether hold was on the list, for restoreHold should the
// session fail after it.
func (h *hooks) replaceHold(ctx context.Context, event, hold, session string) (bool, error) {
	if hold == "" {
		return false, nil
	}
	forgot, err := h.editHolds(ctx, event, func(holds []string) ([]string, bool) {
		if !slices.Contains(holds, hold) {
			return holds, false
		}
		return slices.DeleteFunc(holds, func(r string) bool { return r == hold }), true
	})
	if err != nil {
		return false, err
	}
	if err := h.Engine.Cancel(ctx, hold, "aperio: replaced by the session "+session); err != nil {
		return false, errors.Join(err, h.restoreHold(ctx, event, hold, forgot))
	}
	return forgot, nil
}

// restoreHold puts hold back on event's freebusy_holds when replaceHold took
// it off (forgot) and the session that replaced it failed: the event keeps
// the record, and a hold given back for nothing is flagged by the Syncer
// for a person, as before. It retries a conflict (editEvent); should it
// still fail, the hold may be live in freebusy but unlisted, which is logged
// and is no leak: the next session in the hall still finds it by its key
// (HeldHall), and CancelEvent gives back the holds found by their keys too
// (booking.SweepHalls).
func (h *hooks) restoreHold(ctx context.Context, event, hold string, forgot bool) error {
	if !forgot {
		return nil
	}
	_, err := h.editHolds(context.WithoutCancel(ctx), event, func(holds []string) ([]string, bool) {
		return append(holds, hold), !slices.Contains(holds, hold)
	})
	if err != nil {
		slog.ErrorContext(ctx, "event: replaced hall hold not put back on the event", "event", event, "hold", hold, "err", err)
	}
	return err
}

// editHolds rewrites event's freebusy_holds with edit, which reports
// whether it changed them, through editEvent: two sessions replacing two
// halls at once cannot write back each other's hold.
func (h *hooks) editHolds(ctx context.Context, event string, edit func([]string) ([]string, bool)) (bool, error) {
	changed := false
	_, err := h.editEvent(ctx, event, func(e *eventpbv1.Event) ([]string, []string, error) {
		var holds []string
		if holds, changed = edit(slices.Clone(e.GetFreebusyHolds())); !changed {
			return nil, nil, nil
		}
		e.FreebusyHolds = holds
		return nil, []string{"freebusy_holds"}, nil
	})
	return changed && err == nil, err
}
