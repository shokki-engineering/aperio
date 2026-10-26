// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package stay

// states.go is the custom methods that move a stay through its states:
// PENDING -> CONFIRMED -> CHECKED_IN -> CHECKED_OUT, or to CANCELLED from
// PENDING or CONFIRMED, giving its rooms back to freebusy.

import (
	"context"

	"github.com/shokki-engineering/aperio/internal/booking"
	"github.com/shokki-engineering/aperio/internal/service/generated/servicex"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/stay/v1/staypbv1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// ConfirmReservation confirms a pending reservation: its rooms stay taken.
func (s *Server) ConfirmReservation(ctx context.Context, req *staypbv1.ConfirmReservationRequest) (*staypbv1.Reservation, error) {
	return s.moveReservation(ctx, "ConfirmReservation", req.GetName(), req.GetEtag(),
		[]staypbv1.Reservation_State{staypbv1.Reservation_STATE_PENDING},
		func(r *staypbv1.Reservation) ([]string, error) {
			r.State = staypbv1.Reservation_STATE_CONFIRMED
			return []string{"state"}, nil
		})
}

// CancelReservation cancels a reservation and gives back its rooms.
func (s *Server) CancelReservation(ctx context.Context, req *staypbv1.CancelReservationRequest) (*staypbv1.Reservation, error) {
	return s.moveReservation(ctx, "CancelReservation", req.GetName(), req.GetEtag(),
		[]staypbv1.Reservation_State{staypbv1.Reservation_STATE_PENDING, staypbv1.Reservation_STATE_CONFIRMED},
		func(r *staypbv1.Reservation) ([]string, error) {
			reason := req.GetReason()
			if reason == "" {
				reason = "cancelled"
			}
			if err := s.h.Release(context.WithoutCancel(ctx), r.GetFreebusyReservations(), "aperio: "+reason); err != nil {
				return nil, err
			}
			r.State = staypbv1.Reservation_STATE_CANCELLED
			r.CancelReason = req.GetReason()
			r.CancelTime = timestamppb.New(s.h.Now())
			return []string{"state", "cancel_reason", "cancel_time"}, nil
		})
}

// ArriveReservation records the guests' arrival.
func (s *Server) ArriveReservation(ctx context.Context, req *staypbv1.ArriveReservationRequest) (*staypbv1.Reservation, error) {
	return s.moveReservation(ctx, "ArriveReservation", req.GetName(), req.GetEtag(),
		[]staypbv1.Reservation_State{staypbv1.Reservation_STATE_CONFIRMED},
		func(r *staypbv1.Reservation) ([]string, error) {
			r.State = staypbv1.Reservation_STATE_CHECKED_IN
			r.ArriveTime = timestamppb.New(s.h.Now())
			return []string{"state", "arrive_time"}, nil
		})
}

// DepartReservation records the guests' departure.
func (s *Server) DepartReservation(ctx context.Context, req *staypbv1.DepartReservationRequest) (*staypbv1.Reservation, error) {
	return s.moveReservation(ctx, "DepartReservation", req.GetName(), req.GetEtag(),
		[]staypbv1.Reservation_State{staypbv1.Reservation_STATE_CHECKED_IN},
		func(r *staypbv1.Reservation) ([]string, error) {
			r.State = staypbv1.Reservation_STATE_CHECKED_OUT
			r.DepartTime = timestamppb.New(s.h.Now())
			return []string{"state", "depart_time"}, nil
		})
}

// moveReservation runs one state method: it reads the reservation, checks the
// caller's etag and that it is in one of from, applies move, and writes the
// fields move names.
func (s *Server) moveReservation(ctx context.Context, method, name, etag string, from []staypbv1.Reservation_State,
	move func(*staypbv1.Reservation) ([]string, error)) (*staypbv1.Reservation, error) {
	var out *staypbv1.Reservation
	err := s.Options().Run(ctx, "StayService", method, func(ctx context.Context) error {
		r, err := s.Repositories().Reservations.Get(ctx, name)
		if err != nil {
			return servicex.NotFoundAs(err, reservationType, name)
		}
		if etag != "" {
			if err := servicex.CheckEtag(reservationType, name, etag, r.GetEtag()); err != nil {
				return err
			}
		}
		if !booking.InState(r.GetState(), from) {
			return booking.WrongState(reservationType, name, method, r.GetState().String())
		}
		paths, err := move(r)
		if err != nil {
			return err
		}
		out, err = s.Repositories().Reservations.UpdateOutputOnly(ctx, r, paths)
		return err
	})
	return out, err
}
