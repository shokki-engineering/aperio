// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package config

// auth.go configures staff sign-in and guests' signed links
// (internal/auth): the OIDC issuer whose tokens aperio accepts -- any
// standard one, Google Workspace, Keycloak or Clerk, found from its
// discovery document -- the claims that carry a person's roles and venues,
// and the secret guests' links are signed with.

import "time"

// AuthConfig is the [auth] block.
type AuthConfig struct {
	// Issuer is the OIDC issuer URL, e.g. https://accounts.google.com or
	// https://keycloak.example/realms/aperio; its
	// /.well-known/openid-configuration names the keys tokens are signed
	// with. Empty, auth is off: every call is let through, and the server
	// says so loudly at startup. Never leave it empty on a network.
	Issuer string `koanf:"issuer"`
	// Audience is the client ID tokens must be issued to (their aud).
	Audience string `koanf:"audience"`
	// RolesClaim is the claim holding a person's roles, a list or a
	// space-separated string; a dotted path reaches into an object
	// (Keycloak's realm_access.roles). Empty means "roles".
	RolesClaim string `koanf:"roles_claim"`
	// VenuesClaim is the claim holding the venues a person works at, as
	// venue ids or venues/{venue} names, or "*" for every venue. Empty
	// means "venues". A person whose token has none reaches no venue.
	VenuesClaim string `koanf:"venues_claim"`
	// RoleMap maps a claim value to an aperio role (owner, front_desk,
	// sales, kitchen, housekeeping, accounts), for issuers whose groups
	// are named otherwise: {"aperio-kitchen" = "kitchen"}. A value that
	// is neither mapped nor a role is ignored.
	RoleMap map[string]string `koanf:"role_map"`
	// ClockSkewSeconds is how far past its expiry a token is still taken,
	// for clocks that drift. 0 means 60 seconds.
	ClockSkewSeconds int `koanf:"clock_skew_seconds"`
	// LinkSecret signs guests' links to a quote, reservation or invoice:
	// 32 or more random bytes (openssl rand -hex 32), the same on every
	// replica. Empty, each process makes its own and a link outlives
	// neither a restart nor a move to another replica. A secret: set it
	// through APERIO_AUTH__LINK_SECRET, not a file.
	LinkSecret string `koanf:"link_secret"`
}

// Enabled reports whether an issuer is configured, and so whether calls
// are checked at all.
func (a AuthConfig) Enabled() bool { return a.Issuer != "" }

// ClockSkew is ClockSkewSeconds as a duration, 60 seconds when unset.
func (a AuthConfig) ClockSkew() time.Duration {
	if a.ClockSkewSeconds <= 0 {
		return time.Minute
	}
	return time.Duration(a.ClockSkewSeconds) * time.Second
}
