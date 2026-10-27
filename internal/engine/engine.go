// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package engine is aperio's connection to freebusy, the engine that knows
// what is free: whether a venue's rooms or spaces are free for a window,
// taking them, and giving them back.
//
// aperio keeps the booking -- its guests, rooms, menus and money -- and
// freebusy keeps the time: every reservation and session aperio takes is a
// freebusy reservation first, named on aperio's record. Engine is the little
// of freebusy aperio uses, so the booking service can be tested without one
// (a fake Engine) and served with one (Client).
package engine

import (
	"context"
	"errors"
	"time"
)

// ErrUnavailable is returned by Book when the resources are not free for the
// window: taken by another booking, blocked, or outside their opening hours.
var ErrUnavailable = errors.New("engine: not available")

// ErrLapsed is returned by ExtendHold for a hold that is no longer active:
// expired, released, or made into a reservation.
var ErrLapsed = errors.New("engine: the hold lapsed")

// Engine is what aperio asks of freebusy.
type Engine interface {
	// Free reports whether quantity of resources are free for the whole of
	// window. It takes nothing.
	Free(ctx context.Context, resources []string, window Window, quantity int32) (bool, error)

	// Hold takes quantity of b.Resources for b.Window for a few minutes --
	// freebusy's hold, which lapses unless a reservation is made from it --
	// and returns it. It is idempotent on b.Key, and Book with the same key
	// makes its reservation from this hold rather than taking another: hold
	// first while a guest decides, book once they have. ErrUnavailable when
	// the resources are not free.
	Hold(ctx context.Context, b Booking) (Hold, error)

	// Held reports whether an active hold was made under key: a booking
	// whose rooms a hold already has must not count them as taken.
	Held(ctx context.Context, key string) (bool, error)

	// Book takes quantity of resources for window and returns the freebusy
	// reservation holding them. It is idempotent on b.Key: a retry with the
	// same key returns the reservation the first call made rather than taking
	// more. ErrUnavailable when they are not free.
	Book(ctx context.Context, b Booking) (string, error)

	// Cancel cancels a freebusy reservation, freeing its resources, and
	// takes its Note off their calendars. One that is already cancelled, or
	// gone, is not an error.
	Cancel(ctx context.Context, reservation, reason string) error

	// Note puts n on the calendar of every resource reservation took, or
	// replaces the one already there: who the booking is for, for whoever
	// subscribes to the calendar. It blocks nothing -- the reservation does.
	Note(ctx context.Context, reservation string, n Note) error

	// ExtendHold pushes hold's expiry to d from now -- freebusy allows a day
	// at most -- and returns it. ErrLapsed when it is no longer active.
	ExtendHold(ctx context.Context, hold string, d time.Duration) (Hold, error)

	// ReleaseHold gives hold up, freeing what it took. One already
	// released, lapsed or gone is not an error.
	ReleaseHold(ctx context.Context, hold string) error

	// Find is the live reservation Book made under key, or "" when there
	// is none (never booked, or cancelled since).
	Find(ctx context.Context, key string) (string, error)

	// Lookup reads reservations as freebusy has them now, in batches rather
	// than one call each. A reservation freebusy does not know is missing
	// from the map.
	Lookup(ctx context.Context, reservations []string) (map[string]Reservation, error)
}

// State is where a freebusy reservation stands, as aperio reads it.
type State int

// The states Lookup reports.
const (
	// Live holds its resources: pending, confirmed, or being moved.
	Live State = iota + 1
	// Ended holds nothing: cancelled, being cancelled, or failed.
	Ended
	// Moved was rescheduled: Replacement holds the resources now.
	Moved
)

// Reservation is a freebusy reservation as Lookup reads it.
type Reservation struct {
	Name   string
	State  State
	Window Window
	// Quantity is how many resources it holds.
	Quantity int32
	// Replacement is the reservation a Moved one moved to.
	Replacement string
}

// Note is what a booking shows on its resources' calendars.
type Note struct {
	Summary     string
	Description string
	Location    string
	// Window is when; with AllDay, only its dates count, [Start, End) days.
	Window Window
	AllDay bool
}

// Hold is a hold Hold took: freebusy's name for it, and when it lapses.
type Hold struct {
	Name   string
	Expire time.Time
}

// Window is the span of time a booking takes, [Start, End).
type Window struct {
	Start, End time.Time
}

// Booking is one Book: what to take, for how long, and the key that makes a
// retry find it.
type Booking struct {
	// Key identifies the booking across retries: an aperio record's name and
	// the line of it, e.g. "venues/v/reservations/r#rooms/0".
	Key string

	// Resources are the candidates: Quantity of them are taken.
	Resources []string

	// Window is when.
	Window Window

	// Quantity is how many of Resources.
	Quantity int32

	// Attributes are kept on the freebusy reservation, naming the aperio
	// record it holds the time for.
	Attributes map[string]any
}
