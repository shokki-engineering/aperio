// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package sales

// holds.go is what AcceptQuote and freebusy's webhook do with the holds a
// lead and an accepted quote keep (leads.go has why): the lead's rooms are
// let go before the quote's bookings take them, the quote's halls are held
// for its event's days, and a lead whose hold lapsed in freebusy forgets it.

import (
	"context"
	"fmt"

	"github.com/shokki-engineering/aperio/internal/database/repository/repox"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/sales/v1/salespbv1"
)

// HoldLapsed forgets hold on the lead that kept it: freebusy expired or
// released it. It is not an RPC; freebusy's webhook calls it.
func (s *Server) HoldLapsed(ctx context.Context, hold string) error {
	return s.Options().Run(ctx, "SalesService", "HoldLapsed", func(ctx context.Context) error {
		leads, _, err := s.h.repos.Leads.List(ctx, repox.ListInput{PageSize: 10, Filter: fmt.Sprintf("hold = %q", hold)})
		if err != nil {
			return err
		}
		for _, l := range leads {
			l.Hold = ""
			if _, err := s.h.repos.Leads.UpdateOutputOnly(ctx, l, []string{"hold"}); err != nil {
				return err
			}
		}
		return nil
	})
}

// releaseLead lets go of lead's hold and forgets it, so the bookings of the
// quote it accepts can take its rooms; it returns the lead as it was, for
// holdAgain should the bookings fail.
func (s *Server) releaseLead(ctx context.Context, lead string) *salespbv1.Lead {
	if s.h.base == nil || lead == "" {
		return nil
	}
	l, err := s.h.repos.Leads.Get(ctx, lead)
	if err != nil || l.GetHold() == "" {
		return nil
	}
	s.h.letGo(ctx, lead, l.GetHold())
	l.Hold = ""
	if _, err := s.h.repos.Leads.UpdateOutputOnly(ctx, l, []string{"hold"}); err != nil {
		return nil
	}
	return l
}

// holdAgain takes l's hold again after a failed accept gave it up.
func (s *Server) holdAgain(ctx context.Context, l *salespbv1.Lead) {
	if l != nil {
		s.h.keepHold(context.WithoutCancel(ctx), l)
	}
}

// holdHalls holds every space q sells for event's days, the quote's dates:
// a hall another accepted event has for a day of them fails the accept.
func (s *Server) holdHalls(ctx context.Context, venueName, event string, q *salespbv1.Quote) error {
	if s.h.base == nil || event == "" {
		return nil
	}
	for i, l := range q.GetLines() {
		if l.GetKind() != salespbv1.LineKind_LINE_KIND_SPACE || l.GetItem() == "" {
			continue
		}
		if _, err := s.h.base.HoldHall(ctx, venueName, event, fmt.Sprintf("quote.lines[%d].item", i), l.GetItem(), q.GetDates()); err != nil {
			return err
		}
	}
	return nil
}
