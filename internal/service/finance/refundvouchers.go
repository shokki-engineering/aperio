// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package finance

// refundvouchers.go issues the refund voucher an advance's refund needs
// (CGST Rules, rule 51): when an advance was received and a receipt voucher
// issued for it (vouchers.go), but no tax invoice followed, the refund of
// that advance is documented by a refund voucher against the receipt
// voucher, which takes back the tax the receipt voucher declared.
//
// aperio has no INVOICE_KIND of its own for a refund voucher, so it is made
// as a credit note whose corrects is the receipt voucher: the document that
// reverses another, numbered in the credit-note series (SBR/CN2627/0007),
// with the receipt voucher's lines -- the refund is of the whole advance --
// and notes that name it a refund voucher under rule 51 and the refund it
// documents. Like any credit note it is PAID as it is issued (issue.go),
// and the receipt voucher it corrects, settled on issue itself, is left as
// it is.
//
// An advance a tax invoice has taken over (advances.go) is no longer one:
// its refund is a matter for a credit note on that tax invoice
// (CreditInvoice), and gets no refund voucher here. A refund is settled
// once per state change, but a retried settle finds the voucher already
// made and issues none twice.

import (
	"context"
	"fmt"
	"strings"

	"github.com/shokki-engineering/aperio/internal/service/generated/servicex"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/finance/v1/financepbv1"
	"google.golang.org/protobuf/proto"
)

// refundVoucher issues the rule 51 voucher for the refund r, which has just
// succeeded, when what it refunds was an advance with a receipt voucher.
func (h *hooks) refundVoucher(ctx context.Context, r *financepbv1.Payment) error {
	if r.GetBooking() == "" || r.GetRefunds() == "" {
		return nil
	}
	if r.GetInvoice() != "" {
		inv, err := h.repos.Invoices.Get(ctx, r.GetInvoice())
		if err != nil {
			return err
		}
		if inv.GetKind() == financepbv1.InvoiceKind_INVOICE_KIND_TAX_INVOICE {
			return nil // the tax invoice took the advance over
		}
	}
	rv, err := h.receiptVoucherOf(ctx, r)
	if err != nil || rv == nil {
		return err
	}
	venueName := servicex.ParentOf(r.GetName())
	earlier, err := listAll(func(token string) ([]*financepbv1.Invoice, string, error) {
		return h.repos.Invoices.List(ctx, venueName,
			listIn(fmt.Sprintf(`corrects = %q AND kind = CREDIT_NOTE AND state != CANCELLED`, rv.GetName()), token))
	})
	if err != nil {
		return err
	}
	if len(earlier) > 0 {
		if n := earlier[0]; n.GetState() == financepbv1.Invoice_STATE_DRAFT {
			_, err := h.issue(ctx, n) // made, but its issue failed
			return err
		}
		return nil // issued already, by an earlier settle
	}
	v, err := h.venue(ctx, venueName)
	if err != nil {
		return err
	}
	draft := &financepbv1.Invoice{
		Kind: financepbv1.InvoiceKind_INVOICE_KIND_CREDIT_NOTE, State: financepbv1.Invoice_STATE_DRAFT,
		Contact: rv.GetContact(), BuyerGst: rv.GetBuyerGst(), BillingAddress: rv.GetBillingAddress(),
		Booking: rv.GetBooking(), Quote: rv.GetQuote(), Corrects: rv.GetName(), SupplyStateCode: rv.GetSupplyStateCode(),
		Notes: fmt.Sprintf("Refund voucher (CGST Rules, rule 51): refund %s of the advance on receipt voucher %s",
			r.GetName(), rv.GetNumber()),
	}
	for _, l := range rv.GetLines() {
		line := proto.Clone(l).(*financepbv1.InvoiceLine)
		line.Description = clip("Refund of " + strings.TrimPrefix(l.GetDescription(), "Advance: "))
		draft.Lines = append(draft.Lines, line)
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

// receiptVoucherOf is the issued receipt voucher for the payment the refund
// r returns, nil when it has none.
func (h *hooks) receiptVoucherOf(ctx context.Context, r *financepbv1.Payment) (*financepbv1.Invoice, error) {
	vouchers, err := listAll(func(token string) ([]*financepbv1.Invoice, string, error) {
		return h.repos.Invoices.List(ctx, servicex.ParentOf(r.GetName()),
			listIn(issuedOf(r.GetBooking(), financepbv1.InvoiceKind_INVOICE_KIND_RECEIPT_VOUCHER), token))
	})
	if err != nil {
		return nil, err
	}
	mark := advanceNote + r.GetRefunds()
	for _, rv := range vouchers {
		if n := rv.GetNotes(); n == mark || strings.HasPrefix(n, mark+",") {
			return rv, nil
		}
	}
	return nil, nil
}
