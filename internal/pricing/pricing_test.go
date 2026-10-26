// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package pricing_test

// Tests of the arithmetic and the rules: rounding to paise, a wedding season
// that wraps the new year, a weekend season, and which rule wins.

import (
	"math/big"
	"testing"

	"github.com/shokki-engineering/aperio/internal/pricing"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/pricing/v1/pricingpbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/type/v1/typepbv1"
	"google.golang.org/genproto/googleapis/type/date"
	"google.golang.org/genproto/googleapis/type/dayofweek"
	"google.golang.org/genproto/googleapis/type/decimal"
	"google.golang.org/genproto/googleapis/type/money"
)

func day(y, m, d int32) *date.Date { return &date.Date{Year: y, Month: m, Day: d} }

func inr(units int64, nanos int32) *money.Money {
	return &money.Money{CurrencyCode: "INR", Units: units, Nanos: nanos}
}

func TestMoney(t *testing.T) {
	for _, c := range []struct {
		in   *big.Rat
		want *money.Money
	}{
		{big.NewRat(1, 200), inr(0, 10_000_000)},            // 0.005 rounds up to 0.01
		{big.NewRat(-1, 200), inr(0, -10_000_000)},          // and away from zero
		{big.NewRat(123449, 100), inr(1234, 490_000_000)},   // 1234.49 stays
		{big.NewRat(2468985, 2000), inr(1234, 490_000_000)}, // 1234.4925
	} {
		if got := pricing.Money(c.in, "INR"); got.GetUnits() != c.want.GetUnits() || got.GetNanos() != c.want.GetNanos() {
			t.Errorf("Money(%s) = %d.%09d; want %d.%09d", c.in.FloatString(6), got.GetUnits(), got.GetNanos(), c.want.GetUnits(), c.want.GetNanos())
		}
	}
	// 18% GST on ₹1,499.50 is ₹269.91.
	rate, _ := pricing.Decimal(&decimal.Decimal{Value: "18"})
	if got := pricing.Money(pricing.Percent(pricing.Rat(inr(1499, 500_000_000)), rate), "INR"); got.GetUnits() != 269 || got.GetNanos() != 910_000_000 {
		t.Errorf("18%% of 1499.50 = %v; want 269.91", got)
	}
}

func TestCovers(t *testing.T) {
	wedding := &pricingpbv1.Season{RecursYearly: true, Dates: []*typepbv1.DateRange{{StartDate: day(2025, 11, 1), EndDate: day(2026, 2, 28)}}}
	weekends := &pricingpbv1.Season{Weekdays: []dayofweek.DayOfWeek{dayofweek.DayOfWeek_SATURDAY, dayofweek.DayOfWeek_SUNDAY}}
	diwali := &pricingpbv1.Season{Dates: []*typepbv1.DateRange{{StartDate: day(2026, 11, 6), EndDate: day(2026, 11, 12)}}}
	for _, c := range []struct {
		name   string
		season *pricingpbv1.Season
		d      *date.Date
		want   bool
	}{
		{"wedding, December next year", wedding, day(2027, 12, 12), true},
		{"wedding, January", wedding, day(2028, 1, 20), true},
		{"wedding, monsoon", wedding, day(2027, 7, 1), false},
		{"weekend, Saturday", weekends, day(2026, 10, 31), true},
		{"weekend, Monday", weekends, day(2026, 10, 26), false},
		{"diwali, in", diwali, day(2026, 11, 8), true},
		{"diwali, a year on", diwali, day(2027, 11, 8), false},
	} {
		if got := pricing.Covers(c.season, c.d); got != c.want {
			t.Errorf("%s: Covers = %v; want %v", c.name, got, c.want)
		}
	}
}

func TestPickAndApply(t *testing.T) {
	const menu = "venues/v/menus/silver"
	seasons := map[string]*pricingpbv1.Season{
		"venues/v/seasons/wedding": {RecursYearly: true, Dates: []*typepbv1.DateRange{{StartDate: day(2025, 11, 1), EndDate: day(2026, 2, 28)}}},
	}
	rules := []*pricingpbv1.PriceRule{
		{Name: "venues/v/priceRules/a-season", Active: true, Target: pricingpbv1.PriceTarget_PRICE_TARGET_MENU,
			Season: "venues/v/seasons/wedding", Priority: 10, Adjustment: &pricingpbv1.PriceRule_Percent{Percent: &decimal.Decimal{Value: "15"}}},
		{Name: "venues/v/priceRules/b-volume", Active: true, Target: pricingpbv1.PriceTarget_PRICE_TARGET_MENU, Items: []string{menu},
			MinGuests: 500, Priority: 20, Adjustment: &pricingpbv1.PriceRule_Amount{Amount: inr(-100, 0)}},
		{Name: "venues/v/priceRules/c-off", Active: false, Target: pricingpbv1.PriceTarget_PRICE_TARGET_MENU, Priority: 99,
			Adjustment: &pricingpbv1.PriceRule_FixedPrice{FixedPrice: inr(1, 0)}},
	}
	base := pricing.Rat(inr(1200, 0))
	for _, c := range []struct {
		name   string
		d      *date.Date
		guests int32
		rule   string
		price  int64
	}{
		{"monsoon, small", day(2027, 7, 1), 200, "", 1200},
		{"wedding season", day(2027, 12, 5), 200, "venues/v/priceRules/a-season", 1380},
		{"wedding season, 600 guests", day(2027, 12, 5), 600, "venues/v/priceRules/b-volume", 1100},
	} {
		r := pricing.Pick(rules, pricing.Match{Target: pricingpbv1.PriceTarget_PRICE_TARGET_MENU, Item: menu, Date: c.d, Guests: c.guests, Seasons: seasons})
		if r.GetName() != c.rule {
			t.Errorf("%s: rule = %q; want %q", c.name, r.GetName(), c.rule)
		}
		price := base
		if r != nil {
			price, _, _ = pricing.Apply(r, base)
		}
		if got := pricing.Money(price, "INR").GetUnits(); got != c.price {
			t.Errorf("%s: price = %d; want %d", c.name, got, c.price)
		}
	}
	if _, ok, _ := pricing.Apply(rules[0], nil); ok {
		t.Error("a percentage priced an item with no base price")
	}
}
