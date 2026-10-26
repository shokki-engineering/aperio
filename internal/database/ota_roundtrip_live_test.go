// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package database_test

// Live round-trip tests of the generated OTA repositories, over GORM and over
// Hasura: a hotel with a value-object graph whose enum's stored label Postgres
// could not hold whole (naming.EnumLabel fits it to 63 bytes), and a rate
// plan and a rate nested under it, whose parents are found by name. They pin
// that the generated layers agree with the Prisma-migrated database.
//
//	APERIO_TEST_DATABASE_URL="host=127.0.0.1 port=5433 user=postgres password=postgrespassword dbname=ota sslmode=disable" \
//	APERIO_TEST_GRAPHQL_URL=http://localhost:3281/graphql \
//	  go test ./internal/database/ -run LiveOTA -v
//
// Without APERIO_TEST_DATABASE_URL the tests skip; without
// APERIO_TEST_GRAPHQL_URL only the GORM backend runs.

import (
	"context"
	"net/url"
	"os"
	"testing"

	"buf.build/gen/go/shokki-engineering/protobuf-ota/protocolbuffers/go/protobuf/opentravel/hotel/hotel/v1"
	"buf.build/gen/go/shokki-engineering/protobuf-ota/protocolbuffers/go/protobuf/opentravel/hotel/hotel/v1/codes"
	"buf.build/gen/go/shokki-engineering/protobuf-ota/protocolbuffers/go/protobuf/opentravel/hotel/hotel/v1/types"
	"buf.build/gen/go/shokki-engineering/protobuf-ota/protocolbuffers/go/protobuf/opentravel/hotel/revenue/rate/v1"
	"buf.build/gen/go/shokki-engineering/protobuf-ota/protocolbuffers/go/protobuf/opentravel/hotel/revenue/rate_plan/v1"
	"github.com/shokki-engineering/aperio/internal/database/hasura/otaql"
	"github.com/shokki-engineering/aperio/internal/database/repository/ota/hotel"
	"github.com/shokki-engineering/aperio/internal/database/repository/ota/rate"
	"github.com/shokki-engineering/aperio/internal/database/repository/ota/rateplan"
	"github.com/shokki-engineering/aperio/internal/database/repository/repox"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// liveConns are the backends to run against, by name: gorm always, hasura
// when APERIO_TEST_GRAPHQL_URL is set.
func liveConns(t *testing.T) map[string]repox.Conn {
	t.Helper()
	dsn := os.Getenv("APERIO_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("APERIO_TEST_DATABASE_URL is not set")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]repox.Conn{"gorm": {Gorm: db}}
	if raw := os.Getenv("APERIO_TEST_GRAPHQL_URL"); raw != "" {
		u, err := url.Parse(raw)
		if err != nil {
			t.Fatal(err)
		}
		svc, err := otaql.Connect(u)
		if err != nil {
			t.Fatal(err)
		}
		out["hasura"] = repox.Conn{GraphQL: svc}
	}
	return out
}

// staffEntry's stored label is longer than Postgres allows, so it is stored
// fitted, and must read back as itself.
const staffEntry = codes.HotelAmenityCode_HOTEL_AMENITY_CODE_HOTEL_STAFF_ALLOWED_TO_ENTER_A_HOTEL_ROOM_ONCE_A_GUEST_HAS_CHECKED_IN

func TestLiveOTARoundTrip(t *testing.T) {
	ctx := context.Background()
	for backend, conn := range liveConns(t) {
		t.Run(backend, func(t *testing.T) {
			run := repox.NewULID()
			hotelName := "hotels/rt-" + run

			t.Run("hotel keeps a fitted enum label", func(t *testing.T) {
				hotels := hotel.New(conn).Hotels
				h, err := hotels.Create(ctx, &hotelv1.Hotel{
					Name:         hotelName,
					CurrencyCode: "EUR",
					HotelInfo: &types.HotelInfo{
						Services: []*types.HotelInfo_Service{{Code: staffEntry}},
					},
				})
				if err != nil {
					t.Fatal(err)
				}
				got, err := hotels.Get(ctx, h.GetName())
				if err != nil {
					t.Fatal(err)
				}
				svcs := got.GetHotelInfo().GetServices()
				if got.GetCurrencyCode() != "EUR" || len(svcs) != 1 || svcs[0].GetCode() != staffEntry {
					t.Fatalf("read back currency %q, services %v", got.GetCurrencyCode(), svcs)
				}
			})

			t.Run("rate plan and rate find their parents by name", func(t *testing.T) {
				planName := hotelName + "/ratePlans/bar-" + run
				plan, err := rateplan.New(conn).RatePlans.Create(ctx, hotelName, &rate_planv1.RatePlan{
					Name: planName, RatePlanId: "BAR",
				})
				if err != nil {
					t.Fatal(err)
				}
				if plan.GetName() != planName {
					t.Fatalf("rate plan named %q, want %q", plan.GetName(), planName)
				}
				rateName := planName + "/rates/d-" + run
				r, err := rate.New(conn).Rates.Create(ctx, planName, &ratev1.Rate{Name: rateName})
				if err != nil {
					t.Fatal(err)
				}
				got, err := rate.New(conn).Rates.Get(ctx, r.GetName())
				if err != nil {
					t.Fatal(err)
				}
				if got.GetName() != rateName {
					t.Fatalf("rate named %q, want %q", got.GetName(), rateName)
				}
				page, _, err := rate.New(conn).Rates.List(ctx, planName, repox.ListInput{})
				if err != nil {
					t.Fatal(err)
				}
				if len(page) != 1 || page[0].GetName() != rateName {
					t.Fatalf("rates under %s: %v", planName, page)
				}
			})
		})
	}
}
