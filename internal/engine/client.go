// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package engine

// client.go is Engine over freebusy's Go SDK: availability from the
// AllocationService, a booking as a hold made into a reservation.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/shokki-engineering/freebusy-go"
	"github.com/shokki-engineering/freebusy-go/option"
	"github.com/shokki-engineering/freebusy-go/pb/allocationpb"
	"github.com/shokki-engineering/freebusy-go/pb/reservationpb"
)

// Client is Engine over a freebusy server.
type Client struct {
	fb *freebusy.Client
}

var _ Engine = (*Client)(nil)

// Dial connects to freebusy. The SDK reads FREEBUSY_ENDPOINT,
// FREEBUSY_INSECURE and FREEBUSY_CA_FILE; opts override them.
func Dial(ctx context.Context, opts ...option.RequestOption) (*Client, error) {
	fb, err := freebusy.NewClient(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("engine: dial freebusy: %w", err)
	}
	return &Client{fb: fb}, nil
}

// Close closes the connection.
func (c *Client) Close() error { return c.fb.Close() }

// Free reports whether quantity of resources are free for window.
func (c *Client) Free(ctx context.Context, resources []string, window Window, quantity int32) (bool, error) {
	res, err := c.fb.Availability.CheckAvailability(ctx, &allocationpb.CheckAvailabilityRequest{
		Resources: resources,
		Window:    freebusy.Window(window.Start, window.End),
		Quantity:  quantity,
	})
	if err != nil {
		return false, fmt.Errorf("engine: check availability: %w", err)
	}
	return res.GetBookable(), nil
}

// Hold holds quantity of b.Resources for b.Window, keyed by b.Key.
func (c *Client) Hold(ctx context.Context, b Booking) (Hold, error) {
	h, err := c.hold(ctx, b)
	if err != nil {
		return Hold{}, err
	}
	return Hold{Name: h.GetName(), Expire: h.GetExpireTime().AsTime()}, nil
}

// Held reports whether the hold made under key is still active.
func (c *Client) Held(ctx context.Context, key string) (bool, error) {
	h, err := c.fb.Holds.Get(ctx, &allocationpb.GetHoldRequest{Name: "holds/" + reservationID(key)})
	switch {
	case errors.Is(err, freebusy.ErrNotFound):
		return false, nil
	case err != nil:
		return false, fmt.Errorf("engine: get hold: %w", err)
	}
	return h.GetState() == allocationpb.HoldState_HOLD_STATE_ACTIVE, nil
}

// hold is freebusy's CreateHold for b: a retry, or Book after Hold, with
// the same key gets the first hold back.
func (c *Client) hold(ctx context.Context, b Booking) (*allocationpb.Hold, error) {
	hold, err := c.fb.Holds.New(ctx, &allocationpb.CreateHoldRequest{
		Hold: &allocationpb.Hold{
			Window:             freebusy.Window(b.Window.Start, b.Window.End),
			Quantity:           b.Quantity,
			CandidateResources: b.Resources,
			IdempotencyKey:     b.Key,
		},
		HoldId: reservationID(b.Key), // so Held finds it by the key
	})
	switch {
	case errors.Is(err, freebusy.ErrCapacityExhausted):
		return nil, ErrUnavailable
	case err != nil:
		return nil, fmt.Errorf("engine: hold: %w", err)
	}
	return hold, nil
}

// Book holds quantity of b.Resources for b.Window and makes the hold a
// reservation. Both carry b.Key: the hold as its idempotency key, the
// reservation as its id, so a retry gets the first call's reservation back.
func (c *Client) Book(ctx context.Context, b Booking) (string, error) {
	id := reservationID(b.Key)
	hold, err := c.hold(ctx, b)
	if err != nil {
		return "", err
	}
	attrs, err := freebusy.NewAttributes(b.Attributes)
	if err != nil {
		return "", fmt.Errorf("engine: attributes: %w", err)
	}
	res, err := c.fb.Reservations.New(ctx, &reservationpb.CreateReservationRequest{
		ReservationId: id,
		Reservation:   &reservationpb.Reservation{Hold: hold.GetName(), Attributes: attrs},
	})
	switch {
	case errors.Is(err, freebusy.ErrAlreadyExists):
		// A retry: the first call made it.
		return "reservations/" + id, nil
	case errors.Is(err, freebusy.ErrHoldNotActive):
		// The hold lapsed between the two calls; its resources may be gone.
		return "", ErrUnavailable
	case err != nil:
		return "", fmt.Errorf("engine: reserve: %w", err)
	}
	return res.GetName(), nil
}

// Cancel takes reservation's notes down and cancels it, unless it is
// already cancelled or gone.
func (c *Client) Cancel(ctx context.Context, reservation, reason string) error {
	res, err := c.fb.Reservations.Get(ctx, &reservationpb.GetReservationRequest{Name: reservation})
	switch {
	case errors.Is(err, freebusy.ErrNotFound):
		return nil
	case err != nil:
		return fmt.Errorf("engine: get %s: %w", reservation, err)
	}
	if err := c.unnote(ctx, res); err != nil {
		return err
	}
	switch res.GetState() {
	case reservationpb.ReservationState_RESERVATION_STATE_CANCELLED,
		reservationpb.ReservationState_RESERVATION_STATE_CANCELLING:
		return nil
	}
	_, err = c.fb.Reservations.Cancel(ctx, &reservationpb.CancelReservationRequest{
		Name: reservation, Reason: reason, Etag: res.GetEtag(),
	})
	if err != nil && !errors.Is(err, freebusy.ErrNotFound) {
		return fmt.Errorf("engine: cancel %s: %w", reservation, err)
	}
	return nil
}

// reservationID is the freebusy reservation id for a booking key: a hash, as
// a key (an aperio name and a line) is not an AIP-122 id, and stable, so a
// retry names the same reservation.
func reservationID(key string) string {
	sum := sha256.Sum256([]byte(key))
	return "aperio-" + hex.EncodeToString(sum[:])[:32]
}
