// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package auth

// policy.go is how a call is judged against the policy table
// (policy_table.go): which roles a method allows, whether it is public,
// whether it reaches beyond one venue, whether a guest's link reads it. The
// owner may call every method in the table. A method missing from it is
// refused to everyone -- and Unguarded lists such methods, so a test fails
// before one ships.

import (
	"slices"
	"strings"

	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
)

// Rule is who may call one method.
type Rule struct {
	// Roles may call it, besides the owner, who may call anything.
	Roles []Role
	// Public calls need no token: the guest-facing concierge.
	Public bool
	// Global methods work on resources under no venue -- contacts, leads,
	// the venues themselves -- so a request naming no venue is let through;
	// for a venue-limited principal, a response is narrowed to its venues
	// (venue.go).
	Global bool
	// Unlimited methods are only for a principal reaching every venue:
	// creating a venue.
	Unlimited bool
	// Link methods are a resource's Get, which a guest's signed link for
	// that resource may call.
	Link bool
}

// Allows reports whether p's roles let it call a method under r.
func (r Rule) Allows(p Principal) bool {
	if p.Has(Owner) {
		return true
	}
	for _, role := range r.Roles {
		if p.Has(role) {
			return true
		}
	}
	return false
}

// Policy is the rule for each method, by full method name
// ("/aperio.stay.v1.StayService/GetReservation").
type Policy map[string]Rule

// DefaultPolicy is aperio's policy: policyTable, by full method name.
func DefaultPolicy() Policy {
	p := Policy{}
	for service, methods := range policyTable {
		for method, rule := range methods {
			p["/"+service+"/"+method] = rule
		}
	}
	return p
}

// Rule is the rule for fullMethod, and false when the policy has none.
func (p Policy) Rule(fullMethod string) (Rule, bool) {
	r, ok := p[fullMethod]
	return r, ok
}

// Guarded reports whether fullMethod is aperio's, and so judged by the
// policy. Others -- gRPC health -- are let through.
func Guarded(fullMethod string) bool {
	return strings.HasPrefix(fullMethod, "/aperio.")
}

// Methods are the full names of every method of every aperio service
// registered with the protobuf runtime, sorted: those the binary links in.
func Methods() []string {
	var out []string
	protoregistry.GlobalFiles.RangeFiles(func(f protoreflect.FileDescriptor) bool {
		if !strings.HasPrefix(string(f.Package()), "aperio.") {
			return true
		}
		services := f.Services()
		for i := range services.Len() {
			s := services.Get(i)
			methods := s.Methods()
			for j := range methods.Len() {
				out = append(out, "/"+string(s.FullName())+"/"+string(methods.Get(j).Name()))
			}
		}
		return true
	})
	slices.Sort(out)
	return out
}

// Unguarded are the registered methods p has no rule for.
func (p Policy) Unguarded() []string {
	var out []string
	for _, m := range Methods() {
		if _, ok := p[m]; !ok {
			out = append(out, m)
		}
	}
	return out
}
