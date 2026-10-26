// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package bookings_test

// fake_test.go is an in-memory freebusy: each resource is one unit of
// capacity, a booking takes Quantity of its resources for its window.

import (
	"context"
	"strings"
	"sync"

	"github.com/shokki-engineering/aperio/internal/engine"
)

// fakeEngine is engine.Engine in memory.
type fakeEngine struct {
	mu     sync.Mutex
	held   map[string]fakeHold    // by reservation name, active only
	byKey  map[string]string      // booking key -> reservation name
	failOn string                 // Book fails for keys ending in this
	notes  map[string]engine.Note // by reservation name
}

// fakeHold is what one reservation takes.
type fakeHold struct {
	resources []string
	window    engine.Window
}

func newFakeEngine() *fakeEngine {
	return &fakeEngine{held: map[string]fakeHold{}, byKey: map[string]string{}, notes: map[string]engine.Note{}}
}

// free is the resources not taken for w; the caller holds mu.
func (f *fakeEngine) free(resources []string, w engine.Window) []string {
	var out []string
	for _, r := range resources {
		taken := false
		for _, h := range f.held {
			for _, hr := range h.resources {
				if hr == r && h.window.Start.Before(w.End) && w.Start.Before(h.window.End) {
					taken = true
				}
			}
		}
		if !taken {
			out = append(out, r)
		}
	}
	return out
}

func (f *fakeEngine) Free(_ context.Context, resources []string, w engine.Window, quantity int32) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.free(resources, w)) >= int(quantity), nil
}

func (f *fakeEngine) Book(_ context.Context, b engine.Booking) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if name, ok := f.byKey[b.Key]; ok {
		return name, nil
	}
	if f.failOn != "" && strings.HasSuffix(b.Key, f.failOn) {
		return "", engine.ErrUnavailable
	}
	free := f.free(b.Resources, b.Window)
	if len(free) < int(b.Quantity) {
		return "", engine.ErrUnavailable
	}
	name := "reservations/fake-" + b.Key
	f.held[name] = fakeHold{resources: free[:b.Quantity], window: b.Window}
	f.byKey[b.Key] = name
	return name, nil
}

func (f *fakeEngine) Cancel(_ context.Context, reservation, _ string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.held, reservation)
	delete(f.notes, reservation)
	return nil
}

func (f *fakeEngine) Note(_ context.Context, reservation string, n engine.Note) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.notes[reservation] = n
	return nil
}

// note is the note on reservation's calendars.
func (f *fakeEngine) note(reservation string) engine.Note {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.notes[reservation]
}

// active is how many reservations are held.
func (f *fakeEngine) active() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.held)
}
