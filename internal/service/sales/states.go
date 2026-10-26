// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package sales

// states.go moves a quote through its life: DRAFT -> SENT (prices fixed,
// the offer's clock started) -> DECLINED, or SENT/DECLINED/EXPIRED ->
// SUPERSEDED by a new draft ReviseQuote makes from it.

import (
	"context"
	"slices"

	"github.com/shokki-engineering/aperio/internal/service/generated/servicex"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/sales/v1/salespbv1"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// SendQuote prices a draft one last time and fixes it: what the guest is
// sent is what they are held to, whatever the rules do after.
func (s *Server) SendQuote(ctx context.Context, req *salespbv1.SendQuoteRequest) (*salespbv1.Quote, error) {
	return s.move(ctx, "SendQuote", req.GetName(), req.GetEtag(), []salespbv1.Quote_State{salespbv1.Quote_STATE_DRAFT},
		func(ctx context.Context, q *salespbv1.Quote) (*salespbv1.Quote, error) {
			if err := s.h.reprice(ctx, servicex.ParentOf(q.GetName()), q); err != nil {
				return nil, err
			}
			now := s.h.now()
			q.State = salespbv1.Quote_STATE_SENT
			q.SendTime = timestamppb.New(now)
			q.ExpireTime = timestamppb.New(now.Add(offerTTL))
			return s.Repositories().Quotes.UpdateWithOutputOnly(ctx, q, []string{"lines"},
				append([]string{"state", "send_time", "expire_time"}, totals...))
		})
}

// DeclineQuote records that the guest declined a sent quote.
func (s *Server) DeclineQuote(ctx context.Context, req *salespbv1.DeclineQuoteRequest) (*salespbv1.Quote, error) {
	return s.move(ctx, "DeclineQuote", req.GetName(), req.GetEtag(), []salespbv1.Quote_State{salespbv1.Quote_STATE_SENT},
		func(ctx context.Context, q *salespbv1.Quote) (*salespbv1.Quote, error) {
			q.State = salespbv1.Quote_STATE_DECLINED
			q.DeclineReason = req.GetReason()
			return s.Repositories().Quotes.UpdateOutputOnly(ctx, q, []string{"state", "decline_reason"})
		})
}

// ReviseQuote makes a new draft from a sent, declined or expired quote --
// its lines, priced again for today's rules -- and marks the old one
// superseded. It returns the new draft.
func (s *Server) ReviseQuote(ctx context.Context, req *salespbv1.ReviseQuoteRequest) (*salespbv1.Quote, error) {
	from := []salespbv1.Quote_State{salespbv1.Quote_STATE_SENT, salespbv1.Quote_STATE_DECLINED, salespbv1.Quote_STATE_EXPIRED}
	var draft *salespbv1.Quote
	_, err := s.move(ctx, "ReviseQuote", req.GetName(), req.GetEtag(), from,
		func(ctx context.Context, q *salespbv1.Quote) (*salespbv1.Quote, error) {
			next := proto.Clone(q).(*salespbv1.Quote)
			next.Name, next.Etag, next.Supersedes = "", "", q.GetName()
			next.State = salespbv1.Quote_STATE_DRAFT
			next.SendTime, next.ExpireTime, next.AcceptTime, next.CreateTime, next.UpdateTime = nil, nil, nil, nil, nil
			next.Booking, next.DeclineReason = "", ""
			for _, l := range next.GetLines() {
				if !l.GetPriceOverride() && l.GetItem() != "" {
					l.UnitPrice = nil // priced again from the rules
				}
			}
			parent := servicex.ParentOf(q.GetName())
			if err := s.h.reprice(ctx, parent, next); err != nil {
				return nil, err
			}
			var err error
			if draft, err = s.Repositories().Quotes.Create(ctx, parent, next); err != nil {
				return nil, err
			}
			q.State = salespbv1.Quote_STATE_SUPERSEDED
			return s.Repositories().Quotes.UpdateOutputOnly(ctx, q, []string{"state"})
		})
	return draft, err
}

// move runs one state method: it reads the quote, checks the caller's etag
// and that it is in one of from, and applies write, which stores the change.
func (s *Server) move(ctx context.Context, method, name, etag string, from []salespbv1.Quote_State,
	write func(context.Context, *salespbv1.Quote) (*salespbv1.Quote, error)) (*salespbv1.Quote, error) {
	var out *salespbv1.Quote
	err := s.Options().Run(ctx, "SalesService", method, func(ctx context.Context) error {
		q, err := s.Repositories().Quotes.Get(ctx, name)
		if err != nil {
			return servicex.NotFoundAs(err, quoteType, name)
		}
		if etag != "" {
			if err := servicex.CheckEtag(quoteType, name, etag, q.GetEtag()); err != nil {
				return err
			}
		}
		if !slices.Contains(from, q.GetState()) {
			return servicex.Blocked("INVALID_STATE", quoteType, name,
				method+" cannot be called on a quote that is "+q.GetState().String(), "Read it again and call the method its state allows")
		}
		out, err = write(ctx, q)
		return err
	})
	return out, err
}
