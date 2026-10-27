// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package freebusysync

// sessions.go settles sessions, event by event: an event is cancelled only
// when every session's hall is gone, so each event's sessions are read
// whole, and all their freebusy reservations in one Lookup.

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/shokki-engineering/aperio/internal/engine"
	"github.com/shokki-engineering/aperio/internal/service/generated/servicex"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/event/v1/eventpbv1"
)

// settleSessions settles the events of sessions that are not cancelled.
func (s *Syncer) settleSessions(ctx context.Context, sessions []*eventpbv1.Session) error {
	byEvent := map[string][]*eventpbv1.Session{}
	var order, names []string
	for _, m := range sessions {
		event := servicex.ParentOf(m.GetName())
		if _, seen := byEvent[event]; seen {
			continue
		}
		e, err := s.events.GetEvent(ctx, &eventpbv1.GetEventRequest{Name: event})
		if err != nil {
			return err
		}
		if e.GetState() == eventpbv1.Event_STATE_CANCELLED {
			byEvent[event] = nil
			continue
		}
		all, err := s.sessionsOf(ctx, event)
		if err != nil {
			return err
		}
		byEvent[event] = all
		order = append(order, event)
		for _, m := range all {
			if m.GetFreebusyReservation() != "" {
				names = append(names, m.GetFreebusyReservation())
			}
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
	for _, event := range order {
		errs = append(errs, s.settleEvent(ctx, event, byEvent[event], got))
	}
	return errors.Join(errs...)
}

// settleEvent brings event's sessions in step with got.
func (s *Syncer) settleEvent(ctx context.Context, event string, sessions []*eventpbv1.Session, got map[string]engine.Reservation) error {
	var ended []*eventpbv1.Session
	var errs []error
	linked := 0
	for _, m := range sessions {
		name := m.GetFreebusyReservation()
		if name == "" {
			continue
		}
		linked++
		fb, ok := got[name]
		switch {
		case !ok:
			if _, err := s.events.RebookSession(ctx, m.GetName(), name); err != nil {
				errs = append(errs, s.flagSession(ctx, m, name, "", "freebusy has no reservation for this hall, and it could not be booked again: settle it"))
			}
		case fb.State == engine.Ended:
			ended = append(ended, m)
		case fb.State == engine.Moved:
			errs = append(errs, s.flagSession(ctx, m, name, fb.Replacement, fmt.Sprintf(
				"freebusy moved this hall to %s; the session's times in aperio were not changed: check them", fb.Replacement)))
		default:
			if w, err := s.events.SessionWindow(m); err == nil && (!w.Start.Equal(fb.Window.Start) || !w.End.Equal(fb.Window.End)) {
				errs = append(errs, s.flagSession(ctx, m, name, "", "freebusy holds this hall for "+span(fb.Window)+", not the session's times: check it"))
			}
		}
	}
	if len(ended) == 0 {
		return errors.Join(errs...)
	}
	if len(ended) == linked {
		_, err := s.events.CancelEvent(ctx, &eventpbv1.CancelEventRequest{Name: event, Reason: "cancelled in freebusy"})
		slog.InfoContext(ctx, "freebusysync: event cancelled in freebusy", "event", event, "err", err)
		return errors.Join(append(errs, err)...)
	}
	for _, m := range ended {
		errs = append(errs, s.flagSession(ctx, m, m.GetFreebusyReservation(), "",
			"freebusy cancelled this session's hall; the event keeps its other sessions: book the hall again or delete the session"))
	}
	return errors.Join(errs...)
}

// flagSession relinks and flags session m about reservation.
func (s *Syncer) flagSession(ctx context.Context, m *eventpbv1.Session, reservation, to, note string) error {
	slog.WarnContext(ctx, "freebusysync: session out of step with freebusy", "session", m.GetName(), "reservation", reservation, "note", note)
	_, err := s.events.RelinkSession(ctx, m.GetName(), reservation, to, note)
	return err
}

// sessionsOf reads every session of event.
func (s *Syncer) sessionsOf(ctx context.Context, event string) ([]*eventpbv1.Session, error) {
	var out []*eventpbv1.Session
	req := &eventpbv1.ListSessionsRequest{Parent: event, PageSize: 100}
	for {
		page, err := s.events.ListSessions(ctx, req)
		if err != nil {
			return nil, err
		}
		out = append(out, page.GetSessions()...)
		if page.GetNextPageToken() == "" {
			return out, nil
		}
		req.PageToken = page.GetNextPageToken()
	}
}
