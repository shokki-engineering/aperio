// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package finance

// settle.go is what a payment's settling changes beyond itself:
//
//   - its invoice's paid -- the receipts against it, less their refunds, in
//     the venue's currency -- and due; an invoice paid in full is PAID, and
//     one a refund reopens is ISSUED again;
//   - its booking: a receipt confirms a PENDING stay or a TENTATIVE event,
//     as the advance that holds it.
//
// Paid is summed afresh from the payments each time, never added to, so a
// settle run twice -- a retried webhook -- changes nothing the second time.

import (
	"context"
	"math/big"
	"strings"

	"github.com/shokki-engineering/aperio/internal/pricing"
	"github.com/shokki-engineering/aperio/internal/service/generated/servicex"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/booking/v1/bookingpbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/finance/v1/financepbv1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Confirmer confirms a booking a payment holds: the booking service.
type Confirmer interface {
	GetReservation(context.Context, *bookingpbv1.GetReservationRequest) (*bookingpbv1.Reservation, error)
	ConfirmReservation(context.Context, *bookingpbv1.ConfirmReservationRequest) (*bookingpbv1.Reservation, error)
	GetEvent(context.Context, *bookingpbv1.GetEventRequest) (*bookingpbv1.Event, error)
	ConfirmEvent(context.Context, *bookingpbv1.ConfirmEventRequest) (*bookingpbv1.Event, error)
}

// settle brings p's invoice and booking in line with it.
func (h *hooks) settle(ctx context.Context, p *financepbv1.Payment) error {
	if p.GetInvoice() != "" {
		if err := h.settleInvoice(ctx, p.GetInvoice()); err != nil {
			return err
		}
	}
	if p.GetState() == financepbv1.Payment_STATE_SUCCEEDED && p.GetKind() == financepbv1.PaymentKind_PAYMENT_KIND_RECEIPT {
		return h.confirm(ctx, p.GetBooking())
	}
	return nil
}

// settleInvoice sets name's paid and due from its payments, and its state.
func (h *hooks) settleInvoice(ctx context.Context, name string) error {
	inv, err := h.repos.Invoices.Get(ctx, name)
	if err != nil {
		return err
	}
	if inv.GetState() != financepbv1.Invoice_STATE_ISSUED && inv.GetState() != financepbv1.Invoice_STATE_PAID {
		return nil
	}
	payments, err := listAll(func(token string) ([]*financepbv1.Payment, string, error) {
		return h.repos.Payments.List(ctx, servicex.ParentOf(name), listIn(`invoice = "`+name+`" AND state = SUCCEEDED`, token))
	})
	if err != nil {
		return err
	}
	paid := new(big.Rat)
	for _, p := range payments {
		if p.GetKind() == financepbv1.PaymentKind_PAYMENT_KIND_REFUND {
			paid.Sub(paid, pricing.Rat(p.GetBaseAmount()))
		} else {
			paid.Add(paid, pricing.Rat(p.GetBaseAmount()))
		}
	}
	cur := inv.GetTotal().GetCurrencyCode()
	due := pricing.Sum(pricing.Rat(inv.GetTotal()), new(big.Rat).Neg(paid))
	inv.Paid, inv.Due = pricing.Money(paid, cur), pricing.Money(due, cur)
	inv.State = financepbv1.Invoice_STATE_ISSUED
	if due.Sign() <= 0 {
		inv.State = financepbv1.Invoice_STATE_PAID
	}
	_, err = h.repos.Invoices.UpdateOutputOnly(ctx, inv, []string{"paid", "due", "state"})
	return err
}

// confirm confirms booking, a reservation or an event, when it is waiting on
// its advance; one already confirmed, or moved on, is left as it is.
func (h *hooks) confirm(ctx context.Context, booking string) error {
	if h.confirmer == nil || booking == "" {
		return nil
	}
	var err error
	switch {
	case strings.Contains(booking, "/reservations/"):
		var r *bookingpbv1.Reservation
		if r, err = h.confirmer.GetReservation(ctx, &bookingpbv1.GetReservationRequest{Name: booking}); err == nil &&
			r.GetState() == bookingpbv1.Reservation_STATE_PENDING {
			_, err = h.confirmer.ConfirmReservation(ctx, &bookingpbv1.ConfirmReservationRequest{Name: booking})
		}
	case strings.Contains(booking, "/events/"):
		var e *bookingpbv1.Event
		if e, err = h.confirmer.GetEvent(ctx, &bookingpbv1.GetEventRequest{Name: booking}); err == nil &&
			e.GetState() == bookingpbv1.Event_STATE_TENTATIVE {
			_, err = h.confirmer.ConfirmEvent(ctx, &bookingpbv1.ConfirmEventRequest{Name: booking})
		}
	}
	if status.Code(err) == codes.FailedPrecondition { // confirmed meanwhile
		return nil
	}
	return err
}
