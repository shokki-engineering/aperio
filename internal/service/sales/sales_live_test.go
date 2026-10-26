// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package sales_test

// Live tests of quoting over the dev database, through the catalog and
// pricing services: a ₹1,200 plate
// menu at 5% GST, +15% in a wedding season that wraps the new year, priced
// on its own and on a quote; the quote repriced when edited, fixed when
// sent, and revised into a new draft.
//
//	APERIO_TEST_DATABASE_URL="host=127.0.0.1 port=5433 user=postgres password=postgrespassword dbname=aperio sslmode=disable" \
//	  go test ./internal/service/sales/ -v

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/shokki-engineering/aperio/internal/database/repository/aperio/guest"
	"github.com/shokki-engineering/aperio/internal/database/repository/aperio/venue"
	"github.com/shokki-engineering/aperio/internal/service/generated/aperio/catalog/v1/catalogpbv1svc"
	"github.com/shokki-engineering/aperio/internal/service/pricing"
	"github.com/shokki-engineering/aperio/internal/service/sales"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/catalog/v1/catalogpbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/guest/v1/guestpbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/pricing/v1/pricingpbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/sales/v1/salespbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/type/v1/typepbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/venue/v1/venuepbv1"
	"github.com/shokki-engineering/aperio/test/live"
	"google.golang.org/genproto/googleapis/type/date"
	"google.golang.org/genproto/googleapis/type/decimal"
	"google.golang.org/genproto/googleapis/type/money"
	"google.golang.org/genproto/googleapis/type/postaladdress"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
)

func inr(units int64) *money.Money { return &money.Money{CurrencyCode: "INR", Units: units} }

// rupees is m as "1234.50".
func rupees(m *money.Money) string {
	return fmt.Sprintf("%d.%02d", m.GetUnits(), m.GetNanos()/10_000_000)
}

func TestLiveQuote(t *testing.T) {
	ctx, conn := context.Background(), live.Conn(t)
	venues := venue.New(conn)
	catalogRepos := live.CatalogRepositories(conn)
	menus := catalogpbv1svc.NewCatalogServiceServer(catalogRepos, catalogpbv1svc.NopCatalogServiceHooks{})
	prices := pricing.New(live.PricingRepositories(conn), catalogRepos, venues.Venues)
	svc := sales.New(live.SalesRepositories(conn), venues.Venues, catalogRepos.Menus, prices, nil, nil)
	must := func(err error, what string) {
		t.Helper()
		if err != nil {
			t.Fatalf("%s: %v", what, err)
		}
	}

	v, err := venues.Venues.Create(ctx, &venuepbv1.Venue{DisplayName: "Pricing Hall " + fmt.Sprint(time.Now().UnixNano()),
		Kind: venuepbv1.VenueKind_VENUE_KIND_EVENTS, TimeZone: "Asia/Kolkata", CurrencyCode: "INR",
		Address: &postaladdress.PostalAddress{RegionCode: "IN"}, Gst: &typepbv1.Gst{Gstin: "29ABCDE1234F1Z5", LegalDisplayName: "Test", GstStateCode: "29"}})
	must(err, "create venue")
	c, err := guest.New(conn).Contacts.Create(ctx, &guestpbv1.Contact{Kind: guestpbv1.ContactKind_CONTACT_KIND_PERSON, DisplayName: "John Doe"})
	must(err, "create contact")
	food := &typepbv1.Tax{HsnSacCode: "996334", GstRate: &decimal.Decimal{Value: "5"}}
	dish, err := menus.CreateDish(ctx, &catalogpbv1.CreateDishRequest{Parent: v.GetName(), Dish: &catalogpbv1.Dish{DisplayName: "Paneer tikka", Course: catalogpbv1.Course_COURSE_STARTER}})
	must(err, "create dish")
	menu, err := menus.CreateMenu(ctx, &catalogpbv1.CreateMenuRequest{Parent: v.GetName(), Menu: &catalogpbv1.Menu{
		DisplayName: "Silver Vegetarian", PlatePrice: inr(1200), MinPlates: 100, Tax: food, Active: true,
		Courses: []*catalogpbv1.MenuCourse{{Course: catalogpbv1.Course_COURSE_STARTER, Dishes: []string{dish.GetName()}}},
	}})
	must(err, "create menu")
	season, err := prices.CreateSeason(ctx, &pricingpbv1.CreateSeasonRequest{Parent: v.GetName(), Season: &pricingpbv1.Season{
		DisplayName: "Wedding season", RecursYearly: true,
		Dates: []*typepbv1.DateRange{{StartDate: &date.Date{Year: 2026, Month: 11, Day: 1}, EndDate: &date.Date{Year: 2027, Month: 2, Day: 28}}},
	}})
	must(err, "create season")
	rule, err := prices.CreatePriceRule(ctx, &pricingpbv1.CreatePriceRuleRequest{Parent: v.GetName(), PriceRule: &pricingpbv1.PriceRule{
		DisplayName: "Wedding season menus", Season: season.GetName(), Target: pricingpbv1.PriceTarget_PRICE_TARGET_MENU,
		Adjustment: &pricingpbv1.PriceRule_Percent{Percent: &decimal.Decimal{Value: "15"}}, Active: true,
	}})
	must(err, "create rule")

	// On its own: 200 plates in December are ₹1,380 each, ₹2,76,000, GST ₹13,800.
	december := &date.Date{Year: 2027, Month: 12, Day: 12}
	p, err := prices.CalculatePrice(ctx, &pricingpbv1.CalculatePriceRequest{Venue: v.GetName(), Item: menu.GetName(), ServiceDate: december, Quantity: 200})
	must(err, "calculate price")
	if rupees(p.GetUnitPrice()) != "1380.00" || rupees(p.GetAmount()) != "276000.00" || rupees(p.GetTaxAmount()) != "13800.00" || p.GetPriceRule() != rule.GetName() {
		t.Errorf("price = %s × 200 = %s + GST %s by %q; want 1380.00, 276000.00, 13800.00 by the season rule",
			rupees(p.GetUnitPrice()), rupees(p.GetAmount()), rupees(p.GetTaxAmount()), p.GetPriceRule())
	}
	// In July the base price stands.
	july, err := prices.CalculatePrice(ctx, &pricingpbv1.CalculatePriceRequest{Venue: v.GetName(), Item: menu.GetName(), ServiceDate: &date.Date{Year: 2027, Month: 7, Day: 1}})
	must(err, "calculate july")
	if rupees(july.GetUnitPrice()) != "1200.00" || july.GetPriceRule() != "" {
		t.Errorf("July price = %s by %q; want 1200.00 by no rule", rupees(july.GetUnitPrice()), july.GetPriceRule())
	}

	// A quote: 200 plates and decor at ₹50,000 (18%), less ₹10,000.
	decor := &typepbv1.Tax{HsnSacCode: "998596", GstRate: &decimal.Decimal{Value: "18"}}
	q, err := svc.CreateQuote(ctx, &salespbv1.CreateQuoteRequest{Parent: v.GetName(), Quote: &salespbv1.Quote{
		DisplayName: "Doe wedding", Contact: c.GetName(), Guests: 200,
		Dates: &typepbv1.DateRange{StartDate: december, EndDate: december},
		Lines: []*salespbv1.QuoteLine{
			{Kind: salespbv1.LineKind_LINE_KIND_MENU, Item: menu.GetName(), Quantity: 200},
			{Kind: salespbv1.LineKind_LINE_KIND_SERVICE, Description: "Floral decor", Quantity: 1, UnitPrice: inr(50000), Tax: decor},
			{Kind: salespbv1.LineKind_LINE_KIND_DISCOUNT, Description: "Early booking", Quantity: 1, UnitPrice: inr(-10000)},
		},
	}})
	must(err, "create quote")
	// 276000 + 50000 - 10000 = 316000; GST 13800 + 9000 = 22800.
	if rupees(q.GetSubtotal()) != "316000.00" || rupees(q.GetTaxTotal()) != "22800.00" || rupees(q.GetTotal()) != "338800.00" {
		t.Errorf("quote = %s + %s = %s; want 316000.00 + 22800.00 = 338800.00", rupees(q.GetSubtotal()), rupees(q.GetTaxTotal()), rupees(q.GetTotal()))
	}

	// Below the menu's minimum is refused.
	q.Lines[0].Quantity = 50
	if _, err := svc.UpdateQuote(ctx, &salespbv1.UpdateQuoteRequest{Quote: q, UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"lines"}}}); err == nil {
		t.Error("50 plates of a 100-plate menu were accepted")
	}
	// An edit reprices: 300 plates.
	q, _ = svc.GetQuote(ctx, &salespbv1.GetQuoteRequest{Name: q.GetName()})
	q.Lines[0].Quantity = 300
	q, err = svc.UpdateQuote(ctx, &salespbv1.UpdateQuoteRequest{Quote: q, UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"lines"}}})
	must(err, "update quote")
	if rupees(q.GetSubtotal()) != "454000.00" {
		t.Errorf("after 300 plates, subtotal = %s; want 454000.00", rupees(q.GetSubtotal()))
	}

	sent, err := svc.SendQuote(ctx, &salespbv1.SendQuoteRequest{Name: q.GetName(), Etag: q.GetEtag()})
	must(err, "send")
	if sent.GetState() != salespbv1.Quote_STATE_SENT || sent.GetExpireTime() == nil {
		t.Errorf("sent: state %v, expire_time %v", sent.GetState(), sent.GetExpireTime())
	}
	sent.DisplayName = "changed"
	if _, err := svc.UpdateQuote(ctx, &salespbv1.UpdateQuoteRequest{Quote: sent, UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"display_name"}}}); err == nil {
		t.Error("a sent quote was edited")
	}

	draft, err := svc.ReviseQuote(ctx, &salespbv1.ReviseQuoteRequest{Name: sent.GetName()})
	must(err, "revise")
	old, _ := svc.GetQuote(ctx, &salespbv1.GetQuoteRequest{Name: sent.GetName()})
	if draft.GetState() != salespbv1.Quote_STATE_DRAFT || draft.GetSupersedes() != sent.GetName() || old.GetState() != salespbv1.Quote_STATE_SUPERSEDED {
		t.Errorf("revise: draft %v superseding %q, old %v", draft.GetState(), draft.GetSupersedes(), old.GetState())
	}
	if rupees(draft.GetTotal()) != rupees(sent.GetTotal()) {
		t.Errorf("revised total = %s; want the same prices, %s", rupees(draft.GetTotal()), rupees(sent.GetTotal()))
	}
}
