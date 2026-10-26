// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package bookings

// notes.go puts who a booking is for on the calendars of the rooms and halls
// it took (internal/calendar writes the note, the engine puts it there), and
// refreshes it when the booking is edited. A note that cannot be written is
// logged, not returned: the booking stands without it, and the next edit
// writes it again.

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/shokki-engineering/aperio/internal/calendar"
	"github.com/shokki-engineering/aperio/internal/service/generated/servicex"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/booking/v1/bookingpbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/crm/v1/crmpbv1"
)

// AfterUpdateReservation refreshes the stay's notes: its guests, contact or
// requests may have changed.
func (h *hooks) AfterUpdateReservation(ctx context.Context, m *bookingpbv1.Reservation) error {
	h.noteStay(ctx, m)
	return nil
}

// AfterUpdateSession refreshes the session's note.
func (h *hooks) AfterUpdateSession(ctx context.Context, m *bookingpbv1.Session) error {
	h.noteSession(ctx, m)
	return nil
}

// noteStay notes each room line of r on the calendars of the rooms its
// freebusy reservation took; line i was booked as freebusy reservation i.
func (h *hooks) noteStay(ctx context.Context, r *bookingpbv1.Reservation) {
	if r.GetState() == bookingpbv1.Reservation_STATE_CANCELLED {
		return
	}
	venueName := servicex.ParentOf(r.GetName())
	w, err := h.stayWindow(ctx, venueName, r)
	if err != nil {
		h.noteFailed(ctx, r.GetName(), err)
		return
	}
	c := h.contact(ctx, r.GetContact())
	for i, line := range r.GetRooms() {
		if i >= len(r.GetFreebusyReservations()) {
			break
		}
		n := calendar.Stay(r, c, line.GetRooms(), h.where(ctx, venueName, line.GetSpace()), w)
		if err := h.engine.Note(ctx, r.GetFreebusyReservations()[i], n); err != nil {
			h.noteFailed(ctx, r.GetName(), err)
		}
	}
}

// noteSession notes s on its hall's calendar, in the venue's time zone.
func (h *hooks) noteSession(ctx context.Context, s *bookingpbv1.Session) {
	if s.GetFreebusyReservation() == "" {
		return
	}
	eventName := servicex.ParentOf(s.GetName())
	venueName := servicex.ParentOf(eventName)
	ev, err := h.repos.Events.Get(ctx, eventName)
	if err != nil {
		h.noteFailed(ctx, s.GetName(), err)
		return
	}
	w, err := sessionWindow(s)
	if err == nil {
		var loc *time.Location
		if loc, err = h.zone(ctx, venueName); err == nil {
			w.Start, w.End = w.Start.In(loc), w.End.In(loc)
		}
	}
	if err != nil {
		h.noteFailed(ctx, s.GetName(), err)
		return
	}
	n := calendar.Session(ev, s, h.contact(ctx, ev.GetContact()), h.where(ctx, venueName, s.GetSpace()), w)
	if err := h.engine.Note(ctx, s.GetFreebusyReservation(), n); err != nil {
		h.noteFailed(ctx, s.GetName(), err)
	}
}

// where is a note's location: the space, the venue, and its address.
func (h *hooks) where(ctx context.Context, venueName, space string) string {
	var parts []string
	if sp, err := h.spaces.Get(ctx, space); err == nil {
		parts = append(parts, sp.GetDisplayName())
	}
	if v, err := h.venues.Get(ctx, venueName); err == nil {
		parts = append(parts, v.GetDisplayName())
		parts = append(parts, v.GetAddress().GetAddressLines()...)
		if v.GetAddress().GetLocality() != "" {
			parts = append(parts, v.GetAddress().GetLocality())
		}
	}
	return strings.Join(parts, ", ")
}

// contact reads name, or nil when it cannot be read: the note then names
// the contact by its resource name.
func (h *hooks) contact(ctx context.Context, name string) *crmpbv1.Contact {
	c, err := h.contacts.Get(ctx, name)
	if err != nil {
		return nil
	}
	return c
}

// noteFailed logs a note that could not be written.
func (h *hooks) noteFailed(ctx context.Context, booking string, err error) {
	slog.WarnContext(ctx, "bookings: calendar note not written", "booking", booking, "err", err)
}
