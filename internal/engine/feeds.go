// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package engine

// feeds.go makes the links to freebusy's read-only calendar feeds, which a
// calendar app subscribes to (Google Calendar: Settings > Add calendar >
// From URL). freebusy signs a feed with its [feeds] secret; the recipe here
// is freebusy's (internal/caldav/feed.go), so a link made with the same
// secret is one freebusy accepts.

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"net/url"
	"strings"
)

// FeedURL is the link to subscribe to calendar (calendars/{calendar}) at
// base, freebusy's public HTTPS address, signed with secret: 128 bits of
// HMAC-SHA256 of the calendar's name, URL-safe base64.
func FeedURL(base, calendar string, secret []byte) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(calendar))
	token := base64.RawURLEncoding.EncodeToString(mac.Sum(nil)[:16])
	id := strings.TrimPrefix(calendar, "calendars/")
	return strings.TrimRight(base, "/") + "/feeds/calendars/" + url.PathEscape(id) + ".ics?token=" + token
}
