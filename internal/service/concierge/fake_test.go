// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package concierge_test

// fake_test.go is a freebusy in memory, as far as the concierge needs one:
// each resource one room, and -- as in freebusy -- a hold and a booking made
// with the same key the same claim, so booking after holding takes the held
// rooms rather than more.

import (
	"context"
	"sync"
	"time"

	"github.com/shokki-engineering/aperio/internal/engine"
)

type fakeFreebusy struct {
	mu    sync.Mutex
	taken map[string][]string // claim name -> the resources it takes
	keys  map[string]string   // key -> claim name
	books int                 // Book calls that made a claim of their own
}

func newFakeFreebusy() *fakeFreebusy {
	return &fakeFreebusy{taken: map[string][]string{}, keys: map[string]string{}}
}

// free is resources no claim takes; the caller holds mu. Every claim spans
// the test's one stay, so overlap is not checked.
func (f *fakeFreebusy) free(resources []string) []string {
	var out []string
	for _, r := range resources {
		busy := false
		for _, rs := range f.taken {
			for _, x := range rs {
				busy = busy || x == r
			}
		}
		if !busy {
			out = append(out, r)
		}
	}
	return out
}

func (f *fakeFreebusy) Free(_ context.Context, resources []string, _ engine.Window, quantity int32) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.free(resources)) >= int(quantity), nil
}

// claim takes quantity of resources under key, or returns the claim the key
// already made.
func (f *fakeFreebusy) claim(b engine.Booking) (string, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if name, ok := f.keys[b.Key]; ok {
		return name, false, nil
	}
	free := f.free(b.Resources)
	if len(free) < int(b.Quantity) {
		return "", false, engine.ErrUnavailable
	}
	name := "reservations/fake-" + b.Key
	f.taken[name], f.keys[b.Key] = free[:b.Quantity], name
	return name, true, nil
}

func (f *fakeFreebusy) Hold(_ context.Context, b engine.Booking) (engine.Hold, error) {
	name, _, err := f.claim(b)
	return engine.Hold{Name: name, Expire: time.Now().Add(15 * time.Minute)}, err
}

func (f *fakeFreebusy) Held(_ context.Context, key string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	name, ok := f.keys[key]
	_, active := f.taken[name]
	return ok && active, nil
}

func (f *fakeFreebusy) Book(_ context.Context, b engine.Booking) (string, error) {
	name, made, err := f.claim(b)
	if made {
		f.mu.Lock()
		f.books++
		f.mu.Unlock()
	}
	return name, err
}

func (f *fakeFreebusy) Cancel(_ context.Context, reservation, _ string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.taken, reservation)
	return nil
}

func (f *fakeFreebusy) Note(context.Context, string, engine.Note) error { return nil }

// claims is how many claims are held.
func (f *fakeFreebusy) claims() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.taken)
}
