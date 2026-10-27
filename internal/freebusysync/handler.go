// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package freebusysync

// handler.go is the endpoint freebusy's webhook POSTs to. A delivery is a
// CloudEvent signed per Standard Webhooks (runtime-go events.Sign: HMAC-
// SHA256 of "<webhook-id>.<webhook-timestamp>.<body>" under the whsec_
// secret, "v1,<base64>" in webhook-signature, two of them for a day after a
// rotation); the SDK's ParseWebhook checks it exactly so, and refuses a
// timestamp more than five minutes off. Anything but a 2xx makes freebusy
// try again -- 30 seconds, doubling, eight attempts -- so a failure to
// settle answers 500, and a delivery already settled (the webhook-id is the
// event's id, the same on every retry) answers 204 without settling it
// twice. Settling is idempotent anyway: it acts on what freebusy has now,
// not on the event, so a replay past the in-memory set changes nothing.

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"sync"

	"github.com/shokki-engineering/freebusy-go"
)

// remembered is how many settled delivery ids a Handler keeps.
const remembered = 4096

// Handler serves freebusy's webhook deliveries.
type Handler struct {
	sync *Syncer

	mu      sync.Mutex
	secrets []string
	seen    map[string]bool
	order   []string // seen, oldest first
}

// NewHandler is the endpoint settling deliveries through s, verified with
// secrets: the webhook's whsec_ secret, and during a rotation the old one.
func NewHandler(s *Syncer, secrets ...string) *Handler {
	h := &Handler{sync: s, seen: map[string]bool{}}
	for _, k := range secrets {
		h.AddSecret(k)
	}
	return h
}

// AddSecret accepts deliveries signed with secret too: one Bootstrap minted.
func (h *Handler) AddSecret(secret string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if secret = strings.TrimSpace(secret); secret != "" {
		h.secrets = append(h.secrets, secret)
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	h.mu.Lock()
	secrets := h.secrets
	h.mu.Unlock()
	if len(secrets) == 0 {
		http.Error(w, "no webhook secret is configured", http.StatusServiceUnavailable)
		return
	}
	e, err := freebusy.ParseWebhook(r, secrets...)
	switch {
	case errors.Is(err, freebusy.ErrWebhookSignature):
		http.Error(w, "the signature does not verify", http.StatusUnauthorized)
		return
	case err != nil:
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if h.settled(e.ID) {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if err := h.settle(r, e); err != nil {
		slog.ErrorContext(r.Context(), "freebusysync: delivery not settled", "id", e.ID, "type", e.Type, "subject", e.Subject, "err", err)
		http.Error(w, "not settled; send it again", http.StatusInternalServerError)
		return
	}
	h.remember(e.ID)
	w.WriteHeader(http.StatusNoContent)
}

// settle acts on e: a reservation that ended or moved, a hold that lapsed.
// Other events are not aperio's business, and are accepted as they are.
func (h *Handler) settle(r *http.Request, e *freebusy.WebhookEvent) error {
	switch e.Type {
	case freebusy.EventReservationCancelled, freebusy.EventReservationFailed, freebusy.EventReservationRescheduled:
		return h.sync.Reservation(r.Context(), e.Subject)
	case freebusy.EventHoldExpired, freebusy.EventHoldReleased:
		return h.sync.HoldLapsed(r.Context(), e.Subject)
	}
	return nil
}

// EventTypes are the events aperio's subscription asks for.
var EventTypes = []string{
	freebusy.EventReservationCancelled, freebusy.EventReservationFailed, freebusy.EventReservationRescheduled,
	freebusy.EventHoldExpired, freebusy.EventHoldReleased,
}

// settled reports whether delivery id was settled already.
func (h *Handler) settled(id string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.seen[id]
}

// remember records id as settled, forgetting the oldest past remembered.
func (h *Handler) remember(id string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.seen[id] {
		return
	}
	h.seen[id] = true
	h.order = append(h.order, id)
	if len(h.order) > remembered {
		delete(h.seen, h.order[0])
		h.order = h.order[1:]
	}
}
