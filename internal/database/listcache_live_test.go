// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package database_test

// The venue list cache on a real Redis, over both backends, as freebusy's
// reservation list cache is tested: a page is served from the cache until a
// write through the connection's repositories reaches the collection, and
// then never again.
//
//	docker run -d --rm --name aperio-redis -p 127.0.0.1:6390:6379 redis:7
//	APERIO_TEST_REDIS_ADDR=127.0.0.1:6390 APERIO_TEST_DATABASE_URL=... \
//	APERIO_TEST_GRAPHQL_URL=http://localhost:3280/graphql \
//	  go test ./internal/database/ -run LiveListCache -v

import (
	"context"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/shokki-engineering/aperio/internal/database"
	"github.com/shokki-engineering/aperio/internal/database/repository/aperio/venue"
	"github.com/shokki-engineering/aperio/internal/database/repository/repox"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/type/v1/typepbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/venue/v1/venuepbv1"
	"github.com/shokki-engineering/aperio/test/live"
	"github.com/the-protobuf-project/runtime-go/cache"
	"github.com/the-protobuf-project/runtime-go/cache/redis"
	"google.golang.org/genproto/googleapis/type/postaladdress"
)

func TestLiveListCache(t *testing.T) {
	ctx := context.Background()
	client, err := redis.NewClient(ctx, redis.Config{Address: live.Env(t, live.RedisAddr)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })

	for backend, raw := range liveConns(t) {
		t.Run(backend, func(t *testing.T) {
			run := fmt.Sprintf("%s-%d", backend, time.Now().UnixNano())
			// A prefix of its own, so nothing another run cached is read here.
			provider := redis.New(client, cache.Config{Prefix: "listcache-test-" + run, RequireTTL: true})
			conn := &database.Connection{PgSQLConn: raw.Gorm, Hasura: raw.GraphQL, Provider: database.ProviderGorm}
			if raw.GraphQL != nil {
				conn.Provider = database.ProviderHasura
			}
			if err := conn.UseCaches(ctx, provider); err != nil {
				t.Fatal(err)
			}
			// Repositories built from the connection report their writes; one
			// built from a bare Conn is a writer the cache never hears from.
			observed := conn.Caches.Venue(venue.New(conn.Repox())).Venues
			unseen := venue.New(raw).Venues

			// Every venue this run makes is called tag, so a List can be
			// narrowed to them.
			tag := "Listcache " + run
			var made []string
			create := func(repo venue.VenueRepository) string {
				t.Helper()
				v, err := repo.Create(ctx, &venuepbv1.Venue{
					DisplayName:  tag,
					Kind:         venuepbv1.VenueKind_VENUE_KIND_STAYS,
					Address:      &postaladdress.PostalAddress{RegionCode: "IN", Locality: "Mangaluru"},
					TimeZone:     "Asia/Kolkata",
					CurrencyCode: "INR",
					Gst:          &typepbv1.Gst{Gstin: "29ABCDE1234F1Z5", LegalDisplayName: "Listcache", GstStateCode: "29"},
				})
				if err != nil {
					t.Fatal(err)
				}
				made = append(made, v.GetName())
				return v.GetName()
			}
			t.Cleanup(func() {
				for _, n := range made {
					_ = unseen.Delete(ctx, n)
				}
			})
			want := func(names ...string) {
				t.Helper()
				page, _, err := observed.List(ctx, repox.ListInput{PageSize: 50, Filter: `display_name = "` + tag + `"`})
				if err != nil {
					t.Fatal(err)
				}
				var got []string
				for _, v := range page {
					got = append(got, v.GetName())
				}
				slices.Sort(got)
				names = slices.Clone(names)
				slices.Sort(names)
				if !slices.Equal(got, names) {
					t.Fatalf("List = %v, want %v", got, names)
				}
			}

			a, b := create(observed), create(observed)
			want(a, b)

			// Behind the cache's back: the cached page is what is served.
			hidden := create(unseen)
			want(a, b)

			// A write the repositories report moves the collection on, and the
			// next List is the database's -- the hidden venue included.
			got, err := observed.Get(ctx, a)
			if err != nil {
				t.Fatal(err)
			}
			got.TimeZone = "Asia/Calcutta"
			if _, err := observed.Update(ctx, got, []string{"time_zone"}); err != nil {
				t.Fatal(err)
			}
			want(a, b, hidden)

			if err := observed.Delete(ctx, b); err != nil {
				t.Fatal(err)
			}
			want(a, hidden)

			// An observer added later hears the writes too.
			heard := 0
			conn.Observe(observerFunc(func(string) { heard++ }))
			c := create(observed)
			want(a, hidden, c)
			if heard != 1 {
				t.Fatalf("observer heard %d writes, want 1", heard)
			}
		})
	}
}

// observerFunc is a repox.ChangeObserver from a function.
type observerFunc func(name string)

func (f observerFunc) Changed(_ context.Context, name string) { f(name) }
