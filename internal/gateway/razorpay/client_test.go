// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package razorpay_test

// Tests of the API client against an httptest fake of Razorpay: the link's
// request shape and basic auth, a retried reference read back, and a refund
// of a link's payment made against the payment's own id.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/shokki-engineering/aperio/internal/gateway/razorpay"
	"github.com/shokki-engineering/aperio/internal/service/finance"
	"google.golang.org/genproto/googleapis/type/money"
)

// fake is a Razorpay keeping the links it made, by reference_id.
type fake struct {
	t     *testing.T
	links map[string]map[string]any
	last  map[string]any // the last body posted
}

func (f *fake) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if id, secret, ok := r.BasicAuth(); !ok || id != "rzp_test_key" || secret != "key-secret" {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	var body map[string]any
	if r.Method == http.MethodPost {
		_ = json.NewDecoder(r.Body).Decode(&body)
		f.last = body
	}
	switch {
	case r.Method == http.MethodPost && r.URL.Path == "/v1/payment_links":
		ref, _ := body["reference_id"].(string)
		if f.links[ref] != nil {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":{"code":"BAD_REQUEST_ERROR","description":"reference_id already exists"}}`))
			return
		}
		l := map[string]any{"id": "plink_" + ref[:4], "short_url": "https://rzp.io/i/" + ref[:4],
			"amount": body["amount"], "currency": body["currency"], "expire_by": body["expire_by"]}
		f.links[ref] = l
		_ = json.NewEncoder(w).Encode(l)
	case r.Method == http.MethodGet && r.URL.Path == "/v1/payment_links":
		l := f.links[r.URL.Query().Get("reference_id")]
		_ = json.NewEncoder(w).Encode(map[string]any{"payment_links": []any{l}})
	case r.Method == http.MethodGet && r.URL.Path == "/v1/payment_links/plink_abcd":
		_, _ = w.Write([]byte(`{"id":"plink_abcd","payments":[{"payment_id":"pay_9","status":"captured"}]}`))
	case r.Method == http.MethodPost && r.URL.Path == "/v1/payments/pay_9/refund":
		_, _ = w.Write([]byte(`{"id":"rfnd_1","payment_id":"pay_9"}`))
	default:
		f.t.Errorf("unexpected %s %s", r.Method, r.URL)
		w.WriteHeader(http.StatusNotFound)
	}
}

func TestLinkAndRefund(t *testing.T) {
	f := &fake{t: t, links: map[string]map[string]any{}}
	srv := httptest.NewServer(f)
	defer srv.Close()
	c := razorpay.New(razorpay.Options{BaseURL: srv.URL + "/v1", KeyID: "rzp_test_key", KeySecret: "key-secret", Timeout: time.Second})
	expire := time.Unix(1_900_000_000, 0)
	req := finance.LinkRequest{Reference: "abcd-1234", Invoice: "venues/v/invoices/i", Description: "Payment for SBR/PF2627/0001",
		Amount: &money.Money{CurrencyCode: "INR", Units: 30000, Nanos: 500_000_000}, Name: "John Doe", Phone: "+15555550100", Expire: expire}

	l, err := c.Link(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if l.ID != "plink_abcd" || l.URL != "https://rzp.io/i/abcd" || l.Amount.GetUnits() != 30000 || l.Amount.GetNanos() != 500_000_000 || !l.Expire.Equal(expire) {
		t.Errorf("link = %+v", l)
	}
	b := f.last
	if b["amount"] != 3000050.0 || b["currency"] != "INR" || b["reference_id"] != "abcd-1234" || b["expire_by"] != 1.9e9 ||
		b["accept_partial"] != false || b["reminder_enable"] != false {
		t.Errorf("body = %v", b)
	}
	if n := b["notify"].(map[string]any); n["sms"] != false || n["email"] != false {
		t.Errorf("notify = %v; want both off", n)
	}
	if cu := b["customer"].(map[string]any); cu["name"] != "John Doe" || cu["contact"] != "+15555550100" {
		t.Errorf("customer = %v", cu)
	}
	if n := b["notes"].(map[string]any); n["aperio"] != "venues/v/invoices/i" {
		t.Errorf("notes = %v", n)
	}

	again, err := c.Link(context.Background(), req) // a retry: the same link back
	if err != nil || again.ID != l.ID {
		t.Errorf("retry = %+v, %v; want %s", again, err, l.ID)
	}

	ref, err := c.Refund(context.Background(), "plink_abcd", &money.Money{CurrencyCode: "INR", Units: 100})
	if err != nil || ref != "rfnd_1" {
		t.Fatalf("refund = %q, %v", ref, err)
	}
	if f.last["amount"] != 10000.0 || f.last["notes"].(map[string]any)["aperio"] != "plink_abcd" {
		t.Errorf("refund body = %v", f.last)
	}

	bad := razorpay.New(razorpay.Options{BaseURL: srv.URL + "/v1", KeyID: "rzp_test_key", KeySecret: "wrong"})
	if _, err := bad.Link(context.Background(), req); err == nil {
		t.Error("a wrong secret made a link")
	}
}
