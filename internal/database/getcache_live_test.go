// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package database_test

// The generated read-through Get cache (venuecache.NewCachedVenueStore) on a
// real Redis: a Get is served from Redis, so a row changed behind its back
// stays as it was cached, until an Update through it refreshes the entry.
// It sits on the GORM store, beneath the repositories, which take no store
// -- so nothing in the server wires it yet.
//
//	APERIO_TEST_REDIS_ADDR=127.0.0.1:6390 APERIO_TEST_DATABASE_URL=... \
//	  go test ./internal/database/ -run LiveGetCache -v

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/shokki-engineering/aperio/internal/database/cache/aperio/venuecache"
	"github.com/shokki-engineering/aperio/internal/database/gorm/aperio/venue"
	"github.com/shokki-engineering/aperio/test/live"
	"github.com/the-protobuf-project/runtime-go/cache"
	"github.com/the-protobuf-project/runtime-go/cache/redis"
)

func TestLiveGetCache(t *testing.T) {
	ctx := context.Background()
	client, err := redis.NewClient(ctx, redis.Config{Address: live.Env(t, live.RedisAddr)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })
	db := live.Conn(t).Gorm

	run := fmt.Sprintf("getcache-%d", time.Now().UnixNano())
	provider := redis.New(client, cache.Config{Prefix: "getcache-test-" + run, RequireTTL: true})
	cdb, err := venuecache.OpenVenueCache(ctx, provider)
	if err != nil {
		t.Fatal(err)
	}
	store := venue.NewVenueStore(db)
	cached, err := venuecache.NewCachedVenueStore(ctx, store, cdb)
	if err != nil {
		t.Fatal(err)
	}

	m := &venue.Venue{
		ID: run, Name: "venues/" + run, DisplayName: "Getcache " + run, Kind: venue.VenueKind("STAYS"),
		Address: json.RawMessage(`{"regionCode":"IN"}`), TimeZone: "Asia/Kolkata", CurrencyCode: "INR",
		Gst: json.RawMessage(`{"gstin":"29ABCDE1234F1Z5"}`),
	}
	if err := cached.Create(ctx, m); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.DeleteByID(ctx, run) })

	displayName := func() string {
		t.Helper()
		got, err := cached.GetByID(ctx, run)
		if err != nil {
			t.Fatal(err)
		}
		return got.DisplayName
	}
	if got := displayName(); got != m.DisplayName {
		t.Fatalf("Get = %q, want %q", got, m.DisplayName)
	}

	// Behind the cache's back: Get is served from Redis, not the database.
	if err := db.Model(&venue.Venue{}).Where("id = ?", run).Update("display_name", "Changed behind").Error; err != nil {
		t.Fatal(err)
	}
	if got := displayName(); got != m.DisplayName {
		t.Fatalf("Get = %q after a write the cache did not see, want the cached %q", got, m.DisplayName)
	}
	if keys := client.Unwrap().Keys(ctx, "getcache-test-"+run+"*").Val(); len(keys) == 0 {
		t.Fatal("no key under the run's prefix: Get was not served from Redis")
	}

	// An Update through the cache refreshes the entry.
	fresh, err := store.GetByID(ctx, run)
	if err != nil {
		t.Fatal(err)
	}
	fresh.DisplayName = "Updated " + run
	if err := cached.Update(ctx, fresh); err != nil {
		t.Fatal(err)
	}
	if got := displayName(); got != fresh.DisplayName {
		t.Fatalf("Get = %q after Update, want %q", got, fresh.DisplayName)
	}

	// A delete through it drops the entry.
	if err := cached.DeleteByID(ctx, run); err != nil {
		t.Fatal(err)
	}
	if _, err := cached.GetByID(ctx, run); err == nil {
		t.Fatal("Get found the venue after DeleteByID")
	}
}
