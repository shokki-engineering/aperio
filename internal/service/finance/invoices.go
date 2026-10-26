// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package finance

// invoices.go is the hooks around the standard methods: a draft is computed
// when it is made and after every edit; an issued invoice is a legal record,
// so it is not edited -- it is cancelled, or credited. (The API has no
// DeleteInvoice: a number, once issued, stays accounted for.)

import (
	"context"

	"github.com/shokki-engineering/aperio/internal/service/generated/servicex"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/finance/v1/financepbv1"
	"google.golang.org/protobuf/proto"
)

// BeforeCreateInvoice computes the new draft; Create stores what it sets.
func (h *hooks) BeforeCreateInvoice(ctx context.Context, parent string, m *financepbv1.Invoice) error {
	if m.GetKind() == financepbv1.InvoiceKind_INVOICE_KIND_CREDIT_NOTE {
		return servicex.BadField("CREDIT_NOTE_BY_METHOD", "invoice.kind", "a credit note is made from the invoice it corrects",
			"Call CreditInvoice on the tax invoice")
	}
	v, err := h.venue(ctx, parent)
	if err != nil {
		return err
	}
	return compute(m, v)
}

// BeforeUpdateInvoice refuses to edit an invoice that is no longer a draft.
func (h *hooks) BeforeUpdateInvoice(ctx context.Context, m *financepbv1.Invoice, paths []string) error {
	return h.draftOnly(ctx, m.GetName(), "edited")
}

// AfterUpdateInvoice computes the edited draft again.
func (h *hooks) AfterUpdateInvoice(ctx context.Context, out *financepbv1.Invoice) error {
	v, err := h.venue(ctx, servicex.ParentOf(out.GetName()))
	if err != nil {
		return err
	}
	if err := compute(out, v); err != nil {
		return err
	}
	updated, err := h.repos.Invoices.UpdateWithOutputOnly(ctx, out, []string{"lines", "supply_state_code"}, totals)
	if err != nil {
		return err
	}
	proto.Reset(out)
	proto.Merge(out, updated)
	return nil
}

// draftOnly fails unless invoice name is a draft.
func (h *hooks) draftOnly(ctx context.Context, name, what string) error {
	inv, err := h.repos.Invoices.Get(ctx, name)
	if err != nil {
		return servicex.NotFoundAs(err, invoiceType, name)
	}
	if inv.GetState() != financepbv1.Invoice_STATE_DRAFT {
		return servicex.Blocked("INVOICE_NOT_DRAFT", invoiceType, name,
			"the invoice is "+inv.GetState().String()+"; only a draft is "+what,
			"Cancel it with CancelInvoice, or correct it with CreditInvoice")
	}
	return nil
}
