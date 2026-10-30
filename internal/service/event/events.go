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
	"google.golang.org/protobuf/types/known/timestamppb"
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

// CancelEvent cancels an event and gives back every session's space, and
// the halls it kept since its quote was accepted (its freebusy_holds, and
// any still held under their keys though off the list). The
// sessions are kept, as the record of what was planned, and so is the list
// of holds; the reason and the time are recorded.
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
			if err := s.h.ReleaseHalls(context.WithoutCancel(ctx), e.GetFreebusyHolds(), "aperio: "+reason); err != nil {
				return nil, err
			}
			// And any the list lost track of (booking.SweepHalls).
			if err := s.h.SweepHalls(context.WithoutCancel(ctx), e.GetName(), "aperio: "+reason); err != nil {
				return nil, err
			}
			e.State, e.CancelReason, e.CancelTime = eventpbv1.Event_STATE_CANCELLED, req.GetReason(), timestamppb.New(s.h.Now())
			return []string{"state", "cancel_reason", "cancel_time"}, nil
		})
}

// moveEvent runs one event state method, as moveReservation does for stays.
// The write goes through editEvent: another writer getting in between (a
// session replacing a hall, a webhook flagging one) has move run again on
// the event as it now stands, so move must be safe to repeat -- giving back
// what is given back already is. A client's etag is checked each time
// against the event as read, so one that went stale fails, as it should.
func (s *Server) moveEvent(ctx context.Context, method, name, etag string, from []eventpbv1.Event_State,
	move func(*eventpbv1.Event) ([]string, error)) (*eventpbv1.Event, error) {
	var out *eventpbv1.Event
	err := s.Options().Run(ctx, "EventService", method, func(ctx context.Context) error {
		var err error
		out, err = s.h.editEvent(ctx, name, func(e *eventpbv1.Event) ([]string, []string, error) {
			if etag != "" {
				if err := servicex.CheckEtag(eventType, name, etag, e.GetEtag()); err != nil {
					return nil, nil, err
				}
			}
			if !booking.InState(e.GetState(), from) {
				return nil, nil, booking.WrongState(eventType, name, method, e.GetState().String())
			}
			paths, err := move(e)
			return nil, paths, err
		})
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
