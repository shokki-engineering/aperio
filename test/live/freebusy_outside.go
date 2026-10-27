// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package live

// freebusy_outside.go is the fake freebusy's reads of its reservations
// (Lookup) and what happens in freebusy without aperio asking: an operator
// cancelling or moving a reservation, a link broken, a hold lapsing. The
// tests of aperio's webhook and reconciler drive these.

import (
	"context"

	"github.com/shokki-engineering/aperio/internal/engine"
)

func (f *Freebusy) Lookup(_ context.Context, reservations []string) (map[string]engine.Reservation, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := map[string]engine.Reservation{}
	for _, name := range reservations {
		switch c, live := f.claims[name]; {
		case live:
			out[name] = engine.Reservation{Name: name, State: engine.Live, Window: c.window, Quantity: int32(len(c.resources))}
		case f.moved[name] != "":
			out[name] = engine.Reservation{Name: name, State: engine.Moved, Replacement: f.moved[name]}
		case f.ended[name]:
			out[name] = engine.Reservation{Name: name, State: engine.Ended}
		}
	}
	return out, nil
}

// CancelOutside cancels reservation as an operator would in freebusy: its
// resources are freed, and aperio is not told.
func (f *Freebusy) CancelOutside(reservation string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.claims, reservation)
	f.ended[reservation] = true
}

// Forget removes reservation as if freebusy never had it -- a broken link --
// and its key with it, so a booking under that key starts afresh.
func (f *Freebusy) Forget(reservation string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.claims, reservation)
	for k, n := range f.keys {
		if n == reservation {
			delete(f.keys, k)
		}
	}
}

// MoveOutside reschedules reservation to w, as freebusy's
// RescheduleReservation does: a replacement holds the same resources for w
// and the original is RESCHEDULED. It returns the replacement's name.
func (f *Freebusy) MoveOutside(reservation string, w engine.Window) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	c := f.claims[reservation]
	delete(f.claims, reservation)
	to := reservation + "-moved"
	c.window = w
	f.claims[to], f.moved[reservation] = c, to
	return to
}

// LapseHold lets hold expire, as freebusy's sweeper does.
func (f *Freebusy) LapseHold(hold string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if c, live := f.claims[hold]; live && !c.booked {
		delete(f.claims, hold)
	}
}

// HoldOf is the window and quantity hold takes, and whether it is active.
func (f *Freebusy) HoldOf(hold string) (engine.Window, int, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	c, live := f.claims[hold]
	return c.window, len(c.resources), live && !c.booked
}
