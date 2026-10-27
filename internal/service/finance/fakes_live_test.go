// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package finance_test

// What the money loop's live test (loop_live_test.go) runs against: a fake
// Razorpay that makes payment links, the signed webhook it would deliver,
// and finance's Confirmer over the real stay and event services.

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/shokki-engineering/aperio/internal/database/repository/repox"
	"github.com/shokki-engineering/aperio/internal/gateway/razorpay"
	"github.com/shokki-engineering/aperio/internal/service/event"
	"github.com/shokki-engineering/aperio/internal/service/stay"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/event/v1/eventpbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/stay/v1/staypbv1"
)

// webhookSecret signs the fake Razorpay's deliveries.
const webhookSecret = "whsec-loop-test"

// fakeRazorpay makes payment links, one per reference_id, as Razorpay does.
type fakeRazorpay struct {
	mu    sync.Mutex
	links map[string]map[string]any // by reference_id
}

func newFakeRazorpay(t *testing.T) *httptest.Server {
	f := &fakeRazorpay{links: map[string]map[string]any{}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		if _, _, ok := r.BasicAuth(); !ok {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/payment_links":
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			ref, _ := body["reference_id"].(string)
			if f.links[ref] != nil {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(`{"error":{"code":"BAD_REQUEST_ERROR","description":"reference_id already exists"}}`))
				return
			}
			id := fmt.Sprintf("plink_%d%s", len(f.links), ref[:8])
			body["id"], body["short_url"] = id, "https://rzp.io/i/"+id
			f.links[ref] = body
			_ = json.NewEncoder(w).Encode(body)
		case r.Method == http.MethodGet && r.URL.Path == "/v1/payment_links":
			_ = json.NewEncoder(w).Encode(map[string]any{"payment_links": []any{f.links[r.URL.Query().Get("reference_id")]}})
		default:
			t.Errorf("fake Razorpay: unexpected %s %s", r.Method, r.URL)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

// paidEvent is the payment_link.paid webhook for link, and its signature.
func paidEvent(link, invoice string) (string, string) {
	body := fmt.Sprintf(`{"entity":"event","event":"payment_link.paid","contains":["payment_link","payment"],`+
		`"payload":{"payment_link":{"entity":{"id":%q,"status":"paid","notes":{"aperio":%q}}},`+
		`"payment":{"entity":{"id":"pay_%s","status":"captured","notes":[]}}},"created_at":1900000000}`, link, invoice, link)
	return body, razorpay.Sign(webhookSecret, []byte(body))
}

// confirmer is finance's Confirmer over the stay and event services, as the
// server's assembly has it.
type confirmer struct {
	stays  *stay.Server
	events *event.Server
}

func (c confirmer) GetReservation(ctx context.Context, r *staypbv1.GetReservationRequest) (*staypbv1.Reservation, error) {
	return c.stays.GetReservation(ctx, r)
}
func (c confirmer) ConfirmReservation(ctx context.Context, r *staypbv1.ConfirmReservationRequest) (*staypbv1.Reservation, error) {
	return c.stays.ConfirmReservation(ctx, r)
}
func (c confirmer) GetEvent(ctx context.Context, r *eventpbv1.GetEventRequest) (*eventpbv1.Event, error) {
	return c.events.GetEvent(ctx, r)
}
func (c confirmer) ConfirmEvent(ctx context.Context, r *eventpbv1.ConfirmEventRequest) (*eventpbv1.Event, error) {
	return c.events.ConfirmEvent(ctx, r)
}

// listOf is the first page of filter.
func listOf(filter string) repox.ListInput { return repox.ListInput{PageSize: 50, Filter: filter} }
