// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package sales_test

// A live test of an offer's clock: SendQuote's ttl sets expire_time, 14
// days without one, and the expiry job lapses a sent quote past it. Run as
// sales_live_test.go says.

import (
	"context"
	"testing"
	"time"

	"github.com/shokki-engineering/aperio/protobuf/generated/go/sales/v1/salespbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/type/v1/typepbv1"
	"google.golang.org/genproto/googleapis/type/date"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestLiveQuoteTTLAndExpiry(t *testing.T) {
	f, ctx := holdSetup(t), context.Background()
	draft := func() *salespbv1.Quote {
		q, err := f.svc.CreateQuote(ctx, &salespbv1.CreateQuoteRequest{Parent: f.venue, Quote: &salespbv1.Quote{
			DisplayName: "Doe stay", Contact: f.contact, Dates: dateRange(2027, 10, 1, 3), Guests: 2,
			Lines: []*salespbv1.QuoteLine{{Kind: salespbv1.LineKind_LINE_KIND_SERVICE, Description: "Airport transfer", Quantity: 1, UnitPrice: inr(2500)}},
		}})
		if err != nil {
			t.Fatalf("create quote: %v", err)
		}
		return q
	}
	q, err := f.svc.SendQuote(ctx, &salespbv1.SendQuoteRequest{Name: draft().GetName(), Ttl: durationpb.New(48 * time.Hour)})
	if err != nil {
		t.Fatalf("send with ttl: %v", err)
	}
	if got := q.GetExpireTime().AsTime().Sub(q.GetSendTime().AsTime()); got != 48*time.Hour {
		t.Errorf("expire_time is %v after send_time; want 48h", got)
	}
	plain, err := f.svc.SendQuote(ctx, &salespbv1.SendQuoteRequest{Name: draft().GetName()})
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	if got := plain.GetExpireTime().AsTime().Sub(plain.GetSendTime().AsTime()); got != 14*24*time.Hour {
		t.Errorf("without a ttl the offer stands %v; want 14 days", got)
	}

	// The offer's clock runs out.
	q.ExpireTime = timestamppb.New(time.Now().Add(-time.Minute))
	if _, err := f.repos.Quotes.UpdateOutputOnly(ctx, q, []string{"expire_time"}); err != nil {
		t.Fatalf("lapse the quote: %v", err)
	}
	if _, err := f.svc.QuoteExpiry().Round(ctx); err != nil {
		t.Fatalf("expiry round: %v", err)
	}
	for _, c := range []struct {
		name string
		want salespbv1.Quote_State
	}{{q.GetName(), salespbv1.Quote_STATE_EXPIRED}, {plain.GetName(), salespbv1.Quote_STATE_SENT}} {
		got, err := f.svc.GetQuote(ctx, &salespbv1.GetQuoteRequest{Name: c.name})
		if err != nil || got.GetState() != c.want {
			t.Errorf("%s is %v (%v); want %v", c.name, got.GetState(), err, c.want)
		}
	}
}

// dateRange is [y-m-from, y-m-to].
func dateRange(y, m, from, to int32) *typepbv1.DateRange {
	return &typepbv1.DateRange{StartDate: &date.Date{Year: y, Month: m, Day: from}, EndDate: &date.Date{Year: y, Month: m, Day: to}}
}
