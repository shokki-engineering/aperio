// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package razorpay

// webhook.go receives Razorpay's webhooks. Each is signed: X-Razorpay-
// Signature is the hex HMAC-SHA256 of the raw body under the webhook's
// secret, checked in constant time before anything is read; a bad one is
// 401. Then the event becomes finance's Outcome, settled on the payment
// whose (gateway, gateway_reference) it names:
//
//	payment_link.paid                   the link's id     succeeded
//	payment_link.expired / .cancelled   the link's id     failed
//	payment.captured                    payment, order id succeeded
//	refund.processed / refund.failed    the refund's id   succeeded / failed
//
// Settle is idempotent, so a redelivery is 200; so is a settled payment
// moved on since (nothing to do), an event aperio does not act on, and a
// payment.captured naming no payment of aperio's (a link's payment is
// settled by payment_link.paid). A link or refund aperio made (its notes
// say so) whose payment is not yet recorded is 404, which Razorpay retries;
// anything else that fails is 500, retried too.

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/shokki-engineering/aperio/internal/service/finance"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/finance/v1/financepbv1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Settler records a gateway's outcome: finance's Server.
type Settler interface {
	Settle(ctx context.Context, venueName string, o finance.Outcome) (*financepbv1.Payment, error)
}

// anyVenue reads across every venue (AIP-159): (gateway, gateway_reference)
// is unique across them, so the reference alone finds the payment.
const anyVenue = "venues/-"

// Webhook is the handler for Razorpay's webhooks.
type Webhook struct {
	secret  []byte
	settler Settler
}

// NewWebhook returns the handler checking signatures with secret and
// settling through s.
func NewWebhook(secret string, s Settler) *Webhook {
	return &Webhook{secret: []byte(secret), settler: s}
}

// entity is the part of a payment, link or refund the webhook reads.
type entity struct {
	ID      string          `json:"id"`
	OrderID string          `json:"order_id"`
	Notes   json.RawMessage `json:"notes"` // an object, or [] when empty
}

// event is a webhook's body.
type event struct {
	Event     string                             `json:"event"`
	CreatedAt int64                              `json:"created_at"`
	Payload   map[string]struct{ Entity entity } `json:"payload"`
}

// ServeHTTP checks the signature and settles what the event reports.
func (w *Webhook) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(rw, "POST only", http.StatusMethodNotAllowed)
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(rw, r.Body, 1<<20))
	if err != nil {
		http.Error(rw, "unreadable body", http.StatusBadRequest)
		return
	}
	if !w.Verify(body, r.Header.Get("X-Razorpay-Signature")) {
		http.Error(rw, "bad signature", http.StatusUnauthorized)
		return
	}
	var e event
	if err := json.Unmarshal(body, &e); err != nil {
		http.Error(rw, "not an event", http.StatusBadRequest)
		return
	}
	refs, succeeded, ours := e.refs()
	at := time.Now()
	if e.CreatedAt > 0 {
		at = time.Unix(e.CreatedAt, 0)
	}
	for _, ref := range refs {
		_, err := w.settler.Settle(r.Context(), anyVenue, finance.Outcome{Gateway: name, Reference: ref, Succeeded: succeeded, At: at})
		switch status.Code(err) {
		case codes.OK, codes.FailedPrecondition:
			rw.WriteHeader(http.StatusOK)
			return
		case codes.NotFound:
			continue
		default:
			http.Error(rw, "settling failed", http.StatusInternalServerError)
			return
		}
	}
	if len(refs) > 0 && ours {
		http.Error(rw, "payment not yet recorded", http.StatusNotFound)
		return
	}
	rw.WriteHeader(http.StatusOK)
}

// Verify reports whether signature is the body's, under the secret.
func (w *Webhook) Verify(body []byte, signature string) bool {
	got, err := hex.DecodeString(signature)
	if err != nil || len(w.secret) == 0 {
		return false
	}
	return hmac.Equal(got, mac(w.secret, body))
}

// Sign is the X-Razorpay-Signature Razorpay sends with body under secret:
// for a test's deliveries, and a fake Razorpay's.
func Sign(secret string, body []byte) string { return hex.EncodeToString(mac([]byte(secret), body)) }

// mac is body's HMAC-SHA256 under secret.
func mac(secret, body []byte) []byte {
	m := hmac.New(sha256.New, secret)
	m.Write(body)
	return m.Sum(nil)
}

// refs are the references e settles, in the order to try them, whether it
// reports success, and whether aperio made what it names.
func (e event) refs() (refs []string, succeeded, ours bool) {
	link, payment, refund := e.Payload["payment_link"].Entity, e.Payload["payment"].Entity, e.Payload["refund"].Entity
	switch e.Event {
	case "payment_link.paid":
		return []string{link.ID}, true, marked(link.Notes)
	case "payment_link.expired", "payment_link.cancelled":
		return []string{link.ID}, false, marked(link.Notes)
	case "payment.captured":
		refs = []string{payment.ID}
		if payment.OrderID != "" {
			refs = append(refs, payment.OrderID)
		}
		return refs, true, false
	case "refund.processed", "refund.failed":
		return []string{refund.ID}, e.Event == "refund.processed", marked(refund.Notes)
	}
	return nil, false, false
}

// marked reports whether notes carry aperio's mark.
func marked(notes json.RawMessage) bool {
	var m map[string]any
	return json.Unmarshal(notes, &m) == nil && m[noteKey] != nil
}
