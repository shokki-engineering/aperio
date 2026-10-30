// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package event

// freebusy.go is how a session follows what freebusy did to its hall
// without aperio asking, for internal/freebusysync: following a moved
// reservation, booking a lost one again, and flagging what only a person
// should settle -- for a session's hall, and for a hall an accepted event
// holds (RelinkEvent). An event whose every session lost its hall is
// cancelled with CancelEvent; these are the rest. They are custom methods,
// not RPCs.

import (
	"context"
	"slices"

	"github.com/shokki-engineering/aperio/internal/booking"
	"github.com/shokki-engineering/aperio/internal/engine"
	"github.com/shokki-engineering/aperio/internal/service/generated/servicex"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/event/v1/eventpbv1"
)

// RelinkSession records on session name what freebusy did to its
// reservation from: to, when set, becomes its reservation; note, when set,
// is flagged on its notes once for from.
func (s *Server) RelinkSession(ctx context.Context, name, from, to, note string) (*eventpbv1.Session, error) {
	var out *eventpbv1.Session
	err := s.Options().Run(ctx, "EventService", "RelinkSession", func(ctx context.Context) error {
		m, err := s.Repositories().Sessions.Get(ctx, name)
		if err != nil {
			return servicex.NotFoundAs(err, sessionType, name)
		}
		out = m
		moved := to != "" && m.GetFreebusyReservation() == from
		if moved {
			m.FreebusyReservation = to
		}
		flagged := false
		if note != "" {
			m.Notes, flagged = booking.Flag(m.GetNotes(), booking.Mark(from), note)
		}
		if !moved && !flagged {
			return nil
		}
		out, err = s.Repositories().Sessions.UpdateWithOutputOnly(ctx, m, []string{"notes"}, []string{"freebusy_reservation"})
		return err
	})
	return out, err
}

// RelinkEvent records on event name what freebusy did to from, one of its
// hall holds (freebusy_holds): to, when set, replaces it on the list; note,
// when set, is flagged on the event's notes once for from.
func (s *Server) RelinkEvent(ctx context.Context, name, from, to, note string) (*eventpbv1.Event, error) {
	var out *eventpbv1.Event
	err := s.Options().Run(ctx, "EventService", "RelinkEvent", func(ctx context.Context) error {
		// editEvent: a session replacing a hold at the same moment does not
		// fail the flag, nor is the hold it took off written back.
		var err error
		out, err = s.h.editEvent(ctx, name, func(e *eventpbv1.Event) ([]string, []string, error) {
			i := slices.Index(e.GetFreebusyHolds(), from)
			moved := to != "" && i >= 0
			if moved {
				e.FreebusyHolds[i] = to
			}
			flagged := false
			if note != "" {
				e.Notes, flagged = booking.Flag(e.GetNotes(), booking.Mark(from), note)
			}
			if !moved && !flagged {
				return nil, nil, nil
			}
			return []string{"notes"}, []string{"freebusy_holds"}, nil
		})
		return err
	})
	return out, err
}

// RebookSession books session name's hall again, under a key of its own,
// when freebusy no longer has lost, its reservation; engine.ErrUnavailable
// when the hall has gone to someone else.
func (s *Server) RebookSession(ctx context.Context, name, lost string) (*eventpbv1.Session, error) {
	m, err := s.Repositories().Sessions.Get(ctx, name)
	if err != nil {
		return nil, servicex.NotFoundAs(err, sessionType, name)
	}
	w, err := sessionWindow(m)
	if err != nil {
		return nil, err
	}
	event := servicex.ParentOf(name)
	res, err := s.h.Resources(ctx, servicex.ParentOf(event), "session.space", m.GetSpace())
	if err != nil {
		return nil, err
	}
	booked, err := s.h.Engine.Book(ctx, engine.Booking{
		Key: name + "#rebook/" + lost, Resources: res, Window: w, Quantity: 1,
		Attributes: map[string]any{"aperio_session": name, "aperio_event": event, "guests": m.GetGuests()},
	})
	if err != nil {
		return nil, err
	}
	out, err := s.RelinkSession(ctx, name, lost, booked,
		"freebusy had lost this session's hall ("+lost+"); it was booked again as "+booked)
	if err == nil {
		s.h.noteSession(ctx, out)
	}
	return out, err
}

// SessionWindow is the window session m takes its hall for in freebusy.
func (s *Server) SessionWindow(m *eventpbv1.Session) (engine.Window, error) { return sessionWindow(m) }
