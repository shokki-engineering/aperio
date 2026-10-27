// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package database

import (
	"context"
	"testing"

	"github.com/shokki-engineering/aperio/internal/database/repository/aperio/venue"
)

// TestOpenCacheProviderDisabled pins the documented "no cache configured"
// path: a nil Provider and no error, so a caller reads the database rather
// than treating an intentionally-off cache as a startup failure.
func TestOpenCacheProviderDisabled(t *testing.T) {
	// config.Get() reads the embedded defaults, where cache.enabled is false.
	p, err := OpenCacheProvider(context.Background())
	if err != nil {
		t.Fatalf("OpenCacheProvider with cache disabled: %v", err)
	}
	if p != nil {
		t.Fatalf("OpenCacheProvider = %v, want nil when disabled", p)
	}
}

// TestCachesOffWrapNothing: with the cache off, wrapping hands the
// repositories back as they were, and a connection's writes reach no cache.
func TestCachesOffWrapNothing(t *testing.T) {
	var c Caches
	r := venue.Repositories{}
	if got := c.Venue(r); got != r {
		t.Fatalf("Venue wrapped %v with the cache off", got)
	}
	conn := &Connection{Provider: ProviderGorm}
	if conn.CacheProvider() != nil {
		t.Fatal("CacheProvider set without UseCaches")
	}
	conn.Repox().Changes.Changed(context.Background(), "venues/x")
}

// TestObserveHearsWrites: an observer added after a Repox handle was taken
// still hears the writes reported through it, after Changes.
func TestObserveHearsWrites(t *testing.T) {
	conn := &Connection{Provider: ProviderGorm}
	var heard []string
	conn.Changes = observerFunc(func(name string) { heard = append(heard, "changes:"+name) })
	rc := conn.Repox()
	conn.Observe(observerFunc(func(name string) { heard = append(heard, "observer:"+name) }))
	rc.Changes.Changed(context.Background(), "venues/a")
	if len(heard) != 2 || heard[0] != "changes:venues/a" || heard[1] != "observer:venues/a" {
		t.Fatalf("heard %v", heard)
	}
}

// observerFunc is a ChangeObserver from a function.
type observerFunc func(name string)

func (f observerFunc) Changed(_ context.Context, name string) { f(name) }
