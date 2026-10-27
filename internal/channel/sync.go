// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package channel

// sync.go is Sync: a channel's reservation notifications applied to aperio's
// stays, the same for every channel.
//
// A channel delivers at least once, so every notification is idempotent on
// the channel and its reservation id (a reservation's channel and
// channel_reference, which are indexed): a redelivered new reservation finds
// the stay the first delivery made, a redelivered cancellation finds it
// cancelled. A new stay's id is the channel's reservation id, so two
// deliveries racing each other cannot both make one: the second is refused
// as already existing and finds the first's.

import (
	"context"
	"fmt"
	"strings"

	"github.com/shokki-engineering/aperio/protobuf/generated/go/stay/v1/staypbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/type/v1/typepbv1"
)

// Sync applies a channel's notifications to aperio's stays.
type Sync struct {
	// Channel is the channel the stays are recorded as from.
	Channel typepbv1.Channel
	// Prefix starts the ids of the stays it makes, naming the channel:
	// "bookingcom" makes venues/{venue}/reservations/bookingcom-4105349872.
	Prefix string
	// Hotels are the venues the channel sells, by its codes.
	Hotels []Hotel

	Stays  Stays
	Guests Guests
}

// Apply applies n and returns the stay it made, changed or cancelled.
func (s *Sync) Apply(ctx context.Context, n Notification) (string, error) {
	if strings.TrimSpace(n.Reference) == "" {
		return "", fmt.Errorf("%w: the reservation id", ErrMissing)
	}
	h, ok := hotel(s.Hotels, n.Hotel)
	if !ok && (n.Action != ActionCancel || n.Hotel != "") {
		return "", fmt.Errorf("%w: %q", ErrUnknownHotel, n.Hotel)
	}
	if n.Action == ActionCancel && !ok {
		// A cancellation that names no hotel, with several: the one that
		// has the reservation.
		return s.cancelAnywhere(ctx, n)
	}
	switch n.Action {
	case ActionNew:
		return s.book(ctx, h, n)
	case ActionModify:
		return s.modify(ctx, h, n)
	case ActionCancel:
		return s.cancel(ctx, h, n)
	}
	return "", fmt.Errorf("%w: action %d", ErrUnsupported, n.Action)
}

// found is the stays a channel's reservation id names at a venue: every one,
// newest first (a rebooked reservation has one cancelled stay per change),
// and the one still live, if any.
type found struct {
	all  []*staypbv1.Reservation
	live *staypbv1.Reservation
}

// find is the stays at venue the channel's reference names.
func (s *Sync) find(ctx context.Context, venue, reference string) (found, error) {
	var f found
	req := &staypbv1.ListReservationsRequest{
		Parent:   venue,
		PageSize: 100,
		Filter: fmt.Sprintf("channel = %s AND channel_reference = %q",
			strings.TrimPrefix(s.Channel.String(), "CHANNEL_"), reference),
		OrderBy: "create_time desc",
	}
	for {
		page, err := s.Stays.ListReservations(ctx, req)
		if err != nil {
			return found{}, err
		}
		for _, r := range page.GetReservations() {
			f.all = append(f.all, r)
			if f.live == nil && r.GetState() != staypbv1.Reservation_STATE_CANCELLED {
				f.live = r
			}
		}
		if page.GetNextPageToken() == "" {
			return f, nil
		}
		req.PageToken = page.GetNextPageToken()
	}
}

// cancel cancels the live stay n names at h. One already cancelled, or never
// made, is cancelled: there is nothing left to cancel.
func (s *Sync) cancel(ctx context.Context, h Hotel, n Notification) (string, error) {
	f, err := s.find(ctx, h.Venue, n.Reference)
	if err != nil {
		return "", err
	}
	if f.live == nil {
		if len(f.all) > 0 {
			return f.all[0].GetName(), nil
		}
		return "", nil
	}
	return s.cancelStay(ctx, f.live, "cancelled on "+s.Prefix)
}

// cancelAnywhere cancels n at whichever of the hotels has it.
func (s *Sync) cancelAnywhere(ctx context.Context, n Notification) (string, error) {
	for _, h := range s.Hotels {
		f, err := s.find(ctx, h.Venue, n.Reference)
		if err != nil {
			return "", err
		}
		if len(f.all) > 0 {
			return s.cancel(ctx, h, n)
		}
	}
	return "", fmt.Errorf("%w: %q", ErrUnknownReservation, n.Reference)
}

// cancelStay cancels r with reason.
func (s *Sync) cancelStay(ctx context.Context, r *staypbv1.Reservation, reason string) (string, error) {
	out, err := s.Stays.CancelReservation(ctx, &staypbv1.CancelReservationRequest{Name: r.GetName(), Reason: reason})
	if err != nil {
		return "", err
	}
	return out.GetName(), nil
}
