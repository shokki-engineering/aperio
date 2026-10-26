// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package event

// events.go is an event's life: TENTATIVE -> CONFIRMED, or CANCELLED with
// every session's space given back. An event is cancelled, never deleted.

import (
	"context"

	"github.com/shokki-engineering/aperio/internal/booking"
	"github.com/shokki-engineering/aperio/internal/database/repository/repox"
	"github.com/shokki-engineering/aperio/internal/service/generated/servicex"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/event/v1/eventpbv1"
)

// ConfirmEvent confirms a tentative event.
func (s *Server) ConfirmEvent(ctx context.Context, req *eventpbv1.ConfirmEventRequest) (*eventpbv1.Event, error) {
	return s.moveEvent(ctx, "ConfirmEvent", req.GetName(), req.GetEtag(),
		[]eventpbv1.Event_State{eventpbv1.Event_STATE_TENTATIVE},
		func(e *eventpbv1.Event) ([]string, error) {
			e.State = eventpbv1.Event_STATE_CONFIRMED
			return []string{"state"}, nil
		})
}

// CancelEvent cancels an event and gives back every session's space. The
// sessions are kept, as the record of what was planned.
func (s *Server) CancelEvent(ctx context.Context, req *eventpbv1.CancelEventRequest) (*eventpbv1.Event, error) {
	return s.moveEvent(ctx, "CancelEvent", req.GetName(), req.GetEtag(),
		[]eventpbv1.Event_State{eventpbv1.Event_STATE_TENTATIVE, eventpbv1.Event_STATE_CONFIRMED},
		func(e *eventpbv1.Event) ([]string, error) {
			booked, _, err := s.h.sessionsOf(ctx, e.GetName())
			if err != nil {
				return nil, err
			}
			reason := req.GetReason()
			if reason == "" {
				reason = "cancelled"
			}
			if err := s.h.Release(context.WithoutCancel(ctx), booked, "aperio: "+reason); err != nil {
				return nil, err
			}
			e.State = eventpbv1.Event_STATE_CANCELLED
			return []string{"state"}, nil
		})
}

// moveEvent runs one event state method, as moveReservation does for stays.
func (s *Server) moveEvent(ctx context.Context, method, name, etag string, from []eventpbv1.Event_State,
	move func(*eventpbv1.Event) ([]string, error)) (*eventpbv1.Event, error) {
	var out *eventpbv1.Event
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
		if !booking.InState(e.GetState(), from) {
			return booking.WrongState(eventType, name, method, e.GetState().String())
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
