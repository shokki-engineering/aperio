// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package sales

// quotes.go prices a quote: every line through the pricing service unless its
// price was set by hand, each line's amount and GST, and the quote's totals.
// A draft is priced when it is made and after every edit, so it follows the
// rules as they change; once sent, its prices are fixed and it is not edited.

import (
	"context"
	"fmt"
	"math/big"
	"strings"

	"github.com/shokki-engineering/aperio/internal/pricing"
	"github.com/shokki-engineering/aperio/internal/service/generated/servicex"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/pricing/v1/pricingpbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/sales/v1/salespbv1"
	"google.golang.org/protobuf/proto"
)

// totals are the server-owned fields reprice sets beside the lines.
var totals = []string{"subtotal", "tax_total", "total"}

// BeforeCreateQuote prices the new quote; Create stores what it sets.
func (h *hooks) BeforeCreateQuote(ctx context.Context, parent string, m *salespbv1.Quote) error {
	return h.reprice(ctx, parent, m)
}

// BeforeUpdateQuote refuses to edit a quote that is no longer a draft: its
// prices and terms are what the guest was offered. ReviseQuote makes a new
// draft from it.
func (h *hooks) BeforeUpdateQuote(ctx context.Context, m *salespbv1.Quote, paths []string) error {
	existing, err := h.repos.Quotes.Get(ctx, m.GetName())
	if err != nil {
		return servicex.NotFoundAs(err, quoteType, m.GetName())
	}
	if existing.GetState() != salespbv1.Quote_STATE_DRAFT {
		return servicex.Blocked("QUOTE_NOT_DRAFT", quoteType, m.GetName(),
			"the quote is "+existing.GetState().String()+"; only a draft is edited",
			"Revise it with ReviseQuote, and edit the new draft")
	}
	return nil
}

// AfterUpdateQuote prices the edited draft again: lines, dates or guests may
// have changed, and so may the rules since it was last priced.
func (h *hooks) AfterUpdateQuote(ctx context.Context, out *salespbv1.Quote) error {
	if err := h.reprice(ctx, servicex.ParentOf(out.GetName()), out); err != nil {
		return err
	}
	updated, err := h.repos.Quotes.UpdateWithOutputOnly(ctx, out, []string{"lines"}, totals)
	if err != nil {
		return err
	}
	proto.Reset(out)
	proto.Merge(out, updated)
	return nil
}

// reprice prices every line of q at venueName and totals it, in place.
func (h *hooks) reprice(ctx context.Context, venueName string, q *salespbv1.Quote) error {
	cur, err := h.currency(ctx, venueName)
	if err != nil {
		return err
	}
	subtotal, taxTotal := new(big.Rat), new(big.Rat)
	for i, l := range q.GetLines() {
		field := fmt.Sprintf("quote.lines[%d]", i)
		if err := h.priceLine(ctx, venueName, field, q, l, cur); err != nil {
			return err
		}
		amount := pricing.Mul(pricing.Rat(l.GetUnitPrice()), int64(l.GetQuantity()))
		tax, err := pricing.GST(amount, l.GetTax().GetGstRate())
		if err != nil {
			return servicex.BadField("INVALID_TAX", field+".tax.gst_rate", err.Error(), `Set the rate as a percentage, e.g. "18"`)
		}
		l.Amount, l.TaxAmount = pricing.Money(amount, cur), pricing.Money(tax, cur)
		subtotal.Add(subtotal, pricing.Rat(l.Amount))
		taxTotal.Add(taxTotal, pricing.Rat(l.TaxAmount))
	}
	q.Subtotal = pricing.Money(subtotal, cur)
	q.TaxTotal = pricing.Money(taxTotal, cur)
	q.Total = pricing.Money(pricing.Sum(subtotal, taxTotal), cur)
	return nil
}

// priceLine sets l's unit price, rule and tax: from the pricing service for
// its item and date, unless its price was set by hand or it sells no item.
func (h *hooks) priceLine(ctx context.Context, venueName, field string, q *salespbv1.Quote, l *salespbv1.QuoteLine, cur string) error {
	if l.GetQuantity() <= 0 {
		return servicex.BadField("INVALID_QUANTITY", field+".quantity", "the quantity is not positive", "Set how many: plates, nights, hours or units")
	}
	byHand := l.GetPriceOverride() || l.GetItem() == ""
	if byHand {
		l.PriceRule = ""
		switch {
		case l.GetUnitPrice() == nil:
			return servicex.BadField("MISSING_PRICE", field+".unit_price", "a line priced by hand has no unit price", "Set unit_price, or name an item to price it")
		case l.GetKind() == salespbv1.LineKind_LINE_KIND_DISCOUNT && pricing.Rat(l.GetUnitPrice()).Sign() >= 0:
			return servicex.BadField("INVALID_DISCOUNT", field+".unit_price", "a discount's unit price is not negative", "Set a negative unit_price")
		}
		if err := pricing.SameCurrency(l.GetUnitPrice(), cur, field+".unit_price"); err != nil {
			return servicex.BadField("CURRENCY_MISMATCH", field+".unit_price", err.Error(), "State the price in the venue's currency")
		}
		l.UnitPrice = pricing.Money(pricing.Rat(l.GetUnitPrice()), cur)
		return nil
	}
	d := l.GetServiceDate()
	if d == nil {
		d = q.GetDates().GetStartDate()
	}
	p, err := h.pricer.CalculatePrice(ctx, &pricingpbv1.CalculatePriceRequest{
		Venue: venueName, Item: l.GetItem(), ServiceDate: d, Quantity: 1, Guests: q.GetGuests(),
	})
	if err != nil {
		return err
	}
	if strings.Contains(l.GetItem(), "/menus/") {
		if menu, err := h.menus.Get(ctx, l.GetItem()); err == nil && menu.GetMinPlates() > 0 && l.GetQuantity() < menu.GetMinPlates() {
			return servicex.BadField("BELOW_MIN_PLATES", field+".quantity",
				fmt.Sprintf("the menu is sold for %d plates or more", menu.GetMinPlates()), "Quote at least the minimum, or choose another menu")
		}
	}
	l.UnitPrice, l.PriceRule = p.GetUnitPrice(), p.GetPriceRule()
	if l.GetTax() == nil {
		l.Tax = p.GetTax()
	}
	return nil
}
