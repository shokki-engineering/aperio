// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package finance

// gst.go computes an invoice: each line's taxable value and its GST, split
// by the place of supply, and the invoice's totals.
//
// A supply within the venue's state is taxed CGST and SGST, half the rate
// each; one outside it, IGST at the whole rate (IGST Act, sections 7-8).
// For services at the venue -- a stay, an event -- the place of supply is
// the venue's own state; the API requires the caller to name it, and an
// invoice made in code without one takes the venue's. Each tax is rounded to
// the paisa per line; the invoice value, to the rupee.

import (
	"fmt"
	"math/big"

	"github.com/shokki-engineering/aperio/internal/pricing"
	"github.com/shokki-engineering/aperio/internal/service/generated/servicex"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/finance/v1/financepbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/venue/v1/venuepbv1"
)

// totals are the server-owned fields compute sets beside the lines.
var totals = []string{"taxable_total", "cgst", "sgst", "igst", "total", "due"}

// compute fills in inv's line values and totals for venue v, in place.
func compute(inv *financepbv1.Invoice, v *venuepbv1.Venue) error {
	cur, home := v.GetCurrencyCode(), v.GetGst().GetGstStateCode()
	if inv.GetSupplyStateCode() == "" {
		inv.SupplyStateCode = home
	}
	intra := inv.GetSupplyStateCode() == home
	taxable, cgst, sgst, igst := new(big.Rat), new(big.Rat), new(big.Rat), new(big.Rat)
	for i, l := range inv.GetLines() {
		field := fmt.Sprintf("invoice.lines[%d]", i)
		unit, discount := pricing.Rat(l.GetUnitPrice()), pricing.Rat(l.GetDiscount())
		for _, m := range []struct {
			f string
			v *big.Rat
		}{{"unit_price", unit}, {"discount", discount}} {
			if m.v.Sign() < 0 {
				return servicex.BadField("NEGATIVE_AMOUNT", field+"."+m.f, "an invoice amount is negative",
					"State amounts as positive; a reduction is a discount, or a credit note")
			}
		}
		if err := sameCurrency(field, cur, l); err != nil {
			return err
		}
		gross := pricing.Mul(unit, int64(l.GetQuantity()))
		if discount.Cmp(gross) > 0 {
			return servicex.BadField("DISCOUNT_TOO_LARGE", field+".discount", "the discount is more than the line", "Discount at most quantity × unit_price")
		}
		rate, err := pricing.Decimal(l.GetTax().GetGstRate())
		if err != nil {
			return servicex.BadField("INVALID_TAX", field+".tax.gst_rate", err.Error(), `Set the rate as a percentage, e.g. "18"`)
		}
		value := pricing.Round(pricing.Sum(gross, new(big.Rat).Neg(discount)), cur)
		l.TaxableValue = pricing.Money(value, cur)
		l.Cgst, l.Sgst, l.Igst = nil, nil, nil
		if intra {
			half := pricing.Round(pricing.Percent(value, new(big.Rat).Quo(rate, big.NewRat(2, 1))), cur)
			l.Cgst, l.Sgst = pricing.Money(half, cur), pricing.Money(half, cur)
			cgst.Add(cgst, half)
			sgst.Add(sgst, half)
		} else {
			whole := pricing.Round(pricing.Percent(value, rate), cur)
			l.Igst = pricing.Money(whole, cur)
			igst.Add(igst, whole)
		}
		taxable.Add(taxable, value)
	}
	total := pricing.Whole(pricing.Sum(taxable, cgst, sgst, igst))
	inv.TaxableTotal = pricing.Money(taxable, cur)
	inv.Cgst, inv.Sgst, inv.Igst = pricing.Money(cgst, cur), pricing.Money(sgst, cur), pricing.Money(igst, cur)
	inv.Total = pricing.Money(total, cur)
	inv.Due = pricing.Money(pricing.Sum(total, new(big.Rat).Neg(pricing.Rat(inv.GetPaid()))), cur)
	return nil
}

// sameCurrency fails when a line's amounts are not in the venue's currency.
func sameCurrency(field, cur string, l *financepbv1.InvoiceLine) error {
	for f, m := range map[string]interface{ GetCurrencyCode() string }{"unit_price": l.GetUnitPrice(), "discount": l.GetDiscount()} {
		if c := m.GetCurrencyCode(); c != "" && c != cur {
			return servicex.BadField("CURRENCY_MISMATCH", field+"."+f, "the amount is in "+c+", not the venue's "+cur,
				"Invoice in the venue's currency; record a foreign payment's exchange rate on the payment")
		}
	}
	return nil
}
