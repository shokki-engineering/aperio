// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package finance

// advances.go is GST on advances. Money received for a booking before its
// tax invoice is an advance -- paid against the booking's proforma (the
// link RequestPayment sends), or against no invoice at all -- and GST is
// due on it when it is received (CGST Act, section 13), on a receipt voucher
// (CGST Rules, rule 50; vouchers.go).
//
// When the booking's tax invoice is issued it takes the advances over: the
// booking's payments against its proformas, or against nothing, are counted
// against the tax invoice instead, so its paid and due say what is left to
// pay; and the proformas it supersedes are cancelled, so what is owed is not
// counted twice in receivables. A payment still pending then -- a link
// paid after the invoice -- moves onto the tax invoice when it succeeds.

import (
	"context"

	"github.com/shokki-engineering/aperio/internal/database/repository/repox"
	"github.com/shokki-engineering/aperio/internal/service/generated/servicex"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/finance/v1/financepbv1"
)

// issuedOf is the filter for the booking's issued documents of kind.
func issuedOf(booking string, kind financepbv1.InvoiceKind) string {
	return `booking = "` + booking + `" AND kind = ` + kind.String() + ` AND state != DRAFT AND state != CANCELLED`
}

// advance records the receipt p, which has just succeeded, as an advance
// when it is one: a receipt voucher for it while its booking has no tax
// invoice, or moved onto the tax invoice once it has.
func (h *hooks) advance(ctx context.Context, p *financepbv1.Payment) error {
	if p.GetBooking() == "" {
		return nil
	}
	var proforma *financepbv1.Invoice
	if p.GetInvoice() != "" {
		inv, err := h.repos.Invoices.Get(ctx, p.GetInvoice())
		if err != nil {
			return err
		}
		if inv.GetKind() != financepbv1.InvoiceKind_INVOICE_KIND_PROFORMA {
			return nil // paid against the tax invoice: not an advance
		}
		proforma = inv
	}
	venueName := servicex.ParentOf(p.GetName())
	page, _, err := h.repos.Invoices.List(ctx, venueName, repox.ListInput{PageSize: 1, OrderBy: "number",
		Filter: issuedOf(p.GetBooking(), financepbv1.InvoiceKind_INVOICE_KIND_TAX_INVOICE)})
	if err != nil {
		return err
	}
	if len(page) == 0 {
		return h.voucher(ctx, p, proforma)
	}
	p.Invoice = page[0].GetName()
	if _, err := h.repos.Payments.Update(ctx, p, []string{"invoice"}); err != nil {
		return err
	}
	return h.settleInvoice(ctx, p.GetInvoice())
}

// applyAdvances has the tax invoice inv, just issued, take over its
// booking's advances and supersede its proformas. It returns inv as it then
// stands.
func (h *hooks) applyAdvances(ctx context.Context, inv *financepbv1.Invoice) (*financepbv1.Invoice, error) {
	booking := inv.GetBooking()
	if booking == "" {
		return inv, nil
	}
	venueName := servicex.ParentOf(inv.GetName())
	proformas, err := listAll(func(token string) ([]*financepbv1.Invoice, string, error) {
		return h.repos.Invoices.List(ctx, venueName, listIn(issuedOf(booking, financepbv1.InvoiceKind_INVOICE_KIND_PROFORMA), token))
	})
	if err != nil {
		return nil, err
	}
	superseded := map[string]bool{"": true}
	for _, pf := range proformas {
		superseded[pf.GetName()] = true
	}
	payments, err := listAll(func(token string) ([]*financepbv1.Payment, string, error) {
		return h.repos.Payments.List(ctx, venueName, listIn(`booking = "`+booking+`" AND state != FAILED AND state != VOIDED`, token))
	})
	if err != nil {
		return nil, err
	}
	for _, p := range payments {
		if !superseded[p.GetInvoice()] {
			continue
		}
		p.Invoice = inv.GetName()
		if _, err := h.repos.Payments.Update(ctx, p, []string{"invoice"}); err != nil {
			return nil, err
		}
	}
	for _, pf := range proformas {
		pf.State, pf.CancelReason = financepbv1.Invoice_STATE_CANCELLED, "Superseded by tax invoice "+inv.GetNumber()
		if _, err := h.repos.Invoices.UpdateOutputOnly(ctx, pf, []string{"state", "cancel_reason"}); err != nil {
			return nil, err
		}
	}
	if err := h.settleInvoice(ctx, inv.GetName()); err != nil {
		return nil, err
	}
	return h.repos.Invoices.Get(ctx, inv.GetName())
}
