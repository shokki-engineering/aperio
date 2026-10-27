// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package stay

// notes.go puts who a stay is for on its rooms' calendars (internal/calendar
// writes the note), and refreshes it when the stay is edited.

import (
	"context"
	"time"

	"github.com/shokki-engineering/aperio/internal/calendar"
	"github.com/shokki-engineering/aperio/internal/service/generated/servicex"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/stay/v1/staypbv1"
	"google.golang.org/protobuf/proto"
)

// AfterUpdateReservation refreshes the stay's notes: its guests, contact or
// requests may have changed.
func (h *hooks) AfterUpdateReservation(ctx context.Context, m *staypbv1.Reservation) error {
	h.noteStay(ctx, m)
	return nil
}

// noteLater writes r's notes after the call that booked it has answered.
// A note is best effort -- a failure is logged, and the stay's next edit
// writes it again -- and writing one is a freebusy calendar event (a write
// there, and its availability read model recomputed), which the booking
// need not wait for. WaitNotes waits for those still being written.
func (h *hooks) noteLater(ctx context.Context, r *staypbv1.Reservation) {
	r, ctx = proto.Clone(r).(*staypbv1.Reservation), context.WithoutCancel(ctx)
	h.notes.Add(1)
	go func() {
		defer h.notes.Done()
		ctx, cancel := context.WithTimeout(ctx, noteTimeout)
		defer cancel()
		h.noteStay(ctx, r)
	}()
}

// noteTimeout bounds writing one stay's notes in the background.
const noteTimeout = 30 * time.Second

// WaitNotes waits for the notes being written in the background: on
// shutdown, and in tests that read a note right after a booking.
func (s *Server) WaitNotes() { s.h.notes.Wait() }

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
