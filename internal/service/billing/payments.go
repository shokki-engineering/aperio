// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package billing

// payments.go is the hooks around a payment's standard methods. A payment is
// recorded in the currency it was paid in -- dollars at the resort's desk,
// rupees over UPI -- and counted in the venue's at its exchange rate
// (base_amount). Its state is the server's:
//
//   - cash, UPI, card, bank transfer and a channel's payout are recorded
//     after the money moved: SUCCEEDED at once;
//   - a gateway payment waits for the gateway's webhook, and a cheque for
//     clearing: PENDING.
//
// A refund is made by RefundPayment, never created directly.

import (
	"context"
	"math/big"

	"github.com/shokki-engineering/aperio/internal/pricing"
	"github.com/shokki-engineering/aperio/internal/service/generated/servicex"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/finance/v1/financepbv1"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// BeforeCreatePayment checks the payment, counts it in the venue's currency
// and sets its state.
func (h *hooks) BeforeCreatePayment(ctx context.Context, parent string, m *financepbv1.Payment) error {
	if m.GetKind() != financepbv1.PaymentKind_PAYMENT_KIND_RECEIPT {
		return servicex.BadField("REFUND_BY_METHOD", "payment.kind", "a refund is made from the payment it returns",
			"Call RefundPayment on the payment")
	}
	if pricing.Rat(m.GetAmount()).Sign() <= 0 {
		return servicex.BadField("INVALID_AMOUNT", "payment.amount", "the amount is not positive", "Record what was received")
	}
	if m.GetMethod() == financepbv1.PaymentMethod_PAYMENT_METHOD_GATEWAY && (m.GetGateway() == "" || m.GetGatewayReference() == "") {
		return servicex.BadField("MISSING_GATEWAY", "payment.gateway_reference", "a gateway payment names no gateway or reference",
			"Set the gateway and the gateway's id for the payment")
	}
	if err := h.checkInvoice(ctx, parent, m); err != nil {
		return err
	}
	if err := h.base(ctx, parent, m); err != nil {
		return err
	}
	switch m.GetMethod() {
	case financepbv1.PaymentMethod_PAYMENT_METHOD_GATEWAY, financepbv1.PaymentMethod_PAYMENT_METHOD_CHEQUE:
		m.State = financepbv1.Payment_STATE_PENDING
	default:
		m.State = financepbv1.Payment_STATE_SUCCEEDED
		if m.GetReceiveTime() == nil {
			m.ReceiveTime = timestamppb.New(h.now())
		}
	}
	return nil
}

// AfterCreatePayment settles a payment that succeeded on being recorded.
func (h *hooks) AfterCreatePayment(ctx context.Context, out *financepbv1.Payment) error {
	return h.settle(ctx, out)
}

// BeforeUpdatePayment refuses to move a settled payment to another invoice
// or booking, or to change its rate: the books already count it.
func (h *hooks) BeforeUpdatePayment(ctx context.Context, m *financepbv1.Payment, paths []string) error {
	if !servicex.TouchesAny(paths, []string{"invoice", "booking", "exchange_rate"}) {
		return nil
	}
	existing, err := h.repos.Payments.Get(ctx, m.GetName())
	if err != nil {
		return servicex.NotFoundAs(err, paymentType, m.GetName())
	}
	if existing.GetState() != financepbv1.Payment_STATE_PENDING {
		return servicex.Blocked("PAYMENT_SETTLED", paymentType, m.GetName(),
			"the payment is "+existing.GetState().String()+"; its invoice, booking and rate are fixed",
			"Void it and record it again")
	}
	return h.checkInvoice(ctx, servicex.ParentOf(m.GetName()), m)
}

// AfterUpdatePayment counts a pending payment again at its rate.
func (h *hooks) AfterUpdatePayment(ctx context.Context, out *financepbv1.Payment) error {
	if out.GetState() != financepbv1.Payment_STATE_PENDING {
		return nil
	}
	if err := h.base(ctx, servicex.ParentOf(out.GetName()), out); err != nil {
		return err
	}
	updated, err := h.repos.Payments.UpdateOutputOnly(ctx, out, []string{"base_amount"})
	if err != nil {
		return err
	}
	proto.Reset(out)
	proto.Merge(out, updated)
	return nil
}

// base sets m's amount in the venue's currency: the amount itself when it
// is in that currency, at exchange_rate otherwise, which is then required.
func (h *hooks) base(ctx context.Context, venueName string, m *financepbv1.Payment) error {
	v, err := h.venue(ctx, venueName)
	if err != nil {
		return err
	}
	cur, rate := v.GetCurrencyCode(), big.NewRat(1, 1)
	if m.GetAmount().GetCurrencyCode() != cur {
		if m.GetExchangeRate() == nil {
			return servicex.BadField("MISSING_EXCHANGE_RATE", "payment.exchange_rate",
				"the payment is in "+m.GetAmount().GetCurrencyCode()+", not the venue's "+cur, "Set how many "+cur+" one unit was worth")
		}
		if rate, err = pricing.Decimal(m.GetExchangeRate()); err != nil || rate.Sign() <= 0 {
			return servicex.BadField("INVALID_EXCHANGE_RATE", "payment.exchange_rate", "the rate is not a positive decimal", `Set it, e.g. "83.25"`)
		}
	}
	m.BaseAmount = pricing.Money(new(big.Rat).Mul(pricing.Rat(m.GetAmount()), rate), cur)
	return nil
}

// checkInvoice checks the invoice m is paid against: of the same venue,
// issued, not a credit note. It takes the invoice's booking when m names
// none.
func (h *hooks) checkInvoice(ctx context.Context, venueName string, m *financepbv1.Payment) error {
	if m.GetInvoice() == "" {
		return nil
	}
	if servicex.ParentOf(m.GetInvoice()) != venueName {
		return servicex.BadField("INVOICE_OF_ANOTHER_VENUE", "payment.invoice", m.GetInvoice()+" is not "+venueName+"'s", "Pay against the venue's invoice")
	}
	inv, err := h.repos.Invoices.Get(ctx, m.GetInvoice())
	if err != nil {
		return servicex.NotFoundAs(err, invoiceType, m.GetInvoice())
	}
	switch {
	case inv.GetKind() == financepbv1.InvoiceKind_INVOICE_KIND_CREDIT_NOTE:
		return servicex.BadField("CREDIT_NOTE", "payment.invoice", "a credit note is not paid", "Refund the payment instead")
	case inv.GetState() != financepbv1.Invoice_STATE_ISSUED && inv.GetState() != financepbv1.Invoice_STATE_PAID:
		return servicex.Blocked("INVOICE_NOT_ISSUED", invoiceType, inv.GetName(), "the invoice is "+inv.GetState().String(),
			"Issue it first, or record the payment without an invoice")
	}
	if m.GetBooking() == "" {
		m.Booking = inv.GetBooking()
	}
	return nil
}
