// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package event

// sessions.go is the hooks around an event's sessions: each takes its space
// in freebusy for its window (setup to teardown) -- checked before create,
// booked after, locked against edits, and given back on delete. The first
// session in a hall the event kept since its quote was accepted
// (booking.HoldHall) replaces that hold with its own booking.

import (
	"context"
	"errors"

	"github.com/shokki-engineering/aperio/internal/booking"
	"github.com/shokki-engineering/aperio/internal/engine"
	"github.com/shokki-engineering/aperio/internal/service/generated/servicex"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/event/v1/eventpbv1"
	"google.golang.org/protobuf/proto"
)

// BeforeCreateSession checks that the session's space is free for its
// window. It takes nothing: validate_only runs it too.
func (h *hooks) BeforeCreateSession(ctx context.Context, parent string, m *eventpbv1.Session) error {
	w, err := sessionWindow(m)
	if err != nil {
		return err
	}
	res, err := h.Resources(ctx, servicex.ParentOf(parent), "session.space", m.GetSpace())
	if err != nil {
		return err
	}
	// The hall the event has kept since its quote was accepted is its own:
	// a session within it replaces that hold (AfterCreateSession).
	held, kept, err := h.HeldHall(ctx, parent, m.GetSpace())
	if err != nil {
		return err
	}
	if held != "" && !w.Start.Before(kept.Start) && !w.End.After(kept.End) {
		return nil
	}
	free, err := h.Engine.Free(ctx, res, w, 1)
	if err != nil {
		return err
	}
	if !free {
		return booking.Unavailable(m.GetSpace(), 1)
	}
	return nil
}

// AfterCreateSession books the stored session's space in freebusy and
// records the freebusy reservation on it; should that fail, the session is
// removed and anything booked given back.
func (h *hooks) AfterCreateSession(ctx context.Context, out *eventpbv1.Session) error {
	undo := func(booked string, cause error) error {
		errs := []error{cause}
		if booked != "" {
			errs = append(errs, h.Engine.Cancel(ctx, booked, "aperio: the session could not be completed"))
		}
		errs = append(errs, h.repos.Sessions.Delete(ctx, out.GetName()))
		return errors.Join(errs...)
	}
	w, err := sessionWindow(out)
	if err != nil {
		return undo("", err)
	}
	event := servicex.ParentOf(out.GetName())
	res, err := h.Resources(ctx, servicex.ParentOf(event), "session.space", out.GetSpace())
	if err != nil {
		return undo("", err)
	}
	if held, _, err := h.HeldHall(ctx, event, out.GetSpace()); err != nil || held != "" {
		// The session's own booking replaces the event's hold on the hall.
		if err == nil {
			err = h.Engine.Cancel(ctx, held, "aperio: replaced by the session "+out.GetName())
		}
		if err != nil {
			return undo("", err)
		}
	}
	booked, err := h.Engine.Book(ctx, engine.Booking{
		Key:       out.GetName(),
		Resources: res,
		Window:    w,
		Quantity:  1,
		Attributes: map[string]any{
			"aperio_session": out.GetName(),
			"aperio_event":   event,
			"guests":         out.GetGuests(),
		},
	})
	if errors.Is(err, engine.ErrUnavailable) {
		err = booking.Unavailable(out.GetSpace(), 1)
	}
	if err != nil {
		return undo("", err)
	}
	out.FreebusyReservation = booked
	updated, err := h.repos.Sessions.UpdateOutputOnly(ctx, out, []string{"freebusy_reservation"})
	if err != nil {
		return undo(booked, err)
	}
	proto.Reset(out)
	proto.Merge(out, updated)
	h.noteSession(ctx, out)
	return nil
}

// BeforeUpdateSession refuses to move a session's space or times: they are
// booked in freebusy, and moving them is a new session.
func (h *hooks) BeforeUpdateSession(ctx context.Context, m *eventpbv1.Session, paths []string) error {
	locked := []string{"space", "start_time", "end_time", "setup_time", "teardown_time"}
	if !servicex.TouchesAny(paths, locked) {
		return nil
	}
	existing, err := h.repos.Sessions.Get(ctx, m.GetName())
	if err != nil {
		return servicex.NotFoundAs(err, sessionType, m.GetName())
	}
	if existing.GetSpace() == m.GetSpace() &&
		proto.Equal(existing.GetStartTime(), m.GetStartTime()) &&
		proto.Equal(existing.GetEndTime(), m.GetEndTime()) &&
		proto.Equal(existing.GetSetupTime(), m.GetSetupTime()) &&
		proto.Equal(existing.GetTeardownTime(), m.GetTeardownTime()) {
		return nil
	}
	return servicex.Blocked("BOOKING_LOCKED", sessionType, m.GetName(),
		"a session's space and times are booked in freebusy and cannot be edited",
		"Delete the session and create a new one")
}

// BeforeDeleteSession gives back the session's space.
func (h *hooks) BeforeDeleteSession(ctx context.Context, name string, force bool) error {
	s, err := h.repos.Sessions.Get(ctx, name)
	if err != nil {
		return servicex.NotFoundAs(err, sessionType, name)
	}
	if s.GetFreebusyReservation() == "" {
		return nil
	}
	return h.Release(ctx, []string{s.GetFreebusyReservation()}, "aperio: the session was deleted")
}
