// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package finance

// refunds.go undoes a payment, two ways:
//
//   - RefundPayment returns a succeeded receipt in full, as a refund payment
//     of its own that refunds it -- through the gateway for a gateway
//     payment (PENDING until its webhook), at once for any other;
//   - VoidPayment strikes out a payment recorded in error. A gateway payment
//     is never voided: the gateway moved real money, so it is refunded.
//
// Either way the invoice's paid and due follow (settle.go). A booking a
// payment confirmed stays confirmed: cancelling it is the booking's call.

import (
	"context"
	"fmt"
	"slices"

	"github.com/shokki-engineering/aperio/internal/service/generated/servicex"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/finance/v1/financepbv1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// RefundPayment refunds a succeeded receipt in full and returns the refund.
func (s *Server) RefundPayment(ctx context.Context, req *financepbv1.RefundPaymentRequest) (*financepbv1.Payment, error) {
	var refund *financepbv1.Payment
	_, err := s.movePayment(ctx, "RefundPayment", req.GetName(), req.GetEtag(), []financepbv1.Payment_State{financepbv1.Payment_STATE_SUCCEEDED},
		func(ctx context.Context, p *financepbv1.Payment) (*financepbv1.Payment, error) {
			if p.GetKind() != financepbv1.PaymentKind_PAYMENT_KIND_RECEIPT {
				return nil, servicex.Blocked("NOT_A_RECEIPT", paymentType, p.GetName(), "a refund is not refunded", "Refund the receipt")
			}
			venueName := servicex.ParentOf(p.GetName())
			earlier, _, err := s.Repositories().Payments.List(ctx, venueName,
				listIn(fmt.Sprintf(`refunds = %q AND state != FAILED AND state != VOIDED`, p.GetName()), ""))
			if err != nil {
				return nil, err
			}
			if len(earlier) > 0 {
				return nil, servicex.Blocked("ALREADY_REFUNDED", paymentType, p.GetName(), "it is refunded by "+earlier[0].GetName(), "Read that refund")
			}
			r := &financepbv1.Payment{
				Contact: p.GetContact(), Invoice: p.GetInvoice(), Booking: p.GetBooking(),
				Kind: financepbv1.PaymentKind_PAYMENT_KIND_REFUND, Method: p.GetMethod(), Channel: p.GetChannel(),
				Amount: p.GetAmount(), ExchangeRate: p.GetExchangeRate(), BaseAmount: p.GetBaseAmount(),
				Refunds: p.GetName(), Notes: req.GetReason(), State: financepbv1.Payment_STATE_SUCCEEDED,
				ReceiveTime: timestamppb.New(s.h.now()),
			}
			if p.GetMethod() == financepbv1.PaymentMethod_PAYMENT_METHOD_GATEWAY {
				g, ok := s.h.gateways[p.GetGateway()]
				if !ok {
					return nil, servicex.Blocked("NO_GATEWAY", paymentType, p.GetName(), "aperio has no adapter for "+p.GetGateway(),
						"Refund it in the gateway's dashboard and record the refund there")
				}
				if r.GatewayReference, err = g.Refund(ctx, p.GetGatewayReference(), p.GetAmount()); err != nil {
					return nil, err
				}
				r.Gateway, r.State, r.ReceiveTime = p.GetGateway(), financepbv1.Payment_STATE_PENDING, nil
			}
			if refund, err = s.Repositories().Payments.Create(ctx, venueName, r); err != nil {
				return nil, err
			}
			return p, s.h.settle(ctx, refund)
		})
	return refund, err
}

// VoidPayment strikes out a payment recorded in error.
func (s *Server) VoidPayment(ctx context.Context, req *financepbv1.VoidPaymentRequest) (*financepbv1.Payment, error) {
	from := []financepbv1.Payment_State{financepbv1.Payment_STATE_PENDING, financepbv1.Payment_STATE_SUCCEEDED}
	return s.movePayment(ctx, "VoidPayment", req.GetName(), req.GetEtag(), from,
		func(ctx context.Context, p *financepbv1.Payment) (*financepbv1.Payment, error) {
			if p.GetMethod() == financepbv1.PaymentMethod_PAYMENT_METHOD_GATEWAY {
				return nil, servicex.Blocked("GATEWAY_PAYMENT", paymentType, p.GetName(), "the gateway moved the money", "Refund it with RefundPayment")
			}
			p.State = financepbv1.Payment_STATE_VOIDED
			out, err := s.Repositories().Payments.UpdateOutputOnly(ctx, p, []string{"state"})
			if err != nil {
				return nil, err
			}
			if req.GetReason() != "" {
				out.Notes = "Voided: " + req.GetReason()
				if out, err = s.Repositories().Payments.Update(ctx, out, []string{"notes"}); err != nil {
					return nil, err
				}
			}
			return out, s.h.settle(ctx, out)
		})
}

// movePayment runs one payment state method, as move does for invoices.
func (s *Server) movePayment(ctx context.Context, method, name, etag string, from []financepbv1.Payment_State,
	write func(context.Context, *financepbv1.Payment) (*financepbv1.Payment, error)) (*financepbv1.Payment, error) {
	var out *financepbv1.Payment
	err := s.Options().Run(ctx, "FinanceService", method, func(ctx context.Context) error {
		p, err := s.Repositories().Payments.Get(ctx, name)
		if err != nil {
			return servicex.NotFoundAs(err, paymentType, name)
		}
		if etag != "" {
			if err := servicex.CheckEtag(paymentType, name, etag, p.GetEtag()); err != nil {
				return err
			}
		}
		if !slices.Contains(from, p.GetState()) {
			return servicex.Blocked("INVALID_STATE", paymentType, name,
				method+" cannot be called on a payment that is "+p.GetState().String(), "Read it again and call the method its state allows")
		}
		out, err = write(ctx, p)
		return err
	})
	return out, err
}
