// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package stay

// notes.go puts who a stay is for on its rooms' calendars (internal/calendar
// writes the note), and refreshes it when the stay is edited.

import (
	"context"

	"github.com/shokki-engineering/aperio/internal/calendar"
	"github.com/shokki-engineering/aperio/internal/service/generated/servicex"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/stay/v1/staypbv1"
)

// AfterUpdateReservation refreshes the stay's notes: its guests, contact or
// requests may have changed.
func (h *hooks) AfterUpdateReservation(ctx context.Context, m *staypbv1.Reservation) error {
	h.noteStay(ctx, m)
	return nil
}

// noteStay notes each room line of r on the calendars of the rooms its
// freebusy reservation took; line i was booked as freebusy reservation i.
func (h *hooks) noteStay(ctx context.Context, r *staypbv1.Reservation) {
	if r.GetState() == staypbv1.Reservation_STATE_CANCELLED {
		return
	}
	venueName := servicex.ParentOf(r.GetName())
	w, err := h.stayWindow(ctx, venueName, r)
	if err != nil {
		h.NoteFailed(ctx, r.GetName(), err)
		return
	}
	c := h.Contact(ctx, r.GetContact())
	for i, line := range r.GetRooms() {
		if i >= len(r.GetFreebusyReservations()) {
			break
		}
		n := calendar.Stay(r, c, line.GetRooms(), h.Where(ctx, venueName, line.GetSpace()), w)
		if err := h.Engine.Note(ctx, r.GetFreebusyReservations()[i], n); err != nil {
			h.NoteFailed(ctx, r.GetName(), err)
		}
	}
}
