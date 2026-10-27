// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package engine

// holds.go is what aperio does with a hold beyond taking it: keeping it a
// while longer (a lead negotiated over a day), giving it up (a lead won or
// lost), and finding the reservation a key booked (an accepted event's hall).

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/shokki-engineering/freebusy-go"
	"github.com/shokki-engineering/freebusy-go/pb/allocationpb"
	"github.com/shokki-engineering/freebusy-go/pb/reservationpb"
	"google.golang.org/protobuf/types/known/durationpb"
)

// ExtendHold pushes hold's expiry to d from now.
func (c *Client) ExtendHold(ctx context.Context, hold string, d time.Duration) (Hold, error) {
	h, err := c.fb.Holds.Extend(ctx, &allocationpb.ExtendHoldRequest{Name: hold, Duration: durationpb.New(d)})
	switch {
	case errors.Is(err, freebusy.ErrNotFound), errors.Is(err, freebusy.ErrFailedPrecondition):
		// Expired, released or converted: freebusy refuses to extend any
		// hold that is not ACTIVE.
		return Hold{}, ErrLapsed
	case err != nil:
		return Hold{}, fmt.Errorf("engine: extend %s: %w", hold, err)
	}
	return Hold{Name: h.GetName(), Expire: h.GetExpireTime().AsTime()}, nil
}

// ReleaseHold gives hold up; freebusy's ReleaseHold is idempotent, and a hold
// it no longer has is released already.
func (c *Client) ReleaseHold(ctx context.Context, hold string) error {
	_, err := c.fb.Holds.Release(ctx, &allocationpb.ReleaseHoldRequest{Name: hold})
	if err != nil && !errors.Is(err, freebusy.ErrNotFound) {
		return fmt.Errorf("engine: release %s: %w", hold, err)
	}
	return nil
}

// Find is the reservation Book made under key -- it is named after the key
// -- while it still holds its resources.
func (c *Client) Find(ctx context.Context, key string) (string, error) {
	name := "reservations/" + reservationID(key)
	res, err := c.fb.Reservations.Get(ctx, &reservationpb.GetReservationRequest{Name: name})
	switch {
	case errors.Is(err, freebusy.ErrNotFound):
		return "", nil
	case err != nil:
		return "", fmt.Errorf("engine: get %s: %w", name, err)
	}
	if stateOf(res.GetState()) != Live {
		return "", nil
	}
	return name, nil
}
