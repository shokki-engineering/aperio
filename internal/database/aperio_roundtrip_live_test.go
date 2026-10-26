// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package database_test

// Live round-trip tests of the generated aperio repositories, over GORM and
// over Hasura, against the Prisma-migrated database: a venue, a reservation
// whose OpenTravel room stays and guests, dietary requirements and special
// requests are stored as documents, and a payment in a foreign currency with
// a decimal exchange rate. Each is written, read back and compared whole, so
// a field a converter drops fails the test rather than going quietly missing.
//
//	APERIO_TEST_DATABASE_URL="host=127.0.0.1 port=5433 user=postgres password=postgrespassword dbname=aperio sslmode=disable" \
//	APERIO_TEST_GRAPHQL_URL=http://localhost:3281/graphql \
//	  go test ./internal/database/ -run LiveAperio -v
//
// Without APERIO_TEST_DATABASE_URL the tests skip; without
// APERIO_TEST_GRAPHQL_URL only the GORM backend runs.

import (
	"context"
	"net/url"
	"os"
	"testing"

	"buf.build/gen/go/shokki-engineering/protobuf-ota/protocolbuffers/go/protobuf/opentravel/hotel/booking/reservation/v1/types"
	"github.com/shokki-engineering/aperio/internal/database/hasura/aperioql"
	"github.com/shokki-engineering/aperio/internal/database/repository/aperio/finance"
	"github.com/shokki-engineering/aperio/internal/database/repository/aperio/guest"
	"github.com/shokki-engineering/aperio/internal/database/repository/aperio/stay"
	"github.com/shokki-engineering/aperio/internal/database/repository/aperio/venue"
	"github.com/shokki-engineering/aperio/internal/database/repository/repox"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/finance/v1/financepbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/guest/v1/guestpbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/stay/v1/staypbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/type/v1/typepbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/venue/v1/venuepbv1"
	"google.golang.org/genproto/googleapis/type/date"
	"google.golang.org/genproto/googleapis/type/decimal"
	"google.golang.org/genproto/googleapis/type/money"
	"google.golang.org/genproto/googleapis/type/postaladdress"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/testing/protocmp"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/google/go-cmp/cmp"
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
		svc, err := aperioql.Connect(u)
		if err != nil {
			t.Fatal(err)
		}
		out["hasura"] = repox.Conn{GraphQL: svc}
	}
	return out
}

// same fails t when got differs from want in any field but those the server
// sets.
func same(t *testing.T, what string, want, got proto.Message) {
	t.Helper()
	var server []protoreflect.Name
	fields := got.ProtoReflect().Descriptor().Fields()
	for _, n := range []protoreflect.Name{"name", "create_time", "update_time", "etag", "state", "base_amount"} {
		if fields.ByName(n) != nil {
			server = append(server, n)
		}
	}
	if d := cmp.Diff(want, got, protocmp.Transform(), protocmp.IgnoreFields(got, server...)); d != "" {
		t.Errorf("%s read back differs (-want +got):\n%s", what, d)
	}
}

func TestLiveAperioRoundTrip(t *testing.T) {
	ctx := context.Background()
	for backend, conn := range liveConns(t) {
		t.Run(backend, func(t *testing.T) {
			run := repox.NewULID()

			venueIn := &venuepbv1.Venue{
				DisplayName:  "Sea Breeze Resort " + run,
				Kind:         venuepbv1.VenueKind_VENUE_KIND_STAYS_AND_EVENTS,
				Address:      &postaladdress.PostalAddress{RegionCode: "IN", Locality: "Mangaluru", AddressLines: []string{"1 Beach Road"}},
				TimeZone:     "Asia/Kolkata",
				CurrencyCode: "INR",
				Gst:          &typepbv1.Gst{Gstin: "29ABCDE1234F1Z5", LegalDisplayName: "Sea Breeze Hospitality", GstStateCode: "29"},
			}
			v, err := venue.New(conn).Venues.Create(ctx, venueIn)
			if err != nil {
				t.Fatalf("create venue: %v", err)
			}
			got, err := venue.New(conn).Venues.Get(ctx, v.GetName())
			if err != nil {
				t.Fatalf("get venue: %v", err)
			}
			same(t, "venue", venueIn, got)

			contactIn := &guestpbv1.Contact{
				Kind:                guestpbv1.ContactKind_CONTACT_KIND_PERSON,
				DisplayName:         "John Doe",
				Phones:              []*typepbv1.Phone{{Number: "+919876543210", Kind: typepbv1.PhoneKind_PHONE_KIND_WHATSAPP}},
				Emails:              []string{"ananya@example.com"},
				DietaryRequirements: []*typepbv1.DietaryRequirement{{Diet: typepbv1.Diet_DIET_JAIN}},
				Tags:                []string{"vip"},
			}
			c, err := guest.New(conn).Contacts.Create(ctx, contactIn)
			if err != nil {
				t.Fatalf("create contact: %v", err)
			}
			gotContact, err := guest.New(conn).Contacts.Get(ctx, c.GetName())
			if err != nil {
				t.Fatalf("get contact: %v", err)
			}
			same(t, "contact", contactIn, gotContact)

			resIn := &staypbv1.Reservation{
				Contact:          c.GetName(),
				Channel:          typepbv1.Channel_CHANNEL_BOOKING_COM,
				ChannelReference: "BDC-" + run,
				ArrivalDate:      &date.Date{Year: 2026, Month: 12, Day: 24},
				DepartureDate:    &date.Date{Year: 2026, Month: 12, Day: 27},
				Adults:           2,
				Children:         1,
				Rooms:            []*staypbv1.RoomLine{{Space: v.GetName() + "/spaces/deluxe", Rooms: 1, RatePlanCode: "BAR"}},
				RoomStays:        []*types.RoomStay{{MarketCode: "LEISURE", PromotionCode: "XMAS"}},
				ResGuests:        []*types.ResGuest{{ResGuestRph: "1", Age: proto.Int32(34), Vip: proto.Bool(true)}},
				DietaryRequirements: []*typepbv1.DietaryRequirement{
					{Diet: typepbv1.Diet_DIET_JAIN, GuestDisplayName: "J. Doe"},
					{Diet: typepbv1.Diet_DIET_VEGAN, Allergens: []typepbv1.Allergen{typepbv1.Allergen_ALLERGEN_TREE_NUT}},
				},
				SpecialRequests: []*typepbv1.SpecialRequest{{
					Summary:  "Cot in the room",
					Category: typepbv1.RequestCategory_REQUEST_CATEGORY_ROOM,
					Ota:      &types.SpecialRequest{DisplayName: "Cot", LanguageCode: "en"},
				}},
				Total: &money.Money{CurrencyCode: "INR", Units: 42000},
			}
			r, err := stay.New(conn).Reservations.Create(ctx, v.GetName(), resIn)
			if err != nil {
				t.Fatalf("create reservation: %v", err)
			}
			gotRes, err := stay.New(conn).Reservations.Get(ctx, r.GetName())
			if err != nil {
				t.Fatalf("get reservation: %v", err)
			}
			same(t, "reservation", resIn, gotRes)

			payIn := &financepbv1.Payment{
				Contact:      resIn.GetContact(),
				Booking:      r.GetName(),
				Kind:         financepbv1.PaymentKind_PAYMENT_KIND_RECEIPT,
				Method:       financepbv1.PaymentMethod_PAYMENT_METHOD_BANK_TRANSFER,
				Amount:       &money.Money{CurrencyCode: "USD", Units: 150, Nanos: 500000000},
				ExchangeRate: &decimal.Decimal{Value: "83.4567"},
				Reference:    "UTR-" + run,
			}
			p, err := finance.New(conn).Payments.Create(ctx, v.GetName(), payIn)
			if err != nil {
				t.Fatalf("create payment: %v", err)
			}
			gotPay, err := finance.New(conn).Payments.Get(ctx, p.GetName())
			if err != nil {
				t.Fatalf("get payment: %v", err)
			}
			same(t, "payment", payIn, gotPay)
		})
	}
}
