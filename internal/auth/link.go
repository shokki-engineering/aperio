// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package auth

// link.go signs guests' links. A link lets whoever holds it read one quote,
// reservation or invoice -- the one sent to them -- without an account: the
// token names the resource and when it lapses, and is signed with the
// server's link secret, as the concierge seals its holds
// (internal/service/concierge/token.go). It is sent as
// "Authorization: Link <token>" and taken by that resource's Get method
// alone.

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// linkable are the resources a link may grant: a quote, a reservation, an
// invoice.
var linkable = regexp.MustCompile(`^venues/[^/]+/(quotes|reservations|invoices)/[^/]+$`)

// linkDomain separates a link's signature from anything else the same
// secret might sign.
const linkDomain = "aperio.link.v1\x00"

// ErrLink is any link not signed by this server, lapsed, or malformed.
var ErrLink = errors.New("auth: the link is not valid or has expired")

// Links signs and verifies guests' links with one secret.
type Links struct {
	secret []byte
	now    func() time.Time
}

// NewLinks returns links signed with secret; now is the clock, nil for
// time.Now.
func NewLinks(secret []byte, now func() time.Time) *Links {
	if now == nil {
		now = time.Now
	}
	return &Links{secret: secret, now: now}
}

// linked is what a link token carries.
type linked struct {
	Resource string    `json:"r"`
	Expire   time.Time `json:"exp"`
}

// SignLink is a token granting read of resource -- a quote's, reservation's
// or invoice's name -- for ttl.
func (l *Links) SignLink(resource string, ttl time.Duration) (string, error) {
	token, _, err := l.signFor(resource, ttl)
	return token, err
}

// DefaultLinkTTL is how long a shared link stands when the request names no
// ttl.
const DefaultLinkTTL = 14 * 24 * time.Hour

// ShareLink is the page a guest opens to read resource for ttl
// (DefaultLinkTTL when not positive) -- base, the guest site's URL, then
// "/<resource>?link=<token>" -- and when the link lapses.
func (l *Links) ShareLink(base, resource string, ttl time.Duration) (string, time.Time, error) {
	if ttl <= 0 {
		ttl = DefaultLinkTTL
	}
	token, expire, err := l.signFor(resource, ttl)
	if err != nil {
		return "", time.Time{}, err
	}
	return strings.TrimRight(base, "/") + "/" + resource + "?link=" + url.QueryEscape(token), expire, nil
}

// signFor is SignLink's token, and when it lapses.
func (l *Links) signFor(resource string, ttl time.Duration) (string, time.Time, error) {
	if !linkable.MatchString(resource) {
		return "", time.Time{}, errors.New("auth: a link grants a quote, a reservation or an invoice, not " + resource)
	}
	if ttl <= 0 {
		return "", time.Time{}, errors.New("auth: a link needs a positive lifetime")
	}
	expire := l.now().Add(ttl).UTC()
	body, err := json.Marshal(linked{Resource: resource, Expire: expire})
	if err != nil {
		return "", time.Time{}, err
	}
	enc := base64.RawURLEncoding
	return enc.EncodeToString(body) + "." + enc.EncodeToString(l.sign(body)), expire, nil
}

// VerifyLink is the resource token grants, when this server signed it and
// it has not lapsed.
func (l *Links) VerifyLink(token string) (string, error) {
	enc := base64.RawURLEncoding
	b64, mac, ok := strings.Cut(token, ".")
	if !ok {
		return "", ErrLink
	}
	body, err1 := enc.DecodeString(b64)
	got, err2 := enc.DecodeString(mac)
	if err1 != nil || err2 != nil || !hmac.Equal(got, l.sign(body)) {
		return "", ErrLink
	}
	var g linked
	if err := json.Unmarshal(body, &g); err != nil || !linkable.MatchString(g.Resource) {
		return "", ErrLink
	}
	if !l.now().Before(g.Expire) {
		return "", ErrLink
	}
	return g.Resource, nil
}

// sign is body's HMAC-SHA256 under the secret, in the link domain.
func (l *Links) sign(body []byte) []byte {
	m := hmac.New(sha256.New, l.secret)
	m.Write([]byte(linkDomain))
	m.Write(body)
	return m.Sum(nil)
}
