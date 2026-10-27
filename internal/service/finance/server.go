// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package finance serves aperio.finance.v1 -- GST invoices and payments --
// on the generated AIP server, with the rules the generated layer leaves to
// hooks and custom methods.
//
//   - A draft invoice is computed when it is made and after every edit
//     (gst.go): each line's taxable value, CGST and SGST for a supply within
//     the venue's state or IGST for one outside it, and the totals, the
//     invoice value rounded to the rupee. Only a draft is edited or deleted.
//
//   - IssueInvoice numbers and dates it (series.go): a gap-free series per
//     venue, kind and financial year, e.g. SBR/2627/0042. CancelInvoice
//     keeps the number used; CreditInvoice drafts a credit note correcting
//     an issued tax invoice (states.go).
//
//   - A payment (payments.go) is recorded in the currency it was paid in,
//     counted in the venue's at its exchange rate; once it succeeds, its
//     invoice's paid and due follow, and its booking is confirmed
//     (settle.go). A gateway's webhook settles a gateway payment
//     (gateway.go), RequestPayment asks for one by a link (requests.go),
//     ClearPayment clears a cheque (clearing.go); RefundPayment and
//     VoidPayment undo one (refunds.go). An advance before the tax invoice
//     gets a receipt voucher (advances.go, vouchers.go).
package finance

import (
	"context"
	"time"

	"github.com/shokki-engineering/aperio/internal/database/repository/aperio/guest"
	"github.com/shokki-engineering/aperio/internal/database/repository/aperio/venue"
	"github.com/shokki-engineering/aperio/internal/service/generated/aperio/finance/v1/financepbv1svc"
	"github.com/shokki-engineering/aperio/internal/service/generated/servicex"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/venue/v1/venuepbv1"
)

// Server serves aperio.finance.v1.FinanceService: the generated standard
// methods with the GST hooks, and the invoice state methods.
type Server struct {
	*financepbv1svc.FinanceServiceServer
	h *hooks
}

// New returns the Server over the finance repositories, reading each venue's
// GST registration, currency, time zone and invoice prefix from venues,
// confirming paid bookings through confirmer (nil: bookings are left as they
// are), and refunding through gateways, by name.
func New(repos financepbv1svc.FinanceServiceRepositories, venues venue.VenueRepository, confirmer Confirmer,
	gateways []Gateway, opts ...servicex.Option) *Server {
	h := &hooks{repos: repos, venues: venues, confirmer: confirmer, gateways: map[string]Gateway{}, now: time.Now}
	for _, g := range gateways {
		h.gateways[g.Name()] = g
	}
	return &Server{FinanceServiceServer: financepbv1svc.NewFinanceServiceServer(repos, h, opts...), h: h}
}

// hooks is the business logic around the generated standard methods.
type hooks struct {
	financepbv1svc.NopFinanceServiceHooks

	repos     financepbv1svc.FinanceServiceRepositories
	venues    venue.VenueRepository
	confirmer Confirmer
	gateways  map[string]Gateway
	linker    PaymentLinker           // makes RequestPayment's links; nil: none
	contacts  guest.ContactRepository // names debtors; nil: unnamed
	now       func() time.Time
}

// The resource types errors name.
const (
	invoiceType = "aperio.finance.v1/Invoice"
	paymentType = "aperio.finance.v1/Payment"
	venueType   = "aperio.venue.v1/Venue"
)

// venue reads venueName.
func (h *hooks) venue(ctx context.Context, venueName string) (*venuepbv1.Venue, error) {
	v, err := h.venues.Get(ctx, venueName)
	if err != nil {
		return nil, servicex.NotFoundAs(err, venueType, venueName)
	}
	return v, nil
}

// today is the date now in v's time zone: the date an invoice is issued on.
func (h *hooks) today(v *venuepbv1.Venue) time.Time {
	loc, err := time.LoadLocation(v.GetTimeZone())
	if err != nil {
		loc = time.UTC
	}
	return h.now().In(loc)
}
