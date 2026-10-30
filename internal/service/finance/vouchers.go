// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package finance

// vouchers.go issues the receipt voucher for an advance (CGST Rules, rule
// 50), in the venue's receipt-voucher series (SBR/RV2627/0003). It states
// the advance as the supply it is for, the tax included in what was paid:
//
//   - paid against a proforma, each of its lines in the share the advance
//     is of its total -- a ₹30,000 advance on a ₹1,18,000 proforma is
//     30000/118000 of each line's taxable value, at the line's own rate;
//   - paid against no invoice, one line at 18% under SAC 9963
//     (accommodation, food and beverage services): rule 50 has an advance
//     whose rate cannot yet be told taxed at 18%.
//
// It is issued PAID (issue.go): it records money received, and nothing is
// owed on it. A venue with no invoice series does not invoice through
// aperio, and gets no voucher.

import (
	"context"
	"math/big"

	"github.com/shokki-engineering/aperio/internal/pricing"
	"github.com/shokki-engineering/aperio/internal/service/generated/servicex"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/finance/v1/financepbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/type/v1/typepbv1"
	"google.golang.org/genproto/googleapis/type/decimal"
	"google.golang.org/protobuf/proto"
)

// The line of an advance against no invoice (rule 50).
const (
	advanceSAC  = "9963"
	advanceRate = 18
)

// advanceNote starts a receipt voucher's notes, followed by the payment's
// name: how its refund finds it (refundvouchers.go).
const advanceNote = "Advance received: "

// voucher issues a receipt voucher for the advance p, paid against proforma
// (nil: against no invoice).
func (h *hooks) voucher(ctx context.Context, p *financepbv1.Payment, proforma *financepbv1.Invoice) error {
	venueName := servicex.ParentOf(p.GetName())
	v, err := h.venue(ctx, venueName)
	if err != nil {
		return err
	}
	if v.GetInvoicePrefix() == "" {
		return nil
	}
	cur, amount := v.GetCurrencyCode(), pricing.Rat(p.GetBaseAmount())
	draft := &financepbv1.Invoice{
		Kind: financepbv1.InvoiceKind_INVOICE_KIND_RECEIPT_VOUCHER, State: financepbv1.Invoice_STATE_DRAFT,
		Contact: p.GetContact(), Booking: p.GetBooking(), Notes: advanceNote + p.GetName(),
	}
	if total := pricing.Rat(proforma.GetTotal()); proforma != nil && total.Sign() > 0 {
		draft.BuyerGst, draft.BillingAddress = proforma.GetBuyerGst(), proforma.GetBillingAddress()
		draft.Quote, draft.SupplyStateCode = proforma.GetQuote(), proforma.GetSupplyStateCode()
		draft.Notes += ", against proforma " + proforma.GetNumber()
		share := new(big.Rat).Quo(amount, total)
		for _, l := range proforma.GetLines() {
			value := new(big.Rat).Mul(pricing.Rat(l.GetTaxableValue()), share)
			draft.Lines = append(draft.Lines, &financepbv1.InvoiceLine{
				Description: clip("Advance: " + l.GetDescription()), Item: l.GetItem(),
				Tax: proto.Clone(l.GetTax()).(*typepbv1.Tax), Quantity: 1, UnitPrice: pricing.Money(value, cur),
			})
		}
	} else {
		value := new(big.Rat).Mul(amount, big.NewRat(100, 100+advanceRate))
		draft.Lines = []*financepbv1.InvoiceLine{{
			Description: clip("Advance for " + p.GetBooking()), Quantity: 1, UnitPrice: pricing.Money(value, cur),
			Tax: &typepbv1.Tax{HsnSacCode: advanceSAC, GstRate: &decimal.Decimal{Value: "18"}},
		}}
	}
	if err := compute(draft, v); err != nil {
		return err
	}
	created, err := h.repos.Invoices.Create(ctx, venueName, draft)
	if err != nil {
		return err
	}
	_, err = h.issue(ctx, created)
	return err
}

// clip is s cut to the 500 characters a line's description may have.
func clip(s string) string {
	if r := []rune(s); len(r) > 500 {
		return string(r[:500])
	}
	return s
}
