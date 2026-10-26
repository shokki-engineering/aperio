// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package billing

// states.go moves an invoice through its life: DRAFT -> ISSUED (numbered,
// dated, fixed) -> CANCELLED, its number staying used; and makes a credit
// note, a new draft correcting an issued tax invoice.

import (
	"context"
	"errors"
	"slices"

	"github.com/shokki-engineering/aperio/internal/database/repository/repox"
	"github.com/shokki-engineering/aperio/internal/pricing"
	"github.com/shokki-engineering/aperio/internal/service/generated/servicex"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/finance/v1/financepbv1"
	"google.golang.org/genproto/googleapis/type/date"
	"google.golang.org/protobuf/proto"
)

// issueAttempts is how often IssueInvoice takes the next number when another
// issue took the same one first.
const issueAttempts = 5

// IssueInvoice computes a draft one last time, numbers it in its series and
// dates it today in the venue's time zone.
func (s *Server) IssueInvoice(ctx context.Context, req *financepbv1.IssueInvoiceRequest) (*financepbv1.Invoice, error) {
	return s.move(ctx, "IssueInvoice", req.GetName(), req.GetEtag(), []financepbv1.Invoice_State{financepbv1.Invoice_STATE_DRAFT},
		func(ctx context.Context, inv *financepbv1.Invoice) (*financepbv1.Invoice, error) {
			venueName := servicex.ParentOf(inv.GetName())
			v, err := s.h.venue(ctx, venueName)
			if err != nil {
				return nil, err
			}
			if v.GetInvoicePrefix() == "" {
				return nil, servicex.Blocked("NO_INVOICE_PREFIX", venueType, venueName, "the venue has no invoice series",
					`Set the venue's invoice_prefix, e.g. "SBR"`)
			}
			if err := compute(inv, v); err != nil {
				return nil, err
			}
			day := s.h.today(v)
			stem, err := series(v.GetInvoicePrefix(), inv.GetKind(), day)
			if err != nil {
				return nil, servicex.BadField("INVALID_KIND", "invoice.kind", err.Error(), "Set the kind of document")
			}
			inv.State = financepbv1.Invoice_STATE_ISSUED
			inv.IssueDate = &date.Date{Year: int32(day.Year()), Month: int32(day.Month()), Day: int32(day.Day())}
			etag := inv.GetEtag()
			for range issueAttempts {
				if inv.Number, err = s.h.number(ctx, venueName, stem, inv.GetKind()); err != nil {
					return nil, servicex.Blocked("SERIES_FULL", invoiceType, inv.GetName(), err.Error(), "Change the venue's invoice_prefix to a shorter one")
				}
				inv.Etag = etag
				out, err := s.Repositories().Invoices.UpdateWithOutputOnly(ctx, inv, []string{"lines", "supply_state_code"},
					append([]string{"state", "number", "issue_date"}, totals...))
				if !errors.Is(err, repox.ErrAlreadyExists) {
					return out, err
				}
			}
			return nil, servicex.Blocked("NUMBER_CONTENDED", invoiceType, inv.GetName(),
				"other invoices kept taking the next number", "Issue it again")
		})
}

// CancelInvoice cancels an issued invoice nothing has been paid against.
// Its number stays used: the series keeps no gap.
func (s *Server) CancelInvoice(ctx context.Context, req *financepbv1.CancelInvoiceRequest) (*financepbv1.Invoice, error) {
	return s.move(ctx, "CancelInvoice", req.GetName(), req.GetEtag(), []financepbv1.Invoice_State{financepbv1.Invoice_STATE_ISSUED},
		func(ctx context.Context, inv *financepbv1.Invoice) (*financepbv1.Invoice, error) {
			if pricing.Rat(inv.GetPaid()).Sign() != 0 {
				return nil, servicex.Blocked("INVOICE_PAID", invoiceType, inv.GetName(), "payments have been received against it",
					"Refund them first, or correct it with CreditInvoice")
			}
			inv.State, inv.CancelReason = financepbv1.Invoice_STATE_CANCELLED, req.GetReason()
			return s.Repositories().Invoices.UpdateOutputOnly(ctx, inv, []string{"state", "cancel_reason"})
		})
}

// CreditInvoice makes a draft credit note correcting an issued or paid tax
// invoice: its lines, to be cut down to what is credited, then issued in the
// credit-note series. It returns the draft.
func (s *Server) CreditInvoice(ctx context.Context, req *financepbv1.CreditInvoiceRequest) (*financepbv1.Invoice, error) {
	from := []financepbv1.Invoice_State{financepbv1.Invoice_STATE_ISSUED, financepbv1.Invoice_STATE_PAID}
	var note *financepbv1.Invoice
	_, err := s.move(ctx, "CreditInvoice", req.GetName(), req.GetEtag(), from,
		func(ctx context.Context, inv *financepbv1.Invoice) (*financepbv1.Invoice, error) {
			if inv.GetKind() != financepbv1.InvoiceKind_INVOICE_KIND_TAX_INVOICE {
				return nil, servicex.Blocked("NOT_TAX_INVOICE", invoiceType, inv.GetName(), "only a tax invoice is credited", "Credit the tax invoice")
			}
			venueName := servicex.ParentOf(inv.GetName())
			v, err := s.h.venue(ctx, venueName)
			if err != nil {
				return nil, err
			}
			draft := &financepbv1.Invoice{
				Kind: financepbv1.InvoiceKind_INVOICE_KIND_CREDIT_NOTE, State: financepbv1.Invoice_STATE_DRAFT,
				Contact: inv.GetContact(), BuyerGst: inv.GetBuyerGst(), BillingAddress: inv.GetBillingAddress(),
				Booking: inv.GetBooking(), Quote: inv.GetQuote(), Corrects: inv.GetName(),
				SupplyStateCode: inv.GetSupplyStateCode(), Notes: req.GetReason(),
			}
			for _, l := range inv.GetLines() {
				draft.Lines = append(draft.Lines, proto.Clone(l).(*financepbv1.InvoiceLine))
			}
			if err := compute(draft, v); err != nil {
				return nil, err
			}
			note, err = s.Repositories().Invoices.Create(ctx, venueName, draft)
			return inv, err
		})
	return note, err
}

// move runs one state method: it reads the invoice, checks the caller's etag
// and that it is in one of from, and applies write, which stores the change.
func (s *Server) move(ctx context.Context, method, name, etag string, from []financepbv1.Invoice_State,
	write func(context.Context, *financepbv1.Invoice) (*financepbv1.Invoice, error)) (*financepbv1.Invoice, error) {
	var out *financepbv1.Invoice
	err := s.Options().Run(ctx, "FinanceService", method, func(ctx context.Context) error {
		inv, err := s.Repositories().Invoices.Get(ctx, name)
		if err != nil {
			return servicex.NotFoundAs(err, invoiceType, name)
		}
		if etag != "" {
			if err := servicex.CheckEtag(invoiceType, name, etag, inv.GetEtag()); err != nil {
				return err
			}
		}
		if !slices.Contains(from, inv.GetState()) {
			return servicex.Blocked("INVALID_STATE", invoiceType, name,
				method+" cannot be called on an invoice that is "+inv.GetState().String(), "Read it again and call the method its state allows")
		}
		out, err = write(ctx, inv)
		return err
	})
	return out, err
}
