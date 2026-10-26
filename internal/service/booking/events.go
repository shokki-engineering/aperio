// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package booking

// events.go is an event's life: TENTATIVE -> CONFIRMED, or CANCELLED with
// every session's space given back, and a delete that gives them back too
// (with force) or is refused while the event still has sessions.

import (
	"context"

	"github.com/shokki-engineering/aperio/internal/database/repository/repox"
	"github.com/shokki-engineering/aperio/internal/service/generated/servicex"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/booking/v1/bookingpbv1"
)

// BeforeDeleteEvent refuses to delete an event that has sessions unless
// force is set, and then gives back the sessions' spaces.
func (h *hooks) BeforeDeleteEvent(ctx context.Context, name string, force bool) error {
	booked, n, err := h.sessionsOf(ctx, name)
	if err != nil {
		return err
	}
	if n > 0 && !force {
		return servicex.Blocked("HAS_SESSIONS", eventType, name,
			"the event has sessions", "Delete its sessions first, or set force")
	}
	return h.release(ctx, booked, "aperio: the event was deleted")
}

// ConfirmEvent confirms a tentative event.
func (s *Server) ConfirmEvent(ctx context.Context, req *bookingpbv1.ConfirmEventRequest) (*bookingpbv1.Event, error) {
	return s.moveEvent(ctx, "ConfirmEvent", req.GetName(), req.GetEtag(),
		[]bookingpbv1.Event_State{bookingpbv1.Event_STATE_TENTATIVE},
		func(e *bookingpbv1.Event) ([]string, error) {
			e.State = bookingpbv1.Event_STATE_CONFIRMED
			return []string{"state"}, nil
		})
}

// CancelEvent cancels an event and gives back every session's space. The
// sessions are kept, as the record of what was planned.
func (s *Server) CancelEvent(ctx context.Context, req *bookingpbv1.CancelEventRequest) (*bookingpbv1.Event, error) {
	return s.moveEvent(ctx, "CancelEvent", req.GetName(), req.GetEtag(),
		[]bookingpbv1.Event_State{bookingpbv1.Event_STATE_TENTATIVE, bookingpbv1.Event_STATE_CONFIRMED},
		func(e *bookingpbv1.Event) ([]string, error) {
			booked, _, err := s.h.sessionsOf(ctx, e.GetName())
			if err != nil {
				return nil, err
			}
			reason := req.GetReason()
			if reason == "" {
				reason = "cancelled"
			}
			if err := s.h.release(context.WithoutCancel(ctx), booked, "aperio: "+reason); err != nil {
				return nil, err
			}
			e.State = bookingpbv1.Event_STATE_CANCELLED
			return []string{"state"}, nil
		})
}

// moveEvent runs one event state method, as moveReservation does for stays.
func (s *Server) moveEvent(ctx context.Context, method, name, etag string, from []bookingpbv1.Event_State,
	move func(*bookingpbv1.Event) ([]string, error)) (*bookingpbv1.Event, error) {
	var out *bookingpbv1.Event
	err := s.Options().Run(ctx, "BookingService", method, func(ctx context.Context) error {
		e, err := s.Repositories().Events.Get(ctx, name)
		if err != nil {
			return servicex.NotFoundAs(err, eventType, name)
		}
		if etag != "" {
			if err := servicex.CheckEtag(eventType, name, etag, e.GetEtag()); err != nil {
				return err
			}
		}
		if !inState(e.GetState(), from) {
			return wrongState(eventType, name, method, e.GetState().String())
		}
		paths, err := move(e)
		if err != nil {
			return err
		}
		out, err = s.Repositories().Events.UpdateOutputOnly(ctx, e, paths)
		return err
	})
	return out, err
}

// sessionsOf reads every session of event, page by page, returning the
// freebusy reservations they hold and how many sessions there are.
func (h *hooks) sessionsOf(ctx context.Context, event string) (booked []string, n int, err error) {
	in := repox.ListInput{PageSize: 100}
	for {
		page, next, err := h.repos.Sessions.List(ctx, event, in)
		if err != nil {
			return nil, 0, err
		}
		for _, s := range page {
			n++
			if s.GetFreebusyReservation() != "" {
				booked = append(booked, s.GetFreebusyReservation())
			}
		}
		if next == "" {
			return booked, n, nil
		}
		in.PageToken = next
	}
}
