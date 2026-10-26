// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package guest

// history.go is what SummarizeContact reads a guest's history from: the
// services that keep it, through their gRPC methods, so each one's rules hold
// -- and the venues' spaces, for the room types' names.

import (
	"context"
	"fmt"

	"github.com/shokki-engineering/aperio/internal/database/repository/aperio/venue"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/event/v1/eventpbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/finance/v1/financepbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/sales/v1/salespbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/stay/v1/staypbv1"
	"golang.org/x/sync/errgroup"
)

// History is where a guest's history is read.
type History struct {
	Stays   Stays
	Events  Events
	Sales   Sales
	Finance Finance
	Spaces  venue.SpaceRepository
}

// Stays lists stays (the StayService).
type Stays interface {
	ListReservations(ctx context.Context, req *staypbv1.ListReservationsRequest) (*staypbv1.ListReservationsResponse, error)
}

// Events lists events (the EventService).
type Events interface {
	ListEvents(ctx context.Context, req *eventpbv1.ListEventsRequest) (*eventpbv1.ListEventsResponse, error)
}

// Sales lists leads and quotes (the SalesService).
type Sales interface {
	ListLeads(ctx context.Context, req *salespbv1.ListLeadsRequest) (*salespbv1.ListLeadsResponse, error)
	ListQuotes(ctx context.Context, req *salespbv1.ListQuotesRequest) (*salespbv1.ListQuotesResponse, error)
}

// Finance lists invoices and payments (the FinanceService).
type Finance interface {
	ListInvoices(ctx context.Context, req *financepbv1.ListInvoicesRequest) (*financepbv1.ListInvoicesResponse, error)
	ListPayments(ctx context.Context, req *financepbv1.ListPaymentsRequest) (*financepbv1.ListPaymentsResponse, error)
}

// pages reads every page list returns, from the first.
func pages[T any](list func(token string) ([]T, string, error)) ([]T, error) {
	var out []T
	token := ""
	for {
		page, next, err := list(token)
		if err != nil {
			return nil, err
		}
		out = append(out, page...)
		if next == "" {
			return out, nil
		}
		token = next
	}
}

// record is everything kept about one guest.
type record struct {
	stays    []*staypbv1.Reservation
	events   []*eventpbv1.Event
	leads    []*salespbv1.Lead
	quotes   []*salespbv1.Quote
	invoices []*financepbv1.Invoice
	payments []*financepbv1.Payment
}

// read reads contact's record from every service at once.
func (s *Server) read(ctx context.Context, contact string) (*record, error) {
	h, r, of := s.history, &record{}, fmt.Sprintf("contact = %q", contact)
	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() (err error) {
		r.stays, err = pages(func(t string) ([]*staypbv1.Reservation, string, error) {
			p, err := h.Stays.ListReservations(ctx, &staypbv1.ListReservationsRequest{Parent: "venues/-", PageSize: 1000, PageToken: t,
				Filter: of + " AND state != CANCELLED AND state != NO_SHOW", OrderBy: "arrival_date desc"})
			return p.GetReservations(), p.GetNextPageToken(), err
		})
		return err
	})
	g.Go(func() (err error) {
		r.events, err = pages(func(t string) ([]*eventpbv1.Event, string, error) {
			p, err := h.Events.ListEvents(ctx, &eventpbv1.ListEventsRequest{Parent: "venues/-", PageSize: 1000, PageToken: t, Filter: of + " AND state != CANCELLED"})
			return p.GetEvents(), p.GetNextPageToken(), err
		})
		return err
	})
	g.Go(func() (err error) {
		r.leads, err = pages(func(t string) ([]*salespbv1.Lead, string, error) {
			p, err := h.Sales.ListLeads(ctx, &salespbv1.ListLeadsRequest{PageSize: 1000, PageToken: t, Filter: of + " AND stage != WON AND stage != LOST"})
			return p.GetLeads(), p.GetNextPageToken(), err
		})
		return err
	})
	g.Go(func() (err error) {
		r.quotes, err = pages(func(t string) ([]*salespbv1.Quote, string, error) {
			p, err := h.Sales.ListQuotes(ctx, &salespbv1.ListQuotesRequest{Parent: "venues/-", PageSize: 1000, PageToken: t,
				Filter: of + " AND state != ACCEPTED AND state != DECLINED AND state != EXPIRED AND state != SUPERSEDED"})
			return p.GetQuotes(), p.GetNextPageToken(), err
		})
		return err
	})
	g.Go(func() (err error) {
		r.invoices, err = pages(func(t string) ([]*financepbv1.Invoice, string, error) {
			p, err := h.Finance.ListInvoices(ctx, &financepbv1.ListInvoicesRequest{Parent: "venues/-", PageSize: 1000, PageToken: t, Filter: of + " AND state = ISSUED"})
			return p.GetInvoices(), p.GetNextPageToken(), err
		})
		return err
	})
	g.Go(func() (err error) {
		r.payments, err = pages(func(t string) ([]*financepbv1.Payment, string, error) {
			p, err := h.Finance.ListPayments(ctx, &financepbv1.ListPaymentsRequest{Parent: "venues/-", PageSize: 1000, PageToken: t, Filter: of + " AND state = SUCCEEDED"})
			return p.GetPayments(), p.GetNextPageToken(), err
		})
		return err
	})
	return r, g.Wait()
}
