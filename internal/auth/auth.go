// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package auth decides who may call what. Staff sign in with any standard
// OIDC issuer (verify.go): the token's claims become a Principal -- who, the
// roles they hold, the venues they work at. A policy (policy.go,
// policy_table.go) names the roles each RPC of each service allows, and a
// venue-limited principal reaches only resources under its venues
// (venue.go). The ConciergeService is the guest's and public; a guest holding
// a signed link (link.go) may read the one quote, reservation or invoice it
// names. One gRPC unary interceptor (interceptor.go) applies all of it, on
// gRPC, on the REST gateway (which calls the gRPC server) and on MCP (whose
// tools dispatch through the same chain; mcp.go lifts the HTTP Authorization
// header onto the call).
package auth

import (
	"context"
	"slices"
	"strings"
)

// Role is what a member of staff does.
type Role string

// The roles.
const (
	// Owner may do everything, at every venue they hold.
	Owner Role = "owner"
	// FrontDesk runs stays and guests: check-in, check-out, requests.
	FrontDesk Role = "front_desk"
	// Sales runs leads, quotes and events.
	Sales Role = "sales"
	// Kitchen sees sessions, menus, dietary needs and service requests.
	Kitchen Role = "kitchen"
	// Housekeeping sees service requests and the day's arrivals and
	// departures.
	Housekeeping Role = "housekeeping"
	// Accounts runs invoices and payments, and reads the rest.
	Accounts Role = "accounts"
)

// Roles are every role, owner first.
var Roles = []Role{Owner, FrontDesk, Sales, Kitchen, Housekeeping, Accounts}

// AllVenues is the venues claim value that reaches every venue.
const AllVenues = "*"

// Principal is who is calling: a member of staff from a verified token, or
// a guest holding a signed link.
type Principal struct {
	// Subject is the issuer's id for the person (sub), or "link" for a
	// guest's link.
	Subject string
	// Email is the person's email, when the token carries one.
	Email string
	// Roles are the aperio roles the person holds.
	Roles []Role
	// Venues are the venue names (venues/{venue}) the person works at, or
	// AllVenues alone for every one.
	Venues []string
	// Link is, for a guest's link, the one resource it reads.
	Link string
}

// Has reports whether p holds role r.
func (p Principal) Has(r Role) bool { return slices.Contains(p.Roles, r) }

// Unlimited reports whether p reaches every venue.
func (p Principal) Unlimited() bool { return slices.Contains(p.Venues, AllVenues) }

// Reaches reports whether p may touch resources under venue, a
// venues/{venue} name.
func (p Principal) Reaches(venue string) bool {
	return p.Unlimited() || slices.Contains(p.Venues, venue)
}

// venueName is a venues claim value as a venue name: "lakeside" and
// "venues/lakeside" are both venues/lakeside, and "*" stays itself.
func venueName(v string) string {
	v = strings.TrimSpace(v)
	if v == "" || v == AllVenues || strings.HasPrefix(v, "venues/") {
		return v
	}
	return "venues/" + v
}

// principalKey is the context key the interceptor puts the principal under.
type principalKey struct{}

// WithPrincipal is ctx carrying p.
func WithPrincipal(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, principalKey{}, p)
}

// FromContext is the principal a call was made by, and false when there is
// none: auth is off, or the method is public.
func FromContext(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(principalKey{}).(Principal)
	return p, ok
}
