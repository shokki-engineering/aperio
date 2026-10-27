// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package razorpay_test

// Tests of the webhook handler with a fake Settler: signatures good and
// bad, each event mapped to its outcome, and the answers Razorpay retries
// on and those it does not.

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/shokki-engineering/aperio/internal/gateway/razorpay"
	"github.com/shokki-engineering/aperio/internal/service/finance"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/finance/v1/financepbv1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// settler records outcomes, and knows only the references in known.
type settler struct {
	known map[string]codes.Code
	got   []finance.Outcome
	venue string
}

func (s *settler) Settle(_ context.Context, venue string, o finance.Outcome) (*financepbv1.Payment, error) {
	s.got, s.venue = append(s.got, o), venue
	c, ok := s.known[o.Reference]
	if !ok {
		return nil, status.Error(codes.NotFound, "no such payment")
	}
	return &financepbv1.Payment{}, status.Error(c, "")
}

const secret = "whsec-test"

func deliver(h http.Handler, body, sig string) int {
	r := httptest.NewRequest(http.MethodPost, "/webhooks/razorpay", bytes.NewBufferString(body))
	r.Header.Set("X-Razorpay-Signature", sig)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w.Code
}

func TestWebhook(t *testing.T) {
	s := &settler{known: map[string]codes.Code{"plink_1": codes.OK, "order_7": codes.OK, "rfnd_1": codes.OK, "plink_old": codes.FailedPrecondition}}
	h := razorpay.NewWebhook(secret, s)
	paid := `{"event":"payment_link.paid","created_at":1900000000,"payload":{"payment_link":{"entity":{"id":"plink_1","notes":{"aperio":"venues/v/invoices/i"}}},"payment":{"entity":{"id":"pay_1","notes":[]}}}}`

	if code := deliver(h, paid, razorpay.Sign("another secret", []byte(paid))); code != http.StatusUnauthorized {
		t.Errorf("wrong secret: %d; want 401", code)
	}
	if code := deliver(h, paid, "not hex"); code != http.StatusUnauthorized {
		t.Errorf("garbled signature: %d; want 401", code)
	}
	if len(s.got) != 0 {
		t.Fatalf("an unsigned event was settled: %v", s.got)
	}

	for _, c := range []struct {
		name, body string
		code       int
		ref        string
		succeeded  bool
	}{
		{"link paid", paid, 200, "plink_1", true},
		{"link expired, settled since", `{"event":"payment_link.expired","payload":{"payment_link":{"entity":{"id":"plink_old","notes":{"aperio":"x"}}}}}`, 200, "plink_old", false},
		{"captured, by its order", `{"event":"payment.captured","payload":{"payment":{"entity":{"id":"pay_7","order_id":"order_7","notes":[]}}}}`, 200, "order_7", true},
		{"captured on a link: not ours", `{"event":"payment.captured","payload":{"payment":{"entity":{"id":"pay_1","notes":[]}}}}`, 200, "pay_1", true},
		{"refund processed", `{"event":"refund.processed","payload":{"refund":{"entity":{"id":"rfnd_1","notes":{"aperio":"pay_1"}}}}}`, 200, "rfnd_1", true},
		{"refund failed, not yet recorded", `{"event":"refund.failed","payload":{"refund":{"entity":{"id":"rfnd_2","notes":{"aperio":"pay_1"}}}}}`, 404, "rfnd_2", false},
		{"another app's link", `{"event":"payment_link.paid","payload":{"payment_link":{"entity":{"id":"plink_x","notes":[]}}}}`, 200, "plink_x", true},
	} {
		s.got = nil
		if code := deliver(h, c.body, razorpay.Sign(secret, []byte(c.body))); code != c.code {
			t.Errorf("%s: %d; want %d", c.name, code, c.code)
		}
		last := s.got[len(s.got)-1]
		if last.Gateway != "razorpay" || last.Reference != c.ref || last.Succeeded != c.succeeded || s.venue != "venues/-" {
			t.Errorf("%s: settled %+v at %s; want %s, succeeded %v", c.name, last, s.venue, c.ref, c.succeeded)
		}
	}

	s.got = nil
	other := `{"event":"order.paid","payload":{}}`
	if code := deliver(h, other, razorpay.Sign(secret, []byte(other))); code != 200 || len(s.got) != 0 {
		t.Errorf("an event not acted on: %d, settled %v; want 200, nothing", code, s.got)
	}
}
