// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package auth

// verify_test.go checks token verification against the test issuer: a
// valid token's claims, and the tokens refused.

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"
)

// TestVerifyValid maps a valid token's claims onto the principal.
func TestVerifyValid(t *testing.T) {
	iss := newIssuer(t)
	v, err := NewVerifier(context.Background(), iss.config(), nil)
	if err != nil {
		t.Fatal(err)
	}
	p, err := v.Verify(context.Background(), iss.staff(t, []string{"lakeside", "venues/hilltop"}, "kitchen", "nobody"))
	if err != nil {
		t.Fatal(err)
	}
	if p.Subject != "john-doe" || p.Email != "john.doe@example.com" {
		t.Errorf("principal = %+v", p)
	}
	if !slices.Equal(p.Roles, []Role{Kitchen}) {
		t.Errorf("roles = %v, want [kitchen] (unknown values dropped)", p.Roles)
	}
	if !slices.Equal(p.Venues, []string{"venues/lakeside", "venues/hilltop"}) {
		t.Errorf("venues = %v", p.Venues)
	}
}

// TestVerifyRefuses refuses expired, wrong-audience, wrong-issuer and
// unsigned tokens, and takes one just past expiry within the skew.
func TestVerifyRefuses(t *testing.T) {
	iss := newIssuer(t)
	v, err := NewVerifier(context.Background(), iss.config(), nil)
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string]string{
		"expired":        iss.token(t, []string{"owner"}, nil, time.Now().Add(-2*time.Minute), nil),
		"wrong audience": iss.token(t, []string{"owner"}, nil, time.Now().Add(time.Hour), map[string]any{"aud": "someone-else"}),
		"wrong issuer":   iss.token(t, []string{"owner"}, nil, time.Now().Add(time.Hour), map[string]any{"iss": "https://elsewhere.example"}),
		"garbage":        "not.a.token",
	}
	for name, raw := range cases {
		if _, err := v.Verify(context.Background(), raw); err == nil {
			t.Errorf("%s: verified, want refused", name)
		}
	}
	skewed := iss.token(t, []string{"owner"}, nil, time.Now().Add(-30*time.Second), nil)
	if _, err := v.Verify(context.Background(), skewed); err != nil {
		t.Errorf("30 s past expiry, within the 60 s skew: %v", err)
	}
}

// TestClaimPaths reads roles from a nested claim through a role map, and
// a space-separated venues string.
func TestClaimPaths(t *testing.T) {
	iss := newIssuer(t)
	cfg := iss.config()
	cfg.RolesClaim, cfg.VenuesClaim = "realm_access.roles", "aperio_venues"
	cfg.RoleMap = map[string]string{"aperio-accounts": "accounts"}
	v, err := NewVerifier(context.Background(), cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	raw := iss.token(t, nil, nil, time.Now().Add(time.Hour), map[string]any{
		"realm_access":  map[string]any{"roles": []string{"aperio-accounts", "offline_access"}},
		"aperio_venues": "*",
	})
	p, err := v.Verify(context.Background(), raw)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(p.Roles, []Role{Accounts}) || !p.Unlimited() {
		t.Errorf("principal = %+v", p)
	}
}

// TestVerifierNeedsAudience refuses a configuration without an audience,
// and an issuer that is not there.
func TestVerifierNeedsAudience(t *testing.T) {
	iss := newIssuer(t)
	cfg := iss.config()
	cfg.Audience = ""
	if _, err := NewVerifier(context.Background(), cfg, nil); err == nil || !strings.Contains(err.Error(), "audience") {
		t.Errorf("no audience: %v", err)
	}
	cfg = iss.config()
	cfg.Issuer = iss.URL + "/missing"
	if _, err := NewVerifier(context.Background(), cfg, nil); err == nil {
		t.Error("an issuer with no discovery document was taken")
	}
}
