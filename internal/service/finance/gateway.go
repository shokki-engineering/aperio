// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package finance

// gateway.go is aperio's side of a payment gateway, whichever it is
// (Razorpay, Cashfree, Stripe). An adapter does what is the gateway's own --
// verify its webhook's signature, read its body, refund through its API --
// and hands aperio what happened as an Outcome; Settle records it.
//
// A gateway payment is recorded PENDING when it is started (CreatePayment
// with the gateway's order or payment id as gateway_reference) and settled
// by its webhook. The unique (gateway, gateway_reference) index means a
// retried webhook finds the same payment, and Settle's no-op on a payment
// already where the outcome would put it makes a retry harmless.

import (
	"context"
	"fmt"
	"time"

	"github.com/shokki-engineering/aperio/internal/database/repository/repox"
	"github.com/shokki-engineering/aperio/internal/service/generated/servicex"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/finance/v1/financepbv1"
	"google.golang.org/genproto/googleapis/type/money"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Gateway is what aperio asks of a payment gateway's adapter.
type Gateway interface {
	// Name is the gateway as payments name it, e.g. "razorpay".
	Name() string
	// Refund returns amount of the payment the gateway knows as reference,
	// and gives the refund's own reference, which its webhook will name.
	Refund(ctx context.Context, reference string, amount *money.Money) (string, error)
}

// Outcome is what a gateway's webhook reports about a payment or a refund.
type Outcome struct {
	Gateway   string // the gateway's Name
	Reference string // its id for the payment or refund
	Succeeded bool   // the money moved; false: it failed
	At        time.Time
}

// Settle records o on the payment it names, at venueName -- the webhook
// route says which venue -- and settles that payment's invoice and booking.
func (s *Server) Settle(ctx context.Context, venueName string, o Outcome) (*financepbv1.Payment, error) {
	var out *financepbv1.Payment
	err := s.Options().Run(ctx, "FinanceService", "Settle", func(ctx context.Context) error {
		filter := fmt.Sprintf(`gateway = %q AND gateway_reference = %q`, o.Gateway, o.Reference)
		page, _, err := s.Repositories().Payments.List(ctx, venueName, repox.ListInput{PageSize: 1, Filter: filter})
		if err != nil {
			return err
		}
		if len(page) == 0 {
			return servicex.NotFoundAs(repox.ErrNotFound, paymentType, o.Gateway+"/"+o.Reference)
		}
		p := page[0]
		want := financepbv1.Payment_STATE_FAILED
		if o.Succeeded {
			want = financepbv1.Payment_STATE_SUCCEEDED
		}
		if p.GetState() == want {
			out = p // a retried webhook
			return nil
		}
		if p.GetState() != financepbv1.Payment_STATE_PENDING {
			return servicex.Blocked("PAYMENT_SETTLED", paymentType, p.GetName(),
				"the payment is already "+p.GetState().String(), "Look into the gateway's records")
		}
		p.State, p.ReceiveTime = want, timestamppb.New(o.At)
		if out, err = s.Repositories().Payments.UpdateOutputOnly(ctx, p, []string{"state"}); err != nil {
			return err
		}
		if out, err = s.Repositories().Payments.Update(ctx, out, []string{"receive_time"}); err != nil {
			return err
		}
		return s.h.settle(ctx, out)
	})
	return out, err
}

// listIn is a page of filter from token.
func listIn(filter, token string) repox.ListInput {
	return repox.ListInput{PageSize: 200, Filter: filter, PageToken: token}
}

// listAll reads every page of a List.
func listAll[T any](list func(token string) ([]T, string, error)) ([]T, error) {
	var out []T
	for token := ""; ; {
		page, next, err := list(token)
		if err != nil {
			return nil, err
		}
		out = append(out, page...)
		if token = next; token == "" {
			return out, nil
		}
	}
}
