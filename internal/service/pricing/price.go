// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package pricing

// price.go is CalculatePrice: one item on one date -- a menu (per plate), a
// fixed package (with its extra guests), a space (a hall's hire, a room
// type's night) or a dish -- its base price with the venue's matching rule
// applied, and the GST on it.

import (
	"context"
	"math/big"
	"strings"

	"github.com/shokki-engineering/aperio/internal/database/repository/repox"
	"github.com/shokki-engineering/aperio/internal/pricing"
	"github.com/shokki-engineering/aperio/internal/service/generated/servicex"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/catalog/v1/catalogpbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/pricing/v1/pricingpbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/type/v1/typepbv1"
	"google.golang.org/genproto/googleapis/type/date"
	"google.golang.org/genproto/googleapis/type/money"
)

// priced is an item's price on a date.
type priced struct {
	base *money.Money  // its base price; nil for a space, which has none
	rule string        // the rule applied, if any
	unit *big.Rat      // the price of one, rounded
	tax  *typepbv1.Tax // the item's GST; nil for a space
}

// CalculatePrice prices req.Item for req.ServiceDate at req.Venue.
func (s *Server) CalculatePrice(ctx context.Context, req *pricingpbv1.CalculatePriceRequest) (*pricingpbv1.CalculatePriceResponse, error) {
	var out *pricingpbv1.CalculatePriceResponse
	err := s.Options().Run(ctx, "SalesService", "CalculatePrice", func(ctx context.Context) error {
		cur, err := s.currency(ctx, req.GetVenue())
		if err != nil {
			return err
		}
		p, err := s.price(ctx, req.GetVenue(), "item", req.GetItem(), req.GetServiceDate(), req.GetGuests(), cur)
		if err != nil {
			return err
		}
		qty := int64(max(req.GetQuantity(), 1))
		amount := pricing.Mul(p.unit, qty)
		out = &pricingpbv1.CalculatePriceResponse{
			BasePrice: p.base,
			PriceRule: p.rule,
			UnitPrice: pricing.Money(p.unit, cur),
			Amount:    pricing.Money(amount, cur),
			Tax:       p.tax,
		}
		tax, err := pricing.GST(amount, p.tax.GetGstRate())
		if err != nil {
			return servicex.BadField("INVALID_TAX", "item", err.Error(), `Set the item's GST rate as a percentage, e.g. "18"`)
		}
		out.TaxAmount = pricing.Money(tax, cur)
		return nil
	})
	return out, err
}

// price is item's price at venueName on d for guests; field names the
// request field item came in, for errors.
func (s *Server) price(ctx context.Context, venueName, field, item string, d *date.Date, guests int32, cur string) (priced, error) {
	parts := strings.Split(item, "/")
	if len(parts) != 4 || parts[0]+"/"+parts[1] != venueName {
		return priced{}, servicex.BadField("INVALID_ITEM", field, item+" is not an item of "+venueName,
			"Name a menu, fixed package, space or dish of the venue")
	}
	var (
		p      priced
		target pricingpbv1.PriceTarget
		extra  *big.Rat // a package's extra guests, at the extra-guest price
		err    error
	)
	switch parts[2] {
	case "menus":
		target = pricingpbv1.PriceTarget_PRICE_TARGET_MENU
		var menu *catalogpbv1.Menu
		if menu, err = s.catalog.Menus.Get(ctx, item); err == nil {
			p.base, p.tax = menu.GetPlatePrice(), menu.GetTax()
		}
	case "fixedPackages":
		target = pricingpbv1.PriceTarget_PRICE_TARGET_FIXED_PACKAGE
		var fp *catalogpbv1.FixedPackage
		if fp, err = s.catalog.FixedPackages.Get(ctx, item); err == nil {
			p.base, p.tax = fp.GetPrice(), fp.GetTax()
			if n := guests - fp.GetIncludedGuests(); n > 0 && fp.GetExtraGuestPrice() != nil {
				extra = pricing.Mul(pricing.Rat(fp.GetExtraGuestPrice()), int64(n))
			}
		}
	case "spaces":
		target = pricingpbv1.PriceTarget_PRICE_TARGET_SPACE
	case "dishes":
		target = pricingpbv1.PriceTarget_PRICE_TARGET_DISH
		var dish *catalogpbv1.Dish
		if dish, err = s.catalog.Dishes.Get(ctx, item); err == nil {
			p.base, p.tax = dish.GetPrice(), dish.GetTax()
		}
	default:
		return priced{}, servicex.BadField("INVALID_ITEM", field, item+" is not a menu, fixed package, space or dish",
			"Name a menu, fixed package, space or dish of the venue")
	}
	if err != nil {
		return priced{}, servicex.NotFoundAs(err, "aperio.catalog.v1/*", item)
	}
	if err := pricing.SameCurrency(p.base, cur, item); err != nil {
		return priced{}, servicex.Blocked("CURRENCY_MISMATCH", venueType, venueName, err.Error(), "State the item's price in the venue's currency")
	}

	rule, err := s.rule(ctx, venueName, pricing.Match{Target: target, Item: item, Date: d, Guests: guests})
	if err != nil {
		return priced{}, err
	}
	var unit *big.Rat
	if p.base != nil {
		unit = pricing.Rat(p.base)
	}
	if rule != nil {
		var ok bool
		if unit, ok, err = pricing.Apply(rule, unit); err != nil {
			return priced{}, servicex.Blocked("INVALID_RULE", "aperio.pricing.v1/PriceRule", rule.GetName(), err.Error(), "Fix the rule's percentage")
		} else if ok {
			p.rule = rule.GetName()
		}
		if extra != nil && pricing.Scales(rule) {
			extra, _, _ = pricing.Apply(rule, extra)
		}
	}
	if unit == nil {
		return priced{}, servicex.Blocked("NO_PRICE", "aperio.catalog.v1/*", item, "it has no base price, and no rule fixes one for the date",
			"Add a price rule with a fixed price for it")
	}
	if extra != nil {
		unit = pricing.Sum(unit, extra)
	}
	p.unit = pricing.Round(unit, cur)
	return p, nil
}

// rule is the rule that prices m at venueName, with the venue's rules and
// seasons read for it.
func (s *Server) rule(ctx context.Context, venueName string, m pricing.Match) (*pricingpbv1.PriceRule, error) {
	rules, err := all(func(in repox.ListInput) ([]*pricingpbv1.PriceRule, string, error) {
		return s.Repositories().PriceRules.List(ctx, venueName, in)
	})
	if err != nil {
		return nil, err
	}
	seasons, err := all(func(in repox.ListInput) ([]*pricingpbv1.Season, string, error) {
		return s.Repositories().Seasons.List(ctx, venueName, in)
	})
	if err != nil {
		return nil, err
	}
	m.Seasons = make(map[string]*pricingpbv1.Season, len(seasons))
	for _, s := range seasons {
		m.Seasons[s.GetName()] = s
	}
	return pricing.Pick(rules, m), nil
}
