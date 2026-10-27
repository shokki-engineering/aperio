// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package auth

// link_test.go checks guests' signed links.

import (
	"strings"
	"testing"
	"time"
)

// TestLinks signs and verifies a link, and refuses lapsed, forged and
// foreign ones.
func TestLinks(t *testing.T) {
	now := time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)
	clock := func() time.Time { return now }
	links := NewLinks([]byte("secret"), clock)
	const quote = "venues/lakeside/quotes/q1"
	token, err := links.SignLink(quote, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := links.VerifyLink(token); err != nil || got != quote {
		t.Fatalf("VerifyLink = %q, %v", got, err)
	}
	if _, err := NewLinks([]byte("other"), clock).VerifyLink(token); err == nil {
		t.Error("a link signed with another secret was taken")
	}
	body, mac, _ := strings.Cut(token, ".")
	if _, err := links.VerifyLink(body + "x." + mac); err == nil {
		t.Error("a tampered link was taken")
	}
	later := NewLinks([]byte("secret"), func() time.Time { return now.Add(2 * time.Hour) })
	if _, err := later.VerifyLink(token); err == nil {
		t.Error("a lapsed link was taken")
	}
	for _, resource := range []string{"contacts/c1", "venues/lakeside", "venues/lakeside/leads/l1"} {
		if _, err := links.SignLink(resource, time.Hour); err == nil {
			t.Errorf("SignLink(%s) signed", resource)
		}
	}
	if _, err := links.SignLink(quote, 0); err == nil {
		t.Error("a link with no lifetime was signed")
	}
}
