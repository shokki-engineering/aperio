// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package sales_test

// A live test of ShareQuote: Unimplemented with no link signer; with one,
// the guest site's page for the quote with a token that verifies to that
// quote alone, for 14 days. Run as test/live says.

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/shokki-engineering/aperio/internal/auth"
	"github.com/shokki-engineering/aperio/internal/database/repository/aperio/guest"
	"github.com/shokki-engineering/aperio/internal/database/repository/aperio/venue"
	"github.com/shokki-engineering/aperio/internal/service/pricing"
	"github.com/shokki-engineering/aperio/internal/service/sales"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/guest/v1/guestpbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/sales/v1/salespbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/type/v1/typepbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/venue/v1/venuepbv1"
	"github.com/shokki-engineering/aperio/test/live"
	"google.golang.org/genproto/googleapis/type/date"
	"google.golang.org/genproto/googleapis/type/decimal"
	"google.golang.org/genproto/googleapis/type/postaladdress"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestLiveShareQuote(t *testing.T) {
	ctx, conn := context.Background(), live.Conn(t)
	must := func(err error, what string) {
		t.Helper()
		if err != nil {
			t.Fatalf("%s: %v", what, err)
		}
	}
	venues, catalogRepos := venue.New(conn), live.CatalogRepositories(conn)
	prices := pricing.New(live.PricingRepositories(conn), catalogRepos, venues.Venues)
	svc := sales.New(live.SalesRepositories(conn), venues.Venues, catalogRepos.Menus, prices, nil, nil)
	v, err := venues.Venues.Create(ctx, &venuepbv1.Venue{DisplayName: "Share Hall " + fmt.Sprint(time.Now().UnixNano()),
		Kind: venuepbv1.VenueKind_VENUE_KIND_EVENTS, TimeZone: "Asia/Kolkata", CurrencyCode: "INR",
		Address: &postaladdress.PostalAddress{RegionCode: "IN"}, Gst: &typepbv1.Gst{Gstin: "29ABCDE1234F1Z5", LegalDisplayName: "Test", GstStateCode: "29"}})
	must(err, "create venue")
	c, err := guest.New(conn).Contacts.Create(ctx, &guestpbv1.Contact{Kind: guestpbv1.ContactKind_CONTACT_KIND_PERSON, DisplayName: "John Doe"})
	must(err, "create contact")
	day := &date.Date{Year: 2027, Month: 12, Day: 12}
	q, err := svc.CreateQuote(ctx, &salespbv1.CreateQuoteRequest{Parent: v.GetName(), Quote: &salespbv1.Quote{
		DisplayName: "Doe reception", Contact: c.GetName(), Guests: 100, Dates: &typepbv1.DateRange{StartDate: day, EndDate: day},
		Lines: []*salespbv1.QuoteLine{{Kind: salespbv1.LineKind_LINE_KIND_SERVICE, Description: "Floral decor", Quantity: 1,
			UnitPrice: inr(50000), Tax: &typepbv1.Tax{HsnSacCode: "998596", GstRate: &decimal.Decimal{Value: "18"}}}},
	}})
	must(err, "create quote")

	req := &salespbv1.ShareQuoteRequest{Name: q.GetName()}
	if _, err := svc.ShareQuote(ctx, req); status.Code(err) != codes.Unimplemented {
		t.Errorf("share with no signer = %v; want Unimplemented", err)
	}
	links := auth.NewLinks([]byte(strings.Repeat("s", 32)), nil)
	svc.ShareLinksWith(links, "https://guest.example")
	got, err := svc.ShareQuote(ctx, req)
	must(err, "share")
	page, token, ok := strings.Cut(got.GetUrl(), "?link=")
	if !ok || page != "https://guest.example/"+q.GetName() {
		t.Fatalf("url %q; want the guest site's page for %s with ?link=", got.GetUrl(), q.GetName())
	}
	if granted, err := links.VerifyLink(token); err != nil || granted != q.GetName() {
		t.Errorf("the token grants %q, %v; want %s", granted, err, q.GetName())
	}
	if left := time.Until(got.GetExpireTime().AsTime()); left < 14*24*time.Hour-time.Minute || left > 14*24*time.Hour {
		t.Errorf("the link lapses in %v; want 14 days", left)
	}
	if _, err := svc.ShareQuote(ctx, &salespbv1.ShareQuoteRequest{Name: v.GetName() + "/quotes/missing"}); status.Code(err) != codes.NotFound {
		t.Errorf("sharing a missing quote = %v; want NotFound", err)
	}
}
