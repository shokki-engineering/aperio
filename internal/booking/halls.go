// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package booking

// halls.go is the hold an accepted event keeps on its halls: from the
// moment its quote is accepted, each space the quote sells is booked in
// freebusy for the event's whole days (under SpaceKey), so a second quote
// for the same hall and date cannot be accepted. A session in the space
// replaces the hold with its own, timed booking; cancelling the event gives
// back whatever is still held. It is a freebusy reservation rather than a
// hold, as a hold lapses within a day and an event is planned for months.

import (
	"context"
	"errors"

	"github.com/shokki-engineering/aperio/internal/database/repository/repox"
	"github.com/shokki-engineering/aperio/internal/engine"
	"github.com/shokki-engineering/aperio/internal/service/generated/servicex"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/type/v1/typepbv1"
)

// HoldHall books space, a space of venueName, for event's days, or returns
// the booking a retry already made. A hall that is not free is Unavailable.
func (b *Base) HoldHall(ctx context.Context, venueName, event, field, space string, days *typepbv1.DateRange) (string, error) {
	res, err := b.Resources(ctx, venueName, field, space)
	if err != nil {
		return "", err
	}
	loc, err := b.Zone(ctx, venueName)
	if err != nil {
		return "", err
	}
	name, err := b.Engine.Book(ctx, engine.Booking{
		Key: SpaceKey(event, space), Resources: res, Window: Days(days, loc), Quantity: 1,
		Attributes: map[string]any{"aperio_event": event, "space": space, "aperio_hall_hold": true},
	})
	if errors.Is(err, engine.ErrUnavailable) {
		return "", Unavailable(space, 1)
	}
	return name, err
}

// HeldHall is the booking event keeps on space and its window; "" when it
// keeps none.
func (b *Base) HeldHall(ctx context.Context, event, space string) (string, engine.Window, error) {
	name, err := b.Engine.Find(ctx, SpaceKey(event, space))
	if err != nil || name == "" {
		return "", engine.Window{}, err
	}
	got, err := b.Engine.Lookup(ctx, []string{name})
	if err != nil {
		return "", engine.Window{}, err
	}
	return name, got[name].Window, nil
}

// ReleaseHalls gives back every hall event still keeps: it asks after each
// space of the venue, as nothing in aperio lists them (an event has no
// field for them).
func (b *Base) ReleaseHalls(ctx context.Context, event, reason string) error {
	venueName := servicex.ParentOf(event)
	in := repox.ListInput{PageSize: 200}
	var errs []error
	for {
		spaces, next, err := b.Spaces.List(ctx, venueName, in)
		if err != nil {
			return err
		}
		for _, sp := range spaces {
			name, err := b.Engine.Find(ctx, SpaceKey(event, sp.GetName()))
			if err == nil && name != "" {
				err = b.Engine.Cancel(ctx, name, reason)
			}
			if err != nil {
				errs = append(errs, err)
			}
		}
		if next == "" {
			return errors.Join(errs...)
		}
		in.PageToken = next
	}
}
