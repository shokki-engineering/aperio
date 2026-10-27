// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package stay

// freebusy.go is how a stay follows what freebusy did to its rooms without
// aperio asking -- an operator moving a reservation, a link found broken --
// for internal/freebusysync, which finds the change (a webhook, the
// reconciler). A cancellation is CancelReservation's; these are the rest:
// following a moved reservation, booking a lost one again, and flagging
// what only a person should settle. They are custom methods, not RPCs.

import (
	"context"
	"fmt"
	"slices"

	"github.com/shokki-engineering/aperio/internal/booking"
	"github.com/shokki-engineering/aperio/internal/engine"
	"github.com/shokki-engineering/aperio/internal/service/generated/servicex"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/stay/v1/staypbv1"
)

// Relink records on stay name what freebusy did to its reservation from:
// to, when set, takes from's place among its freebusy reservations; note,
// when set, is flagged on its notes, once for from however often it is
// found. A stay that already says both is not written again.
func (s *Server) Relink(ctx context.Context, name, from, to, note string) (*staypbv1.Reservation, error) {
	var out *staypbv1.Reservation
	err := s.Options().Run(ctx, "StayService", "Relink", func(ctx context.Context) error {
		r, err := s.Repositories().Reservations.Get(ctx, name)
		if err != nil {
			return servicex.NotFoundAs(err, reservationType, name)
		}
		out = r
		i := slices.Index(r.GetFreebusyReservations(), from)
		moved := to != "" && i >= 0
		if moved {
			r.FreebusyReservations[i] = to
		}
		flagged := false
		if note != "" {
			r.Notes, flagged = booking.Flag(r.GetNotes(), booking.Mark(from), note)
		}
		if !moved && !flagged {
			return nil
		}
		out, err = s.Repositories().Reservations.UpdateWithOutputOnly(ctx, r, []string{"notes"}, []string{"freebusy_reservations"})
		return err
	})
	return out, err
}

// Rebook books room line i of stay name again, under a key of its own, when
// freebusy no longer has lost -- the line's reservation -- and the rooms are
// still free, and relinks the stay to the new reservation with a flag. It
// returns engine.ErrUnavailable when the rooms have gone to someone else.
func (s *Server) Rebook(ctx context.Context, name string, i int, lost string) (*staypbv1.Reservation, error) {
	r, err := s.Repositories().Reservations.Get(ctx, name)
	if err != nil {
		return nil, servicex.NotFoundAs(err, reservationType, name)
	}
	if i >= len(r.GetRooms()) {
		return nil, fmt.Errorf("stay: %s has no room line %d", name, i)
	}
	venueName := servicex.ParentOf(name)
	w, err := s.h.stayWindow(ctx, venueName, r)
	if err != nil {
		return nil, err
	}
	line := r.GetRooms()[i]
	res, err := s.h.Resources(ctx, venueName, fmt.Sprintf("reservation.rooms[%d].space", i), line.GetSpace())
	if err != nil {
		return nil, err
	}
	booked, err := s.h.Engine.Book(ctx, engine.Booking{
		// Its own key, the same for every attempt at this repair.
		Key: booking.RoomsKey(name, i) + "#rebook/" + lost, Resources: res, Window: w, Quantity: line.GetRooms(),
		Attributes: map[string]any{"aperio_reservation": name, "contact": r.GetContact(), "space": line.GetSpace()},
	})
	if err != nil {
		return nil, err
	}
	out, err := s.Relink(ctx, name, lost, booked,
		"freebusy had lost this stay's rooms ("+lost+"); they were booked again as "+booked)
	if err == nil {
		s.h.noteStay(ctx, out)
	}
	return out, err
}

// Window is the window stay r takes in freebusy, in its venue's time zone:
// what each of its freebusy reservations should hold.
func (s *Server) Window(ctx context.Context, r *staypbv1.Reservation) (engine.Window, error) {
	return s.h.stayWindow(ctx, servicex.ParentOf(r.GetName()), r)
}
