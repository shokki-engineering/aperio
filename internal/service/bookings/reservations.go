// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package bookings

// reservations.go is the hooks around a stay's standard methods: its rooms
// checked against freebusy before create, booked after it, locked against
// edits, and given back on delete. Its states move in states.go.

import (
	"context"
	"errors"
	"fmt"

	"github.com/shokki-engineering/aperio/internal/engine"
	"github.com/shokki-engineering/aperio/internal/service/generated/servicex"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/booking/v1/bookingpbv1"
	"google.golang.org/protobuf/proto"
)

// BeforeCreateReservation checks that every room line is free for the stay.
// It takes nothing: validate_only runs it too.
func (h *hooks) BeforeCreateReservation(ctx context.Context, parent string, m *bookingpbv1.Reservation) error {
	w, err := h.stayWindow(ctx, parent, m)
	if err != nil {
		return err
	}
	for i, line := range m.GetRooms() {
		res, err := h.resources(ctx, parent, fmt.Sprintf("reservation.rooms[%d].space", i), line.GetSpace())
		if err != nil {
			return err
		}
		free, err := h.engine.Free(ctx, res, w, line.GetRooms())
		if err != nil {
			return err
		}
		if !free {
			return unavailable(line.GetSpace(), line.GetRooms())
		}
	}
	return nil
}

// AfterCreateReservation books the stored reservation's rooms in freebusy and
// records the freebusy reservations on it. Should any line fail, what was
// booked is given back and the reservation removed.
func (h *hooks) AfterCreateReservation(ctx context.Context, out *bookingpbv1.Reservation) error {
	parent := servicex.ParentOf(out.GetName())
	w, err := h.stayWindow(ctx, parent, out)
	if err != nil {
		return h.undoReservation(ctx, out, nil, err)
	}
	var booked []string
	for i, line := range out.GetRooms() {
		res, err := h.resources(ctx, parent, fmt.Sprintf("reservation.rooms[%d].space", i), line.GetSpace())
		if err != nil {
			return h.undoReservation(ctx, out, booked, err)
		}
		name, err := h.engine.Book(ctx, engine.Booking{
			Key:       fmt.Sprintf("%s#rooms/%d", out.GetName(), i),
			Resources: res,
			Window:    w,
			Quantity:  line.GetRooms(),
			Attributes: map[string]any{
				"aperio_reservation": out.GetName(),
				"contact":            out.GetContact(),
				"space":              line.GetSpace(),
			},
		})
		if errors.Is(err, engine.ErrUnavailable) {
			err = unavailable(line.GetSpace(), line.GetRooms())
		}
		if err != nil {
			return h.undoReservation(ctx, out, booked, err)
		}
		booked = append(booked, name)
	}
	out.FreebusyReservations = booked
	updated, err := h.repos.Reservations.UpdateOutputOnly(ctx, out, []string{"freebusy_reservations"})
	if err != nil {
		return h.undoReservation(ctx, out, booked, err)
	}
	proto.Reset(out)
	proto.Merge(out, updated)
	h.noteStay(ctx, out)
	return nil
}

// undoReservation gives back booked and removes the reservation, returning
// cause: a create that could not book its rooms leaves nothing behind.
func (h *hooks) undoReservation(ctx context.Context, r *bookingpbv1.Reservation, booked []string, cause error) error {
	errs := []error{cause}
	for _, name := range booked {
		if err := h.engine.Cancel(ctx, name, "aperio: the reservation could not be completed"); err != nil {
			errs = append(errs, err)
		}
	}
	if err := h.repos.Reservations.Delete(ctx, r.GetName()); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

// BeforeUpdateReservation refuses to change a stay's dates or rooms: they are
// booked in freebusy, and moving them is a new booking.
func (h *hooks) BeforeUpdateReservation(ctx context.Context, m *bookingpbv1.Reservation, paths []string) error {
	if !servicex.TouchesAny(paths, []string{"arrival_date", "departure_date", "rooms"}) {
		return nil
	}
	existing, err := h.repos.Reservations.Get(ctx, m.GetName())
	if err != nil {
		return servicex.NotFoundAs(err, reservationType, m.GetName())
	}
	same := proto.Equal(existing.GetArrivalDate(), m.GetArrivalDate()) &&
		proto.Equal(existing.GetDepartureDate(), m.GetDepartureDate()) &&
		len(existing.GetRooms()) == len(m.GetRooms())
	for i := 0; same && i < len(m.GetRooms()); i++ {
		same = proto.Equal(existing.GetRooms()[i], m.GetRooms()[i])
	}
	if same {
		return nil
	}
	return servicex.Blocked("BOOKING_LOCKED", reservationType, m.GetName(),
		"a reservation's dates and rooms are booked in freebusy and cannot be edited",
		"Cancel it with CancelReservation and make a new one")
}

// BeforeDeleteReservation gives back the reservation's rooms.
func (h *hooks) BeforeDeleteReservation(ctx context.Context, name string, force bool) error {
	r, err := h.repos.Reservations.Get(ctx, name)
	if err != nil {
		return servicex.NotFoundAs(err, reservationType, name)
	}
	return h.release(ctx, r.GetFreebusyReservations(), "aperio: the reservation was deleted")
}
