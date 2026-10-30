// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package auth

// leads.go holds a venue-limited principal to the venues of the leads its
// requests name. A lead's name is leads/{lead}, under no venue, so venue.go
// finds no venue in UpdateLead, CreateActivity, GetActivity and the like;
// the policy lets them through as Global, and only a response naming the
// venue would be narrowed -- after the write was made. So before the call,
// every leads/{lead} the request refers to (its name, parent, or any
// resource reference, e.g. a quote's lead, an activity's parent) is looked
// up, and the venue it is at is judged like any venue the request names. A
// lead at no venue yet (the guest is still choosing) is anyone's. Contacts
// are business-wide by design, and are not looked up (venue.go).

import (
	"context"
	"strings"

	"github.com/shokki-engineering/aperio/protobuf/generated/go/sales/v1/salespbv1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

// LeadReader reads a lead, for its venue: the sales service's GetLead.
type LeadReader interface {
	GetLead(context.Context, *salespbv1.GetLeadRequest) (*salespbv1.Lead, error)
}

// ResolveLeadsWith looks up the venue of every lead a request names through
// leads, and holds a venue-limited principal to it. Until it is called, a
// request naming a lead is judged by the venues it names itself.
func (a *Authenticator) ResolveLeadsWith(leads LeadReader) { a.leads = leads }

// leadVenues are the venues of the leads m refers to. A lead that does not
// exist adds none: the call itself answers NotFound. Any other failure to
// read one refuses the call, as its venue cannot be checked.
func (a *Authenticator) leadVenues(ctx context.Context, m proto.Message) ([]string, error) {
	if a.leads == nil || m == nil {
		return nil, nil
	}
	seen := map[string]bool{}
	walk(m.ProtoReflect(), depth, func(s string) {
		if rest, ok := strings.CutPrefix(s, "leads/"); ok && rest != "" {
			id, _, _ := strings.Cut(rest, "/")
			seen["leads/"+id] = true
		}
	})
	var out []string
	for name := range seen {
		l, err := a.leads.GetLead(ctx, &salespbv1.GetLeadRequest{Name: name})
		switch {
		case status.Code(err) == codes.NotFound:
			continue
		case err != nil:
			return nil, status.Errorf(codes.Unavailable, "auth: the venue of %s could not be checked: %v", name, err)
		}
		if l.GetVenue() != "" {
			out = append(out, l.GetVenue())
		}
	}
	return out, nil
}
