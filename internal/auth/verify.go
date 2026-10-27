// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package auth

// verify.go turns a staff member's bearer token into a Principal. The token
// is a JWT from a standard OIDC issuer: go-oidc reads the issuer's discovery
// document once, at startup, and keeps its JWKS cached, fetching it again
// when a token names a key it does not hold -- so the issuer's key rotation
// needs nothing from aperio. Issuer, audience and expiry are checked, with
// some leeway for clocks that drift.

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/shokki-engineering/aperio/internal/config"
)

// Verifier checks bearer tokens from one OIDC issuer.
type Verifier struct {
	verifier *oidc.IDTokenVerifier
	roles    string            // the roles claim's path
	venues   string            // the venues claim's path
	roleMap  map[string]string // claim value -> role
}

// NewVerifier discovers cfg's issuer and returns a verifier of its tokens.
// It fails when the discovery document cannot be read or names another
// issuer. now is the clock; nil means time.Now.
func NewVerifier(ctx context.Context, cfg config.AuthConfig, now func() time.Time) (*Verifier, error) {
	if cfg.Audience == "" {
		return nil, errors.New("auth: auth.audience (the client ID tokens are issued to) is not set")
	}
	provider, err := oidc.NewProvider(ctx, cfg.Issuer)
	if err != nil {
		return nil, fmt.Errorf("auth: discover OIDC issuer %s: %w", cfg.Issuer, err)
	}
	if now == nil {
		now = time.Now
	}
	skew := cfg.ClockSkew()
	v := &Verifier{
		verifier: provider.Verifier(&oidc.Config{
			ClientID: cfg.Audience,
			// A token is taken until skew past its expiry.
			Now: func() time.Time { return now().Add(-skew) },
		}),
		roles:   orDefault(cfg.RolesClaim, "roles"),
		venues:  orDefault(cfg.VenuesClaim, "venues"),
		roleMap: cfg.RoleMap,
	}
	return v, nil
}

// Verify checks raw -- signature, issuer, audience, expiry -- and returns
// the principal its claims describe.
func (v *Verifier) Verify(ctx context.Context, raw string) (Principal, error) {
	token, err := v.verifier.Verify(ctx, raw)
	if err != nil {
		return Principal{}, err
	}
	var claims map[string]any
	if err := token.Claims(&claims); err != nil {
		return Principal{}, err
	}
	p := Principal{Subject: token.Subject}
	p.Email, _ = claims["email"].(string)
	for _, value := range claimValues(claims, v.roles) {
		if role, ok := v.role(value); ok && !p.Has(role) {
			p.Roles = append(p.Roles, role)
		}
	}
	for _, value := range claimValues(claims, v.venues) {
		if name := venueName(value); name != "" {
			p.Venues = append(p.Venues, name)
		}
	}
	return p, nil
}

// role is the aperio role a claim value names, through the role map first.
func (v *Verifier) role(value string) (Role, bool) {
	if mapped, ok := v.roleMap[value]; ok {
		value = mapped
	}
	for _, r := range Roles {
		if string(r) == value {
			return r, true
		}
	}
	return "", false
}

// claimValues are the strings at path in claims: a list's strings, or a
// string split on spaces and commas. A dotted path reaches into objects.
func claimValues(claims map[string]any, path string) []string {
	var at any = claims
	for _, key := range strings.Split(path, ".") {
		obj, ok := at.(map[string]any)
		if !ok {
			return nil
		}
		at = obj[key]
	}
	switch value := at.(type) {
	case string:
		return strings.FieldsFunc(value, func(r rune) bool { return r == ' ' || r == ',' })
	case []any:
		var out []string
		for _, item := range value {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

// orDefault is s, or fallback when s is empty.
func orDefault(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}
