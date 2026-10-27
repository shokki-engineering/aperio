// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package freebusysync

// stays.go settles stays: their freebusy reservations read in one batch,
// and each stay brought in step with what freebusy has (sync.go says how).

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"

	"github.com/shokki-engineering/aperio/internal/booking"
	"github.com/shokki-engineering/aperio/internal/engine"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/stay/v1/staypbv1"
)

// live are the stay states whose rooms freebusy should be holding.
var live = []staypbv1.Reservation_State{
	staypbv1.Reservation_STATE_PENDING, staypbv1.Reservation_STATE_CONFIRMED, staypbv1.Reservation_STATE_CHECKED_IN,
}

// settleStays settles every live stay of stays, reading all their freebusy
// reservations in one Lookup.
func (s *Syncer) settleStays(ctx context.Context, stays []*staypbv1.Reservation) error {
	var names []string
	for _, r := range stays {
		if booking.InState(r.GetState(), live) {
			names = append(names, r.GetFreebusyReservations()...)
		}
	}
	if len(names) == 0 {
		return nil
	}
	got, err := s.engine.Lookup(ctx, names)
	if err != nil {
		return err
	}
	if err := suspect(names, got); err != nil {
		return err
	}
	var errs []error
	for _, r := range stays {
		if booking.InState(r.GetState(), live) {
			errs = append(errs, s.settleStay(ctx, r, got))
		}
	}
	return errors.Join(errs...)
}

// settleStay brings r in step with got, freebusy's reservations.
func (s *Syncer) settleStay(ctx context.Context, r *staypbv1.Reservation, got map[string]engine.Reservation) error {
	w, err := s.stays.Window(ctx, r)
	if err != nil {
		return err
	}
	var ended []string
	var errs []error
	for i, name := range r.GetFreebusyReservations() {
		fb, ok := got[name]
		switch {
		case !ok:
			errs = append(errs, s.rebookStay(ctx, r, i, name))
		case fb.State == engine.Ended:
			ended = append(ended, name)
		case fb.State == engine.Moved:
			errs = append(errs, s.flagStay(ctx, r, name, fb.Replacement, fmt.Sprintf(
				"freebusy moved these rooms to %s; the stay's dates in aperio were not changed: check them", fb.Replacement)))
		case !fb.Window.Start.Equal(w.Start) || !fb.Window.End.Equal(w.End) || i >= len(r.GetRooms()) || fb.Quantity != r.GetRooms()[i].GetRooms():
			errs = append(errs, s.flagStay(ctx, r, name, "", fmt.Sprintf("freebusy holds %d room(s) for %s, not what this stay says: check it",
				fb.Quantity, span(fb.Window))))
		}
	}
	if len(ended) == 0 {
		return errors.Join(errs...)
	}
	cancellable := r.GetState() != staypbv1.Reservation_STATE_CHECKED_IN
	if cancellable && len(ended) == len(r.GetFreebusyReservations()) {
		_, err := s.stays.CancelReservation(ctx, &staypbv1.CancelReservationRequest{Name: r.GetName(), Reason: "cancelled in freebusy"})
		slog.InfoContext(ctx, "freebusysync: stay cancelled in freebusy", "stay", r.GetName(), "err", err)
		return errors.Join(append(errs, err)...)
	}
	for _, name := range ended {
		errs = append(errs, s.flagStay(ctx, r, name, "",
			"freebusy cancelled these rooms; the stay was not cancelled, as it keeps others or the guest is in: settle it"))
	}
	return errors.Join(errs...)
}

// rebookStay books line i of r again, as freebusy no longer has lost, or
// flags it when the rooms are gone.
func (s *Syncer) rebookStay(ctx context.Context, r *staypbv1.Reservation, i int, lost string) error {
	_, err := s.stays.Rebook(ctx, r.GetName(), i, lost)
	if err == nil {
		slog.InfoContext(ctx, "freebusysync: stay's lost rooms booked again", "stay", r.GetName(), "lost", lost)
		return nil
	}
	slog.WarnContext(ctx, "freebusysync: stay's lost rooms not booked again", "stay", r.GetName(), "lost", lost, "err", err)
	return s.flagStay(ctx, r, lost, "", "freebusy has no reservation for these rooms, and they could not be booked again: settle it")
}

// flagStay relinks and flags r about reservation, logging what it flags.
func (s *Syncer) flagStay(ctx context.Context, r *staypbv1.Reservation, reservation, to, note string) error {
	slog.WarnContext(ctx, "freebusysync: stay out of step with freebusy", "stay", r.GetName(), "reservation", reservation, "note", note)
	_, err := s.stays.Relink(ctx, r.GetName(), reservation, to, note)
	return err
}

// span is w for a note.
func span(w engine.Window) string {
	return w.Start.Format("2006-01-02 15:04") + " to " + w.End.Format("2006-01-02 15:04 MST")
}

// quote is v as an AIP-160 string literal.
func quote(v string) string { return strconv.Quote(v) }
