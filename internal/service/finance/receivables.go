// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package finance

// receivables.go is SummarizeReceivables: what a venue is owed, as its
// accounts desk reads it, counted on today at the venue -- the outstanding
// of its issued invoices, the part past due, aged in buckets of 30 days, who
// owes the most, and what came in this month. It reads the venue's issued
// invoices in one filtered List (the (state, issue_date) index), this
// month's succeeded payments in another (the (state, receive_time) index),
// and the debtors' contacts in one batched read.
//
// A credit note is PAID as it is issued, bringing down the due of the
// invoice it corrects (issue.go), so the ISSUED invoices are what is owed.

import (
	"context"
	"math/big"
	"slices"
	"strings"
	"time"

	"github.com/shokki-engineering/aperio/internal/database/repository/aperio/guest"
	"github.com/shokki-engineering/aperio/internal/database/repository/repox"
	"github.com/shokki-engineering/aperio/internal/pricing"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/finance/v1/financepbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/guest/v1/guestpbv1"
	"google.golang.org/genproto/googleapis/type/date"
)

// defaultDebtors is how many debtors SummarizeReceivables lists when the
// request says none.
const defaultDebtors = 10

// ages are the aging buckets, in order: the days past due each ends on
// (the last is open-ended).
var ages = []struct {
	age  string
	upTo int
}{{"current", 0}, {"1-30", 30}, {"31-60", 60}, {"61-90", 90}, {"90+", -1}}

// ReadContactsFrom names the debtors SummarizeReceivables lists from
// contacts; without it, a debtor is named by its contact alone.
func (s *Server) ReadContactsFrom(contacts guest.ContactRepository) { s.h.contacts = contacts }

// SummarizeReceivables summarizes what req.Venue is owed, today at the venue.
func (s *Server) SummarizeReceivables(ctx context.Context, req *financepbv1.SummarizeReceivablesRequest) (*financepbv1.SummarizeReceivablesResponse, error) {
	var out *financepbv1.SummarizeReceivablesResponse
	err := s.Options().Run(ctx, "FinanceService", "SummarizeReceivables", func(ctx context.Context) error {
		var err error
		out, err = s.h.summarizeReceivables(ctx, req)
		return err
	})
	return out, err
}

func (h *hooks) summarizeReceivables(ctx context.Context, req *financepbv1.SummarizeReceivablesRequest) (*financepbv1.SummarizeReceivablesResponse, error) {
	v, err := h.venue(ctx, req.GetVenue())
	if err != nil {
		return nil, err
	}
	cur, now := v.GetCurrencyCode(), h.today(v)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	invoices, err := listAll(func(token string) ([]*financepbv1.Invoice, string, error) {
		return h.repos.Invoices.List(ctx, req.GetVenue(), listIn(`state = ISSUED`, token))
	})
	if err != nil {
		return nil, err
	}
	outstanding, overdue := new(big.Rat), new(big.Rat)
	buckets := make([]*big.Rat, len(ages))
	counts := make([]int32, len(ages))
	for i := range buckets {
		buckets[i] = new(big.Rat)
	}
	debtors := map[string]*financepbv1.Debtor{}
	owed := map[string]*big.Rat{}
	for _, inv := range invoices {
		due := pricing.Rat(inv.GetDue())
		outstanding.Add(outstanding, due)
		late := 0
		if d := inv.GetDueDate(); d != nil {
			late = int(today.Sub(dayOf(d)).Hours() / 24)
		}
		if late > 0 {
			overdue.Add(overdue, due)
		}
		b := bucket(late)
		buckets[b].Add(buckets[b], due)
		counts[b]++
		if c := inv.GetContact(); c != "" {
			if debtors[c] == nil {
				debtors[c], owed[c] = &financepbv1.Debtor{Contact: c}, new(big.Rat)
			}
			owed[c].Add(owed[c], due)
			debtors[c].Invoices++
			if d := inv.GetDueDate(); d != nil && (debtors[c].OldestDueDate == nil || dayOf(d).Before(dayOf(debtors[c].OldestDueDate))) {
				debtors[c].OldestDueDate = d
			}
		}
	}
	out := &financepbv1.SummarizeReceivablesResponse{
		Date:        &date.Date{Year: int32(today.Year()), Month: int32(today.Month()), Day: int32(today.Day())},
		Outstanding: pricing.Money(outstanding, cur), Overdue: pricing.Money(overdue, cur),
	}
	for i, a := range ages {
		out.Aging = append(out.Aging, &financepbv1.AgeBucket{Age: a.age, Amount: pricing.Money(buckets[i], cur), Invoices: counts[i]})
	}
	if out.Debtors, err = h.topDebtors(ctx, debtors, owed, cur, req.GetDebtors()); err != nil {
		return nil, err
	}
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	collected, err := h.collected(ctx, req.GetVenue(), monthStart)
	if err != nil {
		return nil, err
	}
	out.Collected = pricing.Money(collected, cur)
	return out, nil
}

// bucket is the index in ages of an invoice late days past due.
func bucket(late int) int {
	for i, a := range ages {
		if a.upTo < 0 || late <= a.upTo {
			return i
		}
	}
	return len(ages) - 1
}

// topDebtors are the n debtors (defaultDebtors when n is 0) owed most, most
// first, each named from its contact, read in one batch.
func (h *hooks) topDebtors(ctx context.Context, debtors map[string]*financepbv1.Debtor, owed map[string]*big.Rat,
	cur string, n int32) ([]*financepbv1.Debtor, error) {
	if n == 0 {
		n = defaultDebtors
	}
	names := make([]string, 0, len(debtors))
	for c := range debtors {
		names = append(names, c)
	}
	slices.SortFunc(names, func(a, b string) int {
		if c := owed[b].Cmp(owed[a]); c != 0 {
			return c
		}
		return strings.Compare(a, b)
	})
	names = names[:min(len(names), int(n))]
	byName := map[string]*guestpbv1.Contact{}
	if h.contacts != nil && len(names) > 0 {
		contacts, err := h.contacts.ListByKeys(ctx, "name", names, repox.ListInput{})
		if err != nil {
			return nil, err
		}
		for _, c := range contacts {
			byName[c.GetName()] = c
		}
	}
	out := make([]*financepbv1.Debtor, 0, len(names))
	for _, name := range names {
		d := debtors[name]
		d.Due = pricing.Money(owed[name], cur)
		if c := byName[name]; c != nil {
			d.DisplayName = c.GetDisplayName()
			if len(c.GetPhoneNumbers()) > 0 {
				d.Phone = c.GetPhoneNumbers()[0]
			}
		}
		out = append(out, d)
	}
	return out, nil
}

// collected is what venueName received from since, in its currency:
// succeeded receipts less succeeded refunds.
func (h *hooks) collected(ctx context.Context, venueName string, since time.Time) (*big.Rat, error) {
	filter := `state = SUCCEEDED AND receive_time >= "` + since.UTC().Format(time.RFC3339) + `"`
	payments, err := listAll(func(token string) ([]*financepbv1.Payment, string, error) {
		return h.repos.Payments.List(ctx, venueName, listIn(filter, token))
	})
	if err != nil {
		return nil, err
	}
	sum := new(big.Rat)
	for _, p := range payments {
		if p.GetKind() == financepbv1.PaymentKind_PAYMENT_KIND_REFUND {
			sum.Sub(sum, pricing.Rat(p.GetBaseAmount()))
		} else {
			sum.Add(sum, pricing.Rat(p.GetBaseAmount()))
		}
	}
	return sum, nil
}

// dayOf is d as midnight UTC.
func dayOf(d *date.Date) time.Time {
	return time.Date(int(d.GetYear()), time.Month(d.GetMonth()), int(d.GetDay()), 0, 0, 0, 0, time.UTC)
}
