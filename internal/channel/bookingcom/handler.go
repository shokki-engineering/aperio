// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package bookingcom

// handler.go is the HTTP endpoint Booking.com (or a channel manager speaking
// its OTA) posts reservation messages to: HTTP basic auth with the
// configured credentials, the OTA_HotelResNotifRQ applied through
// channel.Sync, and the OTA_HotelResNotifRS answered. A message that cannot
// be read is answered with OTA Errors, as Booking.com expects, rather than an
// HTTP error; only a bad login is refused at the HTTP level.

import (
	"crypto/subtle"
	"io"
	"log/slog"
	"net/http"

	"github.com/shokki-engineering/aperio/internal/channel"
)

// maxBody is the largest message read: far more than any reservation.
const maxBody = 1 << 20

// Handler serves inbound reservation messages.
type Handler struct {
	Adapter *Adapter
	Sync    *channel.Sync
	// Username and Password are what Booking.com logs in with. With either
	// empty, every request is refused.
	Username, Password string
}

// NewHandler is a Handler applying messages through sync, logged into with
// username and password.
func NewHandler(sync *channel.Sync, username, password string) *Handler {
	return &Handler{Adapter: &Adapter{}, Sync: sync, Username: username, Password: password}
}

// ServeHTTP answers one POSTed OTA_HotelResNotifRQ.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "POST an OTA_HotelResNotifRQ", http.StatusMethodNotAllowed)
		return
	}
	if !h.authorized(r) {
		w.Header().Set("WWW-Authenticate", `Basic realm="aperio channels", charset="UTF-8"`)
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, maxBody))
	if err != nil {
		http.Error(w, "could not read the message", http.StatusBadRequest)
		return
	}
	in, parseErr := h.Adapter.ParseReservations(body)
	var results []channel.Result
	if parseErr == nil {
		for _, n := range in.Notifications {
			name, err := h.Sync.Apply(r.Context(), n)
			if err != nil {
				slog.WarnContext(r.Context(), "channel reservation not applied", "channel", h.Adapter.Name(),
					"reference", n.Reference, "action", n.Action.String(), "err", err)
			}
			results = append(results, channel.Result{Notification: n, Reservation: name, Err: err})
		}
	} else {
		slog.WarnContext(r.Context(), "channel message not read", "channel", h.Adapter.Name(), "err", parseErr)
	}
	out, err := h.Adapter.AnswerReservations(in, results, parseErr)
	if err != nil {
		http.Error(w, "could not answer", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/xml; charset=utf-8")
	_, _ = w.Write(out)
}

// authorized reports whether r logs in with the configured credentials,
// compared in constant time.
func (h *Handler) authorized(r *http.Request) bool {
	user, pass, ok := r.BasicAuth()
	if !ok || h.Username == "" || h.Password == "" {
		return false
	}
	u := subtle.ConstantTimeCompare([]byte(user), []byte(h.Username))
	p := subtle.ConstantTimeCompare([]byte(pass), []byte(h.Password))
	return u&p == 1
}
