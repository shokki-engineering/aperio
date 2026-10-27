// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package engine

// lookup.go reads many freebusy reservations at once, for the reconciler and
// the webhook: freebusy has no batch get, so a ListReservations filtered by
// name (`name = "a" OR name = "b"`) reads a batch in one call.

import (
	"context"
	"fmt"
	"strings"

	"github.com/shokki-engineering/freebusy-go/pb/reservationpb"
)

// lookupBatch is how many reservations one ListReservations names: its
// filter stays a few kilobytes.
const lookupBatch = 50

// Lookup reads reservations, lookupBatch to a call.
func (c *Client) Lookup(ctx context.Context, reservations []string) (map[string]Reservation, error) {
	out := make(map[string]Reservation, len(reservations))
	for start := 0; start < len(reservations); start += lookupBatch {
		batch := reservations[start:min(start+lookupBatch, len(reservations))]
		terms := make([]string, len(batch))
		for i, name := range batch {
			terms[i] = fmt.Sprintf("name = %q", name)
		}
		req := &reservationpb.ListReservationsRequest{PageSize: int32(len(batch)), Filter: strings.Join(terms, " OR ")}
		for {
			page, err := c.fb.Reservations.List(ctx, req)
			if err != nil {
				return nil, fmt.Errorf("engine: list reservations: %w", err)
			}
			for _, r := range page.GetReservations() {
				out[r.GetName()] = Reservation{
					Name:        r.GetName(),
					State:       stateOf(r.GetState()),
					Window:      Window{Start: r.GetStartTime().AsTime(), End: r.GetEndTime().AsTime()},
					Quantity:    int32(len(r.GetResources())),
					Replacement: r.GetReplacement(),
				}
			}
			if page.GetNextPageToken() == "" {
				break
			}
			req.PageToken = page.GetNextPageToken()
		}
	}
	return out, nil
}

// stateOf is freebusy's reservation state as aperio reads it: a reservation
// being moved still holds its resources until the move is done.
func stateOf(s reservationpb.ReservationState) State {
	switch s {
	case reservationpb.ReservationState_RESERVATION_STATE_RESCHEDULED:
		return Moved
	case reservationpb.ReservationState_RESERVATION_STATE_CANCELLING,
		reservationpb.ReservationState_RESERVATION_STATE_CANCELLED,
		reservationpb.ReservationState_RESERVATION_STATE_FAILED:
		return Ended
	}
	return Live
}
