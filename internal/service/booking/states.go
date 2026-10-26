// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package booking

// states.go is the custom methods that move a stay through its states:
// PENDING -> CONFIRMED -> CHECKED_IN -> CHECKED_OUT, or to CANCELLED from
// PENDING or CONFIRMED, giving its rooms back to freebusy.

import (
	"context"

	"github.com/shokki-engineering/aperio/internal/service/generated/servicex"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/booking/v1/bookingpbv1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// ConfirmReservation confirms a pending reservation: its rooms stay taken.
func (s *Server) ConfirmReservation(ctx context.Context, req *bookingpbv1.ConfirmReservationRequest) (*bookingpbv1.Reservation, error) {
	return s.moveReservation(ctx, "ConfirmReservation", req.GetName(), req.GetEtag(),
		[]bookingpbv1.Reservation_State{bookingpbv1.Reservation_STATE_PENDING},
		func(r *bookingpbv1.Reservation) ([]string, error) {
			r.State = bookingpbv1.Reservation_STATE_CONFIRMED
			return []string{"state"}, nil
		})
}

// CancelReservation cancels a reservation and gives back its rooms.
func (s *Server) CancelReservation(ctx context.Context, req *bookingpbv1.CancelReservationRequest) (*bookingpbv1.Reservation, error) {
	return s.moveReservation(ctx, "CancelReservation", req.GetName(), req.GetEtag(),
		[]bookingpbv1.Reservation_State{bookingpbv1.Reservation_STATE_PENDING, bookingpbv1.Reservation_STATE_CONFIRMED},
		func(r *bookingpbv1.Reservation) ([]string, error) {
			reason := req.GetReason()
			if reason == "" {
				reason = "cancelled"
			}
			if err := s.h.release(context.WithoutCancel(ctx), r.GetFreebusyReservations(), "aperio: "+reason); err != nil {
				return nil, err
			}
			r.State = bookingpbv1.Reservation_STATE_CANCELLED
			r.CancelReason = req.GetReason()
			r.CancelTime = timestamppb.New(s.h.now())
			return []string{"state", "cancel_reason", "cancel_time"}, nil
		})
}

// ArriveReservation records the guests' arrival.
func (s *Server) ArriveReservation(ctx context.Context, req *bookingpbv1.ArriveReservationRequest) (*bookingpbv1.Reservation, error) {
	return s.moveReservation(ctx, "ArriveReservation", req.GetName(), req.GetEtag(),
		[]bookingpbv1.Reservation_State{bookingpbv1.Reservation_STATE_CONFIRMED},
		func(r *bookingpbv1.Reservation) ([]string, error) {
			r.State = bookingpbv1.Reservation_STATE_CHECKED_IN
			r.ArriveTime = timestamppb.New(s.h.now())
			return []string{"state", "arrive_time"}, nil
		})
}

// DepartReservation records the guests' departure.
func (s *Server) DepartReservation(ctx context.Context, req *bookingpbv1.DepartReservationRequest) (*bookingpbv1.Reservation, error) {
	return s.moveReservation(ctx, "DepartReservation", req.GetName(), req.GetEtag(),
		[]bookingpbv1.Reservation_State{bookingpbv1.Reservation_STATE_CHECKED_IN},
		func(r *bookingpbv1.Reservation) ([]string, error) {
			r.State = bookingpbv1.Reservation_STATE_CHECKED_OUT
			r.DepartTime = timestamppb.New(s.h.now())
			return []string{"state", "depart_time"}, nil
		})
}

// moveReservation runs one state method: it reads the reservation, checks the
// caller's etag and that it is in one of from, applies move, and writes the
// fields move names.
func (s *Server) moveReservation(ctx context.Context, method, name, etag string, from []bookingpbv1.Reservation_State,
	move func(*bookingpbv1.Reservation) ([]string, error)) (*bookingpbv1.Reservation, error) {
	var out *bookingpbv1.Reservation
	err := s.Options().Run(ctx, "BookingService", method, func(ctx context.Context) error {
		r, err := s.Repositories().Reservations.Get(ctx, name)
		if err != nil {
			return servicex.NotFoundAs(err, reservationType, name)
		}
		if etag != "" {
			if err := servicex.CheckEtag(reservationType, name, etag, r.GetEtag()); err != nil {
				return err
			}
		}
		if !inState(r.GetState(), from) {
			return wrongState(reservationType, name, method, r.GetState().String())
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

// wrongState is the error for a state method called in a state it does not
// move from.
func wrongState(resourceType, name, method, state string) error {
	return servicex.Blocked("INVALID_STATE", resourceType, name,
		method+" cannot be called on one that is "+state, "Read it again and call the method its state allows")
}

// inState reports whether s is one of states.
func inState[S comparable](s S, states []S) bool {
	for _, x := range states {
		if s == x {
			return true
		}
	}
	return false
}
