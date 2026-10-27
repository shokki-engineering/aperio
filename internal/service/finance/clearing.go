// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package finance

// clearing.go is ClearPayment: a cheque (or a bank transfer recorded before
// the bank credited it) is PENDING until the bank says the money is in;
// clearing it makes it SUCCEEDED, with what any receipt that succeeds
// brings -- its invoice's paid and due, an advance's receipt voucher, its
// booking confirmed (settle.go).

import (
	"context"

	"github.com/shokki-engineering/aperio/internal/service/generated/servicex"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/finance/v1/financepbv1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// ClearPayment clears a PENDING cheque or bank transfer.
func (s *Server) ClearPayment(ctx context.Context, req *financepbv1.ClearPaymentRequest) (*financepbv1.Payment, error) {
	return s.movePayment(ctx, "ClearPayment", req.GetName(), req.GetEtag(), []financepbv1.Payment_State{financepbv1.Payment_STATE_PENDING},
		func(ctx context.Context, p *financepbv1.Payment) (*financepbv1.Payment, error) {
			if m := p.GetMethod(); m != financepbv1.PaymentMethod_PAYMENT_METHOD_CHEQUE && m != financepbv1.PaymentMethod_PAYMENT_METHOD_BANK_TRANSFER {
				return nil, servicex.Blocked("NOT_CLEARED_BY_BANK", paymentType, p.GetName(), "a "+m.String()+" payment is not cleared by hand",
					"Wait for the gateway's webhook")
			}
			p.State = financepbv1.Payment_STATE_SUCCEEDED
			out, err := s.Repositories().Payments.UpdateOutputOnly(ctx, p, []string{"state"})
			if err != nil {
				return nil, err
			}
			if out.GetReceiveTime() == nil {
				out.ReceiveTime = timestamppb.New(s.h.now())
				if out, err = s.Repositories().Payments.Update(ctx, out, []string{"receive_time"}); err != nil {
					return nil, err
				}
			}
			return out, s.h.settle(ctx, out)
		})
}
