// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package finance

// issue.go is what issuing a draft does, whoever issues it -- IssueInvoice,
// or finance itself for a receipt voucher (vouchers.go):
//
//   - it is computed one last time, numbered in its series (series.go) and
//     dated today in the venue's time zone;
//   - a credit note and a receipt voucher are settled the moment they are
//     issued: PAID, paid its total, nothing due. Neither is ever paid
//     against -- a credit note is applied to the invoice it corrects, whose
//     due it brings down (settle.go); a receipt voucher records money already
//     received -- so neither is owed, and neither shows in receivables;
//   - a tax invoice takes over its booking's advances (advances.go).

import (
	"context"
	"errors"
	"math/big"

	"github.com/shokki-engineering/aperio/internal/database/repository/repox"
	"github.com/shokki-engineering/aperio/internal/pricing"
	"github.com/shokki-engineering/aperio/internal/service/generated/servicex"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/finance/v1/financepbv1"
	"google.golang.org/genproto/googleapis/type/date"
)

// issueAttempts is how often a draft takes the next number when another
// issue took the same one first.
const issueAttempts = 5

// settledOnIssue are the kinds that are settled as they are issued.
var settledOnIssue = map[financepbv1.InvoiceKind]bool{
	financepbv1.InvoiceKind_INVOICE_KIND_CREDIT_NOTE:     true,
	financepbv1.InvoiceKind_INVOICE_KIND_RECEIPT_VOUCHER: true,
}

// issue numbers, dates and fixes the draft inv, then brings what it touches
// in line: the invoice a credit note corrects, the advances a tax invoice
// takes over. It returns the issued invoice as it then stands.
func (h *hooks) issue(ctx context.Context, inv *financepbv1.Invoice) (*financepbv1.Invoice, error) {
	venueName := servicex.ParentOf(inv.GetName())
	v, err := h.venue(ctx, venueName)
	if err != nil {
		return nil, err
	}
	if v.GetInvoicePrefix() == "" {
		return nil, servicex.Blocked("NO_INVOICE_PREFIX", venueType, venueName, "the venue has no invoice series",
			`Set the venue's invoice_prefix, e.g. "SBR"`)
	}
	if err := compute(inv, v); err != nil {
		return nil, err
	}
	day := h.today(v)
	stem, err := series(v.GetInvoicePrefix(), inv.GetKind(), day)
	if err != nil {
		return nil, servicex.BadField("INVALID_KIND", "invoice.kind", err.Error(), "Set the kind of document")
	}
	inv.State = financepbv1.Invoice_STATE_ISSUED
	if settledOnIssue[inv.GetKind()] {
		cur := inv.GetTotal().GetCurrencyCode()
		inv.State, inv.Paid, inv.Due = financepbv1.Invoice_STATE_PAID, inv.GetTotal(), pricing.Money(new(big.Rat), cur)
	}
	inv.IssueDate = &date.Date{Year: int32(day.Year()), Month: int32(day.Month()), Day: int32(day.Day())}
	out, err := h.number(ctx, venueName, stem, inv)
	if err != nil {
		return nil, err
	}
	switch out.GetKind() {
	case financepbv1.InvoiceKind_INVOICE_KIND_CREDIT_NOTE:
		return out, h.settleInvoice(ctx, out.GetCorrects())
	case financepbv1.InvoiceKind_INVOICE_KIND_TAX_INVOICE:
		return h.applyAdvances(ctx, out)
	}
	return out, nil
}

// number writes inv with the next number in stem, taking the one after when
// another issue took it first.
func (h *hooks) number(ctx context.Context, venueName, stem string, inv *financepbv1.Invoice) (*financepbv1.Invoice, error) {
	etag := inv.GetEtag()
	for range issueAttempts {
		var err error
		if inv.Number, err = h.next(ctx, venueName, stem, inv.GetKind()); err != nil {
			return nil, servicex.Blocked("SERIES_FULL", invoiceType, inv.GetName(), err.Error(), "Change the venue's invoice_prefix to a shorter one")
		}
		inv.Etag = etag
		out, err := h.repos.Invoices.UpdateWithOutputOnly(ctx, inv, []string{"lines", "supply_state_code"},
			append([]string{"state", "number", "issue_date", "paid"}, totals...))
		if !errors.Is(err, repox.ErrAlreadyExists) {
			return out, err
		}
	}
	return nil, servicex.Blocked("NUMBER_CONTENDED", invoiceType, inv.GetName(),
		"other invoices kept taking the next number", "Issue it again")
}
