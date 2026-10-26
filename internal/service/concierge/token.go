// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package concierge

// token.go seals a hold: what HoldStay held -- the rooms, the dates, the
// guest, the price -- signed with the server's hold secret, so ConfirmStay
// books exactly that and nothing a caller (or a model) changed on the way.
// The token is the whole state: aperio stores nothing until the stay is
// booked, and an abandoned hold lapses in freebusy on its own.

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

// held is what a hold token carries.
type held struct {
	Venue       string    `json:"v"`
	Reservation string    `json:"r"` // the id the stay is to be booked as
	Space       string    `json:"s"`
	Arrival     [3]int32  `json:"a"` // year, month, day
	Nights      int32     `json:"n"`
	Rooms       int32     `json:"q"`
	Adults      int32     `json:"ad"`
	Children    int32     `json:"ch,omitempty"`
	Guest       string    `json:"g"`
	Phone       string    `json:"p"`
	Email       string    `json:"e,omitempty"`
	Requests    string    `json:"x,omitempty"`
	Channel     int32     `json:"c,omitempty"`
	Total       int64     `json:"t"`  // units of Currency
	TotalNanos  int32     `json:"tn"` // and nanos
	Currency    string    `json:"cu"`
	Expire      time.Time `json:"exp"`
}

// errToken is any token not sealed by this server's secret.
var errToken = errors.New("concierge: the hold token is not valid")

// sealToken is h as a token: its JSON and the JSON's HMAC, URL-safe.
func sealToken(secret []byte, h held) (string, error) {
	body, err := json.Marshal(h)
	if err != nil {
		return "", err
	}
	enc := base64.RawURLEncoding
	return enc.EncodeToString(body) + "." + enc.EncodeToString(sign(secret, body)), nil
}

// openToken is the hold a token carries, when this server's secret sealed it.
func openToken(secret []byte, token string) (held, error) {
	enc := base64.RawURLEncoding
	b64, mac, ok := strings.Cut(token, ".")
	if !ok {
		return held{}, errToken
	}
	body, err1 := enc.DecodeString(b64)
	got, err2 := enc.DecodeString(mac)
	if err1 != nil || err2 != nil || !hmac.Equal(got, sign(secret, body)) {
		return held{}, errToken
	}
	var h held
	if err := json.Unmarshal(body, &h); err != nil {
		return held{}, errToken
	}
	return h, nil
}

// sign is body's HMAC-SHA256 under secret.
func sign(secret, body []byte) []byte {
	m := hmac.New(sha256.New, secret)
	m.Write(body)
	return m.Sum(nil)
}
