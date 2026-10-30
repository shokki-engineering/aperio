// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package database_test

// Live tests of the filter shapes the summaries and reports send -- an enum
// !=, a timestamp range, filterx.In, ListByKeys, a List across every venue
// (venues/-) -- over both backends, so the GraphQL repositories are held to
// what the GORM ones do. Run as aperio_roundtrip_live_test.go's are: without
// APERIO_TEST_DATABASE_URL they skip, without APERIO_TEST_GRAPHQL_URL only
// GORM runs.

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/shokki-engineering/aperio/internal/database/gorm/filterx"
	"github.com/shokki-engineering/aperio/internal/database/repository/aperio/finance"
	"github.com/shokki-engineering/aperio/internal/database/repository/aperio/guest"
	"github.com/shokki-engineering/aperio/internal/database/repository/aperio/venue"
	"github.com/shokki-engineering/aperio/internal/database/repository/repox"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/finance/v1/financepbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/guest/v1/guestpbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/type/v1/typepbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/venue/v1/venuepbv1"
	"google.golang.org/genproto/googleapis/type/money"
	"google.golang.org/genproto/googleapis/type/postaladdress"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// names are the resource names of ms, sorted.
func names[M interface{ GetName() string }](ms []M) []string {
	out := make([]string, 0, len(ms))
	for _, m := range ms {
		out = append(out, m.GetName())
	}
	slices.Sort(out)
	return out
}

// expect fails t, naming the shape, when got (or err) is not want.
func expect(t *testing.T, shape string, got []string, err error, want ...string) {
	t.Helper()
	slices.Sort(want)
	switch {
	case err != nil:
		t.Errorf("%s: %v", shape, err)
	case !slices.Equal(got, want):
		t.Errorf("%s: got %v, want %v", shape, got, want)
	}
}

func TestLiveSummaryFilters(t *testing.T) {
	ctx := context.Background()
	for backend, conn := range liveConns(t) {
		t.Run(backend, func(t *testing.T) {
			run := repox.NewULID()
			v, err := venue.New(conn).Venues.Create(ctx, &venuepbv1.Venue{DisplayName: "Doe Filters " + run,
				Kind: venuepbv1.VenueKind_VENUE_KIND_STAYS, TimeZone: "Asia/Kolkata", CurrencyCode: "INR",
				Address: &postaladdress.PostalAddress{RegionCode: "IN", Locality: "Mangaluru", AddressLines: []string{"1 Beach Road"}},
				Gst:     &typepbv1.Gst{Gstin: "29ABCDE1234F1Z5", LegalDisplayName: "Doe Hospitality", GstStateCode: "29"}})
			if err != nil {
				t.Fatalf("create venue: %v", err)
			}
			contacts := guest.New(conn).Contacts
			var people []string
			for _, n := range []string{"John Doe", "Jane Doe"} {
				c, err := contacts.Create(ctx, &guestpbv1.Contact{Kind: guestpbv1.ContactKind_CONTACT_KIND_PERSON, DisplayName: n + " " + run})
				if err != nil {
					t.Fatalf("create contact: %v", err)
				}
				people = append(people, c.GetName())
			}

			// Three payments an hour apart: succeeded, pending, failed.
			payments := finance.New(conn).Payments
			t0 := time.Date(2026, 11, 2, 9, 0, 0, 0, time.UTC)
			states := []financepbv1.Payment_State{financepbv1.Payment_STATE_SUCCEEDED, financepbv1.Payment_STATE_PENDING, financepbv1.Payment_STATE_FAILED}
			var paid []string
			for i, st := range states {
				p, err := payments.Create(ctx, v.GetName(), &financepbv1.Payment{Contact: people[0],
					Kind: financepbv1.PaymentKind_PAYMENT_KIND_RECEIPT, Method: financepbv1.PaymentMethod_PAYMENT_METHOD_BANK_TRANSFER,
					Amount: &money.Money{CurrencyCode: "INR", Units: int64(1000 * (i + 1))}, Reference: "UTR-" + run,
					ReceiveTime: timestamppb.New(t0.Add(time.Duration(i) * time.Hour))})
				if err != nil {
					t.Fatalf("create payment: %v", err)
				}
				p.State = st
				if _, err := payments.UpdateOutputOnly(ctx, p, []string{"state"}); err != nil {
					t.Fatalf("set payment state: %v", err)
				}
				paid = append(paid, p.GetName())
			}
			at := func(d time.Duration) string { return `"` + t0.Add(d).Format(time.RFC3339) + `"` }
			list := func(parent string, in repox.ListInput) ([]string, error) {
				in.PageSize = 50
				got, _, err := payments.List(ctx, parent, in)
				return names(got), err
			}

			got, err := list(v.GetName(), repox.ListInput{Filter: "state != PENDING"})
			expect(t, "enum !=", got, err, paid[0], paid[2])
			got, err = list(v.GetName(), repox.ListInput{Filter: "state != PENDING AND state != FAILED"})
			expect(t, "enum != twice", got, err, paid[0])
			got, err = list(v.GetName(), repox.ListInput{Filter: "state = SUCCEEDED AND receive_time >= " + at(0)})
			expect(t, "enum = and timestamp >=", got, err, paid[0])
			got, err = list(v.GetName(), repox.ListInput{Filter: "receive_time >= " + at(30*time.Minute) + " AND receive_time < " + at(150*time.Minute)})
			expect(t, "timestamp range", got, err, paid[1], paid[2])
			got, err = list(v.GetName(), repox.ListInput{Where: []filterx.Condition{filterx.In("name", paid[0], paid[2])}})
			expect(t, "filterx.In on name", got, err, paid[0], paid[2])
			got, err = list(v.GetName(), repox.ListInput{Filter: "state != PENDING", Where: []filterx.Condition{filterx.In("name", paid[1], paid[2])}})
			expect(t, "filterx.In with a filter", got, err, paid[2])
			got, err = list("venues/-", repox.ListInput{Filter: `reference = "UTR-` + run + `"`})
			expect(t, "venues/- with a filter", got, err, paid...)

			byKeys, err := payments.ListByKeys(ctx, "name", paid[:2], repox.ListInput{})
			expect(t, "ListByKeys on name across venues", names(byKeys), err, paid[0], paid[1])
			byKeys, err = payments.ListByKeys(ctx, "contact", people, repox.ListInput{Filter: "state != SUCCEEDED"})
			expect(t, "ListByKeys on a reference, with a filter", names(byKeys), err, paid[1], paid[2])

			cs, _, err := contacts.List(ctx, repox.ListInput{PageSize: 10, Where: []filterx.Condition{filterx.In("name", people...)}})
			expect(t, "contacts filterx.In", names(cs), err, people...)
			cs, err = contacts.ListByKeys(ctx, "name", people, repox.ListInput{})
			expect(t, "contacts ListByKeys", names(cs), err, people...)
		})
	}
}
