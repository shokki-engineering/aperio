// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package sales

// invoice.go raises an accepted quote's tax invoice: once AcceptQuote has
// made the bookings and accepted the quote, a DRAFT tax invoice for the
// booking is made through the finance service's CreateInvoice, from the
// quote's lines (invoice_lines.go), naming the booking, the guest and the
// quote, supplied in the quote's state, else the venue's. The finance
// service computes its GST and totals, as it does for any invoice; staff
// check it and issue it.
//
// Its id is the quote's (quote-{id}), so a retried accept -- or two at once
// -- raises one invoice: the second is AlreadyExists, which is success.
//
// Raising it is best effort. The booking is what the guest accepted, and it
// stands whether or not its invoice could be drafted: a failure here does
// not undo the accept (the quote is already ACCEPTED when this runs), it is
// logged, and the invoice is left for staff to raise by hand.

import (
	"context"
	"log/slog"
	"strings"

	"github.com/shokki-engineering/aperio/internal/service/generated/servicex"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/finance/v1/financepbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/sales/v1/salespbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/venue/v1/venuepbv1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Invoices raises the tax invoice of an accepted quote: the finance service.
type Invoices interface {
	CreateInvoice(context.Context, *financepbv1.CreateInvoiceRequest) (*financepbv1.Invoice, error)
}

// InvoiceWith raises each accepted quote's draft tax invoice through
// invoices; until it is called, none is raised.
func (s *Server) InvoiceWith(invoices Invoices) { s.invoices = invoices }

// InvoiceID is the id of the invoice accepting q raises: the quote's own,
// so a retry finds it rather than raising another.
func InvoiceID(q string) string { return "quote-" + strings.ToLower(servicex.IDOf(q)) }

// raiseInvoice drafts accepted quote q's tax invoice, logging a failure:
// the accepted booking stands without it, and staff raise it by hand.
func (s *Server) raiseInvoice(ctx context.Context, q *salespbv1.Quote) {
	if s.invoices == nil || q == nil || q.GetBooking() == "" {
		return
	}
	ctx = context.WithoutCancel(ctx) // the accept is done: finish its invoice
	lines, err := s.invoiceLines(ctx, q)
	state := q.GetSupplyStateCode()
	if err == nil && state == "" { // services rendered at the venue: its state
		var v *venuepbv1.Venue
		if v, err = s.h.venues.Get(ctx, servicex.ParentOf(q.GetName())); err == nil {
			state = v.GetGst().GetGstStateCode()
		}
	}
	if err == nil {
		_, err = s.invoices.CreateInvoice(ctx, &financepbv1.CreateInvoiceRequest{
			Parent:    servicex.ParentOf(q.GetName()),
			InvoiceId: InvoiceID(q.GetName()),
			Invoice: &financepbv1.Invoice{
				Kind:            financepbv1.InvoiceKind_INVOICE_KIND_TAX_INVOICE,
				Contact:         q.GetContact(),
				Booking:         q.GetBooking(),
				Quote:           q.GetName(),
				SupplyStateCode: state,
				Lines:           lines,
			},
		})
	}
	if err != nil && status.Code(err) != codes.AlreadyExists {
		slog.WarnContext(ctx, "sales: the accepted quote's invoice was not raised; raise it by hand",
			"quote", q.GetName(), "booking", q.GetBooking(), "err", err)
	}
}
