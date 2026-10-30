// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package booking

// halls.go is the hold an accepted event keeps on its halls: from the
// moment its quote is accepted, each space the quote sells is booked in
// freebusy for the event's whole days (under SpaceKey), so a second quote
// for the same hall and date cannot be accepted. A session in the space
// replaces the hold with its own, timed booking; cancelling the event gives
// back whatever is still held. The event records each hold it keeps
// (Event.freebusy_holds), which is what is given back and what the
// reconciler checks. It is a freebusy reservation rather than a
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

// Hall is a space an accepted event holds, and the request field it was
// named in, for errors.
type Hall struct {
	Field, Space string
}

// ReleaseHalls gives back holds, the freebusy reservations an event keeps
// on its halls (Event.freebusy_holds): exactly those, rather than asking
// freebusy after every space of the venue (SweepHalls does that, for what
// the list missed). One already given back -- a session replaced it, an
// operator cancelled it -- is not an error.
func (b *Base) ReleaseHalls(ctx context.Context, holds []string, reason string) error {
	return b.Release(ctx, holds, reason)
}

// SweepHalls gives back every hold event still keeps on a space of its
// venue, found by its key (SpaceKey) rather than by the event's list; Find
// names only live ones, so what the list gave back is not cancelled twice.
// The list is the record, but a hold can be live and off it: a session
// that took it off to replace it, failed to cancel it, and failed to put it
// back (the event service's restoreHold). Such a hold does its job
// while the event lives -- the hall stays the event's, and the next session
// there finds it by its key -- so the one moment it leaks is the event's
// cancel, and that is where the sweep runs, rather than in every reconciler
// round, which would cost a lookup per space for every live event. It
// costs one per bookable space of the venue, once per cancelled event.
func (b *Base) SweepHalls(ctx context.Context, event, reason string) error {
	in := repox.ListInput{PageSize: 200}
	var errs []error
	for {
		spaces, next, err := b.Spaces.List(ctx, servicex.ParentOf(event), in)
		if err != nil {
			return errors.Join(append(errs, err)...)
		}
		for _, sp := range spaces {
			if len(sp.GetResources()) == 0 {
				continue // never bookable, so never held
			}
			name, err := b.Engine.Find(ctx, SpaceKey(event, sp.GetName()))
			if err == nil && name != "" {
				err = b.Engine.Cancel(ctx, name, reason)
			}
			errs = append(errs, err)
		}
		if next == "" {
			return errors.Join(errs...)
		}
		in.PageToken = next
	}
}
