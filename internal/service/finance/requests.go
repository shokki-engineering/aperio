// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package finance

// requests.go is RequestPayment: a payment link for an issued invoice -- a
// booking's proforma for its advance, or its tax invoice for the rest --
// from the gateway's adapter (a PaymentLinker). The link is recorded at
// once as a PENDING gateway payment whose gateway_reference is the link's
// id, so the gateway's webhook settles it as Settle settles any (gateway.go).
//
// request_id makes it idempotent (AIP-155): it is the link's own reference
// at the gateway, which gives the same link back for it, and that link's id
// finds the payment already recorded.

import (
	"context"
	"fmt"
	"math/big"
	"time"

	"github.com/google/uuid"
	"github.com/shokki-engineering/aperio/internal/database/repository/repox"
	"github.com/shokki-engineering/aperio/internal/pricing"
	"github.com/shokki-engineering/aperio/internal/service/generated/servicex"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/finance/v1/financepbv1"
	"google.golang.org/genproto/googleapis/type/money"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// defaultLinkTTL is how long a link stands when the request says nothing.
const defaultLinkTTL = 7 * 24 * time.Hour

// PaymentLinker is a gateway's adapter that makes payment links.
type PaymentLinker interface {
	// Name is the gateway as payments name it, e.g. "razorpay".
	Name() string
	// Link makes the link r asks for, or returns the one already made for
	// r.Reference.
	Link(ctx context.Context, r LinkRequest) (Link, error)
}

// LinkRequest is a payment link to make.
type LinkRequest struct {
	Reference   string // ours, unique per link: the request id
	Invoice     string // the invoice it pays, for the gateway's notes
	Amount      *money.Money
	Description string
	Name        string // the payer's, when known
	Phone       string
	Email       string
	Expire      time.Time
}

// Link is a payment link the gateway made.
type Link struct {
	ID     string // the gateway's id for it, which its webhook names
	URL    string
	Amount *money.Money
	Expire time.Time
}

// LinkPaymentsWith makes RequestPayment's links through l; without it,
// RequestPayment is unimplemented.
func (s *Server) LinkPaymentsWith(l PaymentLinker) { s.h.linker = l }

// RequestPayment makes a payment link for an issued invoice's due, or part
// of it, and records the PENDING payment its webhook will settle.
func (s *Server) RequestPayment(ctx context.Context, req *financepbv1.RequestPaymentRequest) (*financepbv1.RequestPaymentResponse, error) {
	var out *financepbv1.RequestPaymentResponse
	err := s.Options().Run(ctx, "FinanceService", "RequestPayment", func(ctx context.Context) error {
		if s.h.linker == nil {
			return status.Error(codes.Unimplemented, "RequestPayment: no payment-link gateway is configured")
		}
		inv, err := s.Repositories().Invoices.Get(ctx, req.GetName())
		if err != nil {
			return servicex.NotFoundAs(err, invoiceType, req.GetName())
		}
		amount, err := requested(inv, req.GetAmount())
		if err != nil {
			return err
		}
		ttl, ref := defaultLinkTTL, req.GetRequestId()
		if req.GetTtl() != nil && req.GetTtl().AsDuration() > 0 {
			ttl = req.GetTtl().AsDuration()
		}
		if ref == "" {
			ref = uuid.NewString()
		}
		lr := LinkRequest{Reference: ref, Invoice: inv.GetName(), Amount: amount, Expire: s.h.now().Add(ttl),
			Description: fmt.Sprintf("Payment for %s", inv.GetNumber())}
		if s.h.contacts != nil && inv.GetContact() != "" {
			if c, err := s.h.contacts.Get(ctx, inv.GetContact()); err == nil {
				lr.Name = c.GetDisplayName()
				if len(c.GetPhoneNumbers()) > 0 {
					lr.Phone = c.GetPhoneNumbers()[0]
				}
				if len(c.GetEmails()) > 0 {
					lr.Email = c.GetEmails()[0]
				}
			}
		}
		link, err := s.h.linker.Link(ctx, lr)
		if err != nil {
			return err
		}
		if err := s.h.recordLink(ctx, inv, link); err != nil {
			return err
		}
		out = &financepbv1.RequestPaymentResponse{Url: link.URL, Gateway: s.h.linker.Name(), GatewayReference: link.ID,
			Amount: link.Amount, ExpireTime: timestamppb.New(link.Expire)}
		return nil
	})
	return out, err
}

// requested is what to ask for on inv: amount, or all that is due, and
// never more; inv must be an issued tax invoice or proforma.
func requested(inv *financepbv1.Invoice, amount *money.Money) (*money.Money, error) {
	if k := inv.GetKind(); k != financepbv1.InvoiceKind_INVOICE_KIND_TAX_INVOICE && k != financepbv1.InvoiceKind_INVOICE_KIND_PROFORMA {
		return nil, servicex.Blocked("NOT_PAYABLE", invoiceType, inv.GetName(), "a "+k.String()+" is not paid", "Request payment on the tax invoice or proforma")
	}
	if inv.GetState() != financepbv1.Invoice_STATE_ISSUED {
		return nil, servicex.Blocked("NOTHING_DUE", invoiceType, inv.GetName(), "the invoice is "+inv.GetState().String(),
			"Request payment on an issued invoice with something due")
	}
	due := pricing.Rat(inv.GetDue())
	if amount == nil {
		amount = inv.GetDue()
	}
	if c := amount.GetCurrencyCode(); c != inv.GetDue().GetCurrencyCode() {
		return nil, servicex.BadField("CURRENCY_MISMATCH", "amount", "the amount is in "+c+", the invoice in "+inv.GetDue().GetCurrencyCode(),
			"Ask in the invoice's currency")
	}
	a := pricing.Rat(amount)
	if a.Sign() <= 0 || a.Cmp(due) > 0 || due.Sign() <= 0 {
		return nil, servicex.BadField("INVALID_AMOUNT", "amount", "the amount is not more than nothing and at most what is due",
			"Ask for a part of the due amount, or leave amount unset for all of it")
	}
	return pricing.Money(new(big.Rat).Set(a), amount.GetCurrencyCode()), nil
}

// recordLink records link as a PENDING gateway payment against inv, unless
// a retry of the same request recorded it already.
func (h *hooks) recordLink(ctx context.Context, inv *financepbv1.Invoice, link Link) error {
	venueName := servicex.ParentOf(inv.GetName())
	filter := fmt.Sprintf(`gateway = %q AND gateway_reference = %q`, h.linker.Name(), link.ID)
	page, _, err := h.repos.Payments.List(ctx, venueName, repox.ListInput{PageSize: 1, Filter: filter})
	if err != nil || len(page) > 0 {
		return err
	}
	p := &financepbv1.Payment{
		Contact: inv.GetContact(), Invoice: inv.GetName(), Booking: inv.GetBooking(),
		Kind: financepbv1.PaymentKind_PAYMENT_KIND_RECEIPT, Method: financepbv1.PaymentMethod_PAYMENT_METHOD_GATEWAY,
		Amount: link.Amount, Gateway: h.linker.Name(), GatewayReference: link.ID, State: financepbv1.Payment_STATE_PENDING,
	}
	if err := h.base(ctx, venueName, p); err != nil {
		return err
	}
	_, err = h.repos.Payments.Create(ctx, venueName, p)
	return err
}
