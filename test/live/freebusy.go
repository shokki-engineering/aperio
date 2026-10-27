// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package live

// freebusy.go is freebusy in memory, as far as aperio's services ask it
// (engine.Engine): each resource is one unit, taken for a window by a claim,
// and -- as in freebusy -- a hold and a booking made with the same key are
// the same claim, so booking after holding takes the held rooms, not more.
// What freebusy does on its own -- an operator cancelling or moving a
// reservation, a hold lapsing -- is freebusy_outside.go's.

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/shokki-engineering/aperio/internal/engine"
)

// Freebusy is engine.Engine in memory.
type Freebusy struct {
	mu     sync.Mutex
	claims map[string]claim       // by reservation name, active only
	keys   map[string]string      // booking key -> reservation name
	notes  map[string]engine.Note // by reservation name
	ended  map[string]bool        // reservations cancelled, here or outside
	moved  map[string]string      // reservation -> the one it moved to
	books  int                    // Book calls that made a claim of their own

	// FailOn makes Book fail as unavailable for keys ending in it.
	FailOn string
}

// claim is what one hold or reservation takes.
type claim struct {
	resources []string
	window    engine.Window
	booked    bool // made a reservation by Book, not only held
	expire    time.Time
}

// NewFreebusy is an empty Freebusy: everything free.
func NewFreebusy() *Freebusy {
	return &Freebusy{claims: map[string]claim{}, keys: map[string]string{}, notes: map[string]engine.Note{},
		ended: map[string]bool{}, moved: map[string]string{}}
}

// free is resources no claim takes during w; the caller holds mu.
func (f *Freebusy) free(resources []string, w engine.Window) []string {
	var out []string
	for _, r := range resources {
		taken := false
		for _, c := range f.claims {
			for _, cr := range c.resources {
				taken = taken || (cr == r && c.window.Start.Before(w.End) && w.Start.Before(c.window.End))
			}
		}
		if !taken {
			out = append(out, r)
		}
	}
	return out
}

func (f *Freebusy) Free(_ context.Context, resources []string, w engine.Window, quantity int32) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.free(resources, w)) >= int(quantity), nil
}

// take claims b, or returns the claim its key already made.
func (f *Freebusy) take(b engine.Booking, book bool) (name string, made bool, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if name, ok := f.keys[b.Key]; ok {
		if c, live := f.claims[name]; live && book {
			c.booked = true
			f.claims[name] = c
		}
		return name, false, nil
	}
	free := f.free(b.Resources, b.Window)
	if len(free) < int(b.Quantity) {
		return "", false, engine.ErrUnavailable
	}
	name = "reservations/fake-" + b.Key
	f.claims[name] = claim{resources: free[:b.Quantity], window: b.Window, booked: book, expire: time.Now().Add(15 * time.Minute)}
	f.keys[b.Key] = name
	return name, true, nil
}

func (f *Freebusy) Hold(_ context.Context, b engine.Booking) (engine.Hold, error) {
	name, _, err := f.take(b, false)
	return engine.Hold{Name: name, Expire: time.Now().Add(15 * time.Minute)}, err
}

func (f *Freebusy) Held(_ context.Context, key string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	name, ok := f.keys[key]
	_, active := f.claims[name]
	return ok && active, nil
}

func (f *Freebusy) Book(_ context.Context, b engine.Booking) (string, error) {
	if f.FailOn != "" && strings.HasSuffix(b.Key, f.FailOn) {
		return "", engine.ErrUnavailable
	}
	name, made, err := f.take(b, true)
	if made {
		f.mu.Lock()
		f.books++
		f.mu.Unlock()
	}
	return name, err
}

func (f *Freebusy) Cancel(_ context.Context, reservation, _ string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, live := f.claims[reservation]; live {
		f.ended[reservation] = true
	}
	delete(f.claims, reservation)
	delete(f.notes, reservation)
	return nil
}

func (f *Freebusy) Note(_ context.Context, reservation string, n engine.Note) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.notes[reservation] = n
	return nil
}

func (f *Freebusy) ExtendHold(_ context.Context, hold string, d time.Duration) (engine.Hold, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	c, live := f.claims[hold]
	if !live || c.booked {
		return engine.Hold{}, engine.ErrLapsed
	}
	c.expire = time.Now().Add(d)
	f.claims[hold] = c
	return engine.Hold{Name: hold, Expire: c.expire}, nil
}

func (f *Freebusy) ReleaseHold(_ context.Context, hold string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if c, live := f.claims[hold]; live && !c.booked {
		delete(f.claims, hold)
	}
	return nil
}

func (f *Freebusy) Find(_ context.Context, key string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	name := f.keys[key]
	if c, live := f.claims[name]; live && c.booked {
		return name, nil
	}
	return "", nil
}

// NoteOf is the note on reservation's calendars.
func (f *Freebusy) NoteOf(reservation string) engine.Note {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.notes[reservation]
}

// Active is how many claims are held.
func (f *Freebusy) Active() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.claims)
}

// Booked is how many Book calls made a claim of their own rather than
// taking a hold's.
func (f *Freebusy) Booked() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.books
}
