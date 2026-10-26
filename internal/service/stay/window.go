// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package stay

// window.go is the window a stay takes in freebusy.

import (
	"context"

	"github.com/shokki-engineering/aperio/internal/engine"
	"github.com/shokki-engineering/aperio/internal/service/generated/servicex"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/stay/v1/staypbv1"
)

// stayWindow is the window a stay takes in its venue's time zone: from the
// arrival day's midnight to the departure day's, so the nights between are
// the guest's and a stay ending on a day never overlaps one starting on it.
func (h *hooks) stayWindow(ctx context.Context, venueName string, r *staypbv1.Reservation) (engine.Window, error) {
	loc, err := h.Zone(ctx, venueName)
	if err != nil {
		return engine.Window{}, err
	}
	w := engine.Stay(r.GetArrivalDate(), r.GetDepartureDate(), loc)
	if !w.End.After(w.Start) {
		return engine.Window{}, servicex.BadField("INVALID_DATES", "reservation.departure_date",
			"the departure date is not after the arrival date", "Set a departure date after the arrival date")
	}
	return w, nil
}
