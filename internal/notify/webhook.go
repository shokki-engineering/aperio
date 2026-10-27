// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package notify

// webhook.go is the generic webhook: each message is POSTed as JSON, its
// body signed with HMAC-SHA256 under a shared secret, so the receiver knows
// it came from aperio and was not changed on the way. The receiver computes
// Sign(secret, body) over the raw body it read and compares it, in constant
// time (hmac.Equal), with the X-Aperio-Signature header.

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"time"
)

// SignatureHeader carries the body's signature: "sha256=" and its hex HMAC.
const SignatureHeader = "X-Aperio-Signature"

// EventHeader carries the message's event, to route by without parsing.
const EventHeader = "X-Aperio-Event"

// Webhook is the Notifier that posts each message, signed, to URL.
type Webhook struct {
	// URL receives the messages.
	URL string
	// Secret keys the signature.
	Secret []byte
	// Client sends them; http.DefaultClient when nil.
	Client *http.Client
	// Now stamps them; time.Now when nil.
	Now func() time.Time
}

// webhookBody is what a webhook receives: the message and when it was sent.
type webhookBody struct {
	Message
	SendTime time.Time `json:"send_time"`
}

// Notify posts m.
func (w *Webhook) Notify(ctx context.Context, m Message) error {
	now := time.Now
	if w.Now != nil {
		now = w.Now
	}
	body, err := json.Marshal(webhookBody{Message: m, SendTime: now().UTC()})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, w.URL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(EventHeader, m.Event)
	req.Header.Set(SignatureHeader, Sign(w.Secret, body))
	return post(w.Client, req, "webhook")
}

// Sign is the signature of body under secret: "sha256=" and the hex of its
// HMAC-SHA256.
func Sign(secret, body []byte) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

// Verify reports whether signature is body's under secret, in constant time:
// what a receiver written in Go calls.
func Verify(secret, body []byte, signature string) bool {
	return hmac.Equal([]byte(Sign(secret, body)), []byte(signature))
}
