// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package guest

// summary.go is SummarizeContact: a guest's history in one call -- "how often
// has John Doe stayed in the villa this year", "does she owe us anything" --
// read from the services that keep it (history.go), each list filtered by the
// guest's contact, which every one of them indexes, across every venue
// (AIP-159's venues/-), and all of them at once.

import (
	"context"
	"math/big"
	"slices"
	"strings"
	"time"

	"github.com/shokki-engineering/aperio/internal/pricing"
	"github.com/shokki-engineering/aperio/internal/service/generated/servicex"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/finance/v1/financepbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/guest/v1/guestpbv1"
	"google.golang.org/genproto/googleapis/type/date"
	"google.golang.org/genproto/googleapis/type/money"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// SummarizeContact summarizes the history of the contact req names.
func (s *Server) SummarizeContact(ctx context.Context, req *guestpbv1.SummarizeContactRequest) (*guestpbv1.SummarizeContactResponse, error) {
	var out *guestpbv1.SummarizeContactResponse
	err := s.Options().Run(ctx, "GuestService", "SummarizeContact", func(ctx context.Context) error {
		if s.history.Stays == nil {
			return status.Error(codes.Unavailable, "guest: a guest's history is not wired to the other services")
		}
		c, err := s.Repositories().Contacts.Get(ctx, req.GetName())
		if err != nil {
			return servicex.NotFoundAs(err, "aperio.guest.v1/Contact", req.GetName())
		}
		out, err = s.summarize(ctx, c, req.GetStartDate())
		return err
	})
	return out, err
}

// summarize sums c's record, counting from start (all of it when nil).
func (s *Server) summarize(ctx context.Context, c *guestpbv1.Contact, start *date.Date) (*guestpbv1.SummarizeContactResponse, error) {
	r, err := s.read(ctx, c.GetName())
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	today := &date.Date{Year: int32(now.Year()), Month: int32(now.Month()), Day: int32(now.Day())}
	out := &guestpbv1.SummarizeContactResponse{Contact: c.GetName(), DisplayName: c.GetDisplayName(), OpenLeads: int32(len(r.leads)), OpenQuotes: int32(len(r.quotes))}
	if len(c.GetPhoneNumbers()) > 0 {
		out.Phone = c.GetPhoneNumbers()[0]
	}
	byType := map[string]int32{}
	for _, st := range r.stays { // newest first
		if !before(st.GetArrivalDate(), today) && (out.NextArrival == nil || before(st.GetArrivalDate(), out.NextArrival)) {
			out.NextArrival = st.GetArrivalDate()
		}
		if !before(today, st.GetDepartureDate()) && (out.LastDeparture == nil || before(out.LastDeparture, st.GetDepartureDate())) {
			out.LastDeparture = st.GetDepartureDate()
		}
		if start != nil && before(st.GetArrivalDate(), start) {
			continue
		}
		out.Stays++
		out.Nights += int32(dayOf(st.GetDepartureDate()).Sub(dayOf(st.GetArrivalDate())).Hours() / 24)
		for _, sp := range st.GetSpaces() {
			byType[sp]++
		}
		for _, sr := range st.GetSpecialRequests() {
			out.Requests = appendOnce(out.Requests, sr.GetSummary())
		}
		for _, dr := range st.GetDietaryRequirements() {
			out.Requests = appendOnce(out.Requests, strings.ToLower(strings.TrimPrefix(dr.GetDiet().String(), "DIET_")))
		}
	}
	for sp, n := range byType {
		name := servicex.IDOf(sp)
		if got, err := s.history.Spaces.Get(ctx, sp); err == nil {
			name = got.GetDisplayName()
		}
		out.RoomTypes = append(out.RoomTypes, &guestpbv1.RoomTypeCount{Space: sp, DisplayName: name, Stays: n})
	}
	slices.SortFunc(out.RoomTypes, func(a, b *guestpbv1.RoomTypeCount) int { return int(b.GetStays() - a.GetStays()) })
	for _, e := range r.events {
		first := e.GetDates().GetStartDate()
		if !before(first, today) && (out.NextEvent == nil || before(first, out.NextEvent)) {
			out.NextEvent = first
		}
		if start == nil || !before(first, start) {
			out.Events++
		}
	}
	paid, due := sums{}, sums{}
	for _, p := range r.payments {
		if start != nil && p.GetCreateTime().AsTime().Before(dayOf(start)) {
			continue
		}
		amount := pricing.Rat(p.GetBaseAmount())
		if p.GetKind() == financepbv1.PaymentKind_PAYMENT_KIND_REFUND {
			amount.Neg(amount)
		}
		paid.add(p.GetBaseAmount().GetCurrencyCode(), amount)
	}
	for _, inv := range r.invoices {
		due.add(inv.GetDue().GetCurrencyCode(), pricing.Rat(inv.GetDue()))
	}
	out.Paid, out.Due = paid.money(), due.money()
	return out, nil
}

// sums are amounts by currency, in the order first met.
type sums struct {
	order []string
	by    map[string]*big.Rat
}

func (s *sums) add(currency string, r *big.Rat) {
	if s.by == nil {
		s.by = map[string]*big.Rat{}
	}
	if _, ok := s.by[currency]; !ok {
		s.order, s.by[currency] = append(s.order, currency), new(big.Rat)
	}
	s.by[currency].Add(s.by[currency], r)
}

func (s *sums) money() []*money.Money {
	var out []*money.Money
	for _, c := range s.order {
		out = append(out, pricing.Money(s.by[c], c))
	}
	return out
}

// before reports whether a is a day before b.
func before(a, b *date.Date) bool { return dayOf(a).Before(dayOf(b)) }

// dayOf is d at UTC midnight.
func dayOf(d *date.Date) time.Time {
	return time.Date(int(d.GetYear()), time.Month(d.GetMonth()), int(d.GetDay()), 0, 0, 0, 0, time.UTC)
}

// appendOnce appends v to vs unless it is empty or there already.
func appendOnce(vs []string, v string) []string {
	if v = strings.TrimSpace(v); v == "" || v == "unspecified" || slices.Contains(vs, v) {
		return vs
	}
	return append(vs, v)
}
