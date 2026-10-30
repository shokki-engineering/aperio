// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package sales

// invoice_lines.go turns an accepted quote's lines into its invoice's: each
// line's description, item, quantity, unit price and tax as the quote fixed
// them when it was sent.
//
// An invoice states amounts as positive, and a reduction as a line's
// discount, so a quote's discount line (a negative price) is not a line of
// its own: its amount is taken off the lines it discounts -- the nearest
// before it at the same GST rate first, then the nearest before it at any
// rate, then the nearest after it -- each up to its value. A line the quote
// gave no GST rate was charged none, and is invoiced at a rate of 0.

import (
	"context"
	"fmt"
	"math/big"
	"slices"
	"strings"

	"github.com/shokki-engineering/aperio/internal/pricing"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/finance/v1/financepbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/sales/v1/salespbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/type/v1/typepbv1"
	"google.golang.org/genproto/googleapis/type/decimal"
	"google.golang.org/protobuf/proto"
)

// maxDescription is the longest description an invoice line takes.
const maxDescription = 500

// invoiceLines are q's lines as its invoice's, discounts taken off the
// lines they discount.
func (s *Server) invoiceLines(ctx context.Context, q *salespbv1.Quote) ([]*financepbv1.InvoiceLine, error) {
	var out []*financepbv1.InvoiceLine
	var at []int // the quote line each invoice line is
	for i, l := range q.GetLines() {
		if discount(l) {
			continue
		}
		tax, _ := proto.Clone(l.GetTax()).(*typepbv1.Tax)
		if tax == nil {
			tax = &typepbv1.Tax{}
		}
		if tax.GetGstRate().GetValue() == "" {
			tax.GstRate = &decimal.Decimal{Value: "0"} // the quote charged none
		}
		out = append(out, &financepbv1.InvoiceLine{Description: s.describe(ctx, l), Item: l.GetItem(),
			Tax: tax, Quantity: l.GetQuantity(), UnitPrice: l.GetUnitPrice()})
		at = append(at, i)
	}
	for i, l := range q.GetLines() {
		if !discount(l) {
			continue
		}
		left := new(big.Rat).Neg(pricing.Mul(pricing.Rat(l.GetUnitPrice()), int64(l.GetQuantity())))
		for _, j := range order(i, at, out, l.GetTax().GetGstRate().GetValue()) {
			left = takeOff(out[j], left, l.GetUnitPrice().GetCurrencyCode())
		}
		if left.Sign() > 0 {
			return nil, fmt.Errorf("quote line %d: its discount is more than the lines it could come off", i)
		}
	}
	return out, nil
}

// discount reports whether l takes money off: a discount line, or any line
// priced below nothing.
func discount(l *salespbv1.QuoteLine) bool {
	return l.GetKind() == salespbv1.LineKind_LINE_KIND_DISCOUNT || pricing.Rat(l.GetUnitPrice()).Sign() < 0
}

// order is the invoice lines (indexes into lines; at[k] is line k's quote
// line) a discount at quote line i comes off, in turn: before it at rate,
// before it at any rate, then after it.
func order(i int, at []int, lines []*financepbv1.InvoiceLine, rate string) []int {
	var same, before, after []int
	for k := len(at) - 1; k >= 0; k-- {
		switch {
		case at[k] > i:
			after = append([]int{k}, after...)
		case lines[k].GetTax().GetGstRate().GetValue() == rate:
			same = append(same, k)
		default:
			before = append(before, k)
		}
	}
	return slices.Concat(same, before, after)
}

// takeOff discounts line by as much of amount as its value allows, and
// returns what is left.
func takeOff(line *financepbv1.InvoiceLine, amount *big.Rat, currency string) *big.Rat {
	if amount.Sign() <= 0 {
		return amount
	}
	had := pricing.Rat(line.GetDiscount())
	room := pricing.Sum(pricing.Mul(pricing.Rat(line.GetUnitPrice()), int64(line.GetQuantity())), new(big.Rat).Neg(had))
	take := amount
	if take.Cmp(room) > 0 {
		take = room
	}
	if take.Sign() <= 0 {
		return amount
	}
	line.Discount = pricing.Money(pricing.Sum(had, take), currency)
	return pricing.Sum(amount, new(big.Rat).Neg(take))
}

// describe is how the invoice prints l: its description, else its item's
// name, else what kind of line it is.
func (s *Server) describe(ctx context.Context, l *salespbv1.QuoteLine) string {
	d := strings.TrimSpace(l.GetDescription())
	if d == "" && strings.Contains(l.GetItem(), "/menus/") && s.h.menus != nil {
		if m, err := s.h.menus.Get(ctx, l.GetItem()); err == nil {
			d = m.GetDisplayName()
		}
	}
	if d == "" && strings.Contains(l.GetItem(), "/spaces/") && s.h.base != nil {
		if sp, err := s.h.base.Spaces.Get(ctx, l.GetItem()); err == nil {
			d = sp.GetDisplayName()
		}
	}
	if d == "" {
		kind := strings.TrimPrefix(l.GetKind().String(), "LINE_KIND_")
		d = strings.ReplaceAll(strings.ToUpper(kind[:1])+strings.ToLower(kind[1:]), "_", " ")
	}
	if r := []rune(d); len(r) > maxDescription {
		d = string(r[:maxDescription])
	}
	return d
}
