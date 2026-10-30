// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package auth

// leads_test.go holds a venue-limited salesperson to the venues of the
// leads its requests name, and refills a List page that narrowing emptied.

import (
	"context"
	"testing"

	"github.com/shokki-engineering/aperio/protobuf/generated/go/sales/v1/salespbv1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

// leadBook is a LeadReader over leads by name.
type leadBook map[string]*salespbv1.Lead

func (b leadBook) GetLead(_ context.Context, req *salespbv1.GetLeadRequest) (*salespbv1.Lead, error) {
	if l, ok := b[req.GetName()]; ok {
		return l, nil
	}
	return nil, status.Error(codes.NotFound, "no such lead")
}

const salesSvc = "/aperio.sales.v1.SalesService/"

func TestInterceptorLeadVenues(t *testing.T) {
	iss := newIssuer(t)
	a := iss.authenticator(t)
	john := "Bearer " + iss.staff(t, []string{"lakeside"}, "sales")
	update := &salespbv1.UpdateLeadRequest{Lead: &salespbv1.Lead{Name: "leads/hill", DisplayName: "John Doe"}}
	if _, got := call(a, john, salesSvc+"UpdateLead", update, &salespbv1.Lead{}); got != codes.OK {
		t.Fatalf("without a resolver UpdateLead is judged by its own venues: %v", got)
	}
	a.ResolveLeadsWith(leadBook{
		"leads/hill": {Name: "leads/hill", Venue: "venues/hilltop"},
		"leads/lake": {Name: "leads/lake", Venue: "venues/lakeside"},
		"leads/open": {Name: "leads/open"},
	})
	cases := []struct {
		name, method string
		req          proto.Message
		want         codes.Code
	}{
		{"update another venue's lead", "UpdateLead", update, codes.PermissionDenied},
		{"update own lead", "UpdateLead", &salespbv1.UpdateLeadRequest{Lead: &salespbv1.Lead{Name: "leads/lake"}}, codes.OK},
		{"update undecided lead", "UpdateLead", &salespbv1.UpdateLeadRequest{Lead: &salespbv1.Lead{Name: "leads/open"}}, codes.OK},
		{"activity on another venue's lead", "CreateActivity", &salespbv1.CreateActivityRequest{Parent: "leads/hill"}, codes.PermissionDenied},
		{"activity of another venue's lead", "GetActivity", &salespbv1.GetActivityRequest{Name: "leads/hill/activities/a1"}, codes.PermissionDenied},
		{"activities of own lead", "ListActivities", &salespbv1.ListActivitiesRequest{Parent: "leads/lake"}, codes.OK},
		{"missing lead", "UpdateLead", &salespbv1.UpdateLeadRequest{Lead: &salespbv1.Lead{Name: "leads/none"}}, codes.OK},
	}
	for _, c := range cases {
		if _, got := call(a, john, salesSvc+c.method, c.req, &salespbv1.Lead{}); got != c.want {
			t.Errorf("%s: %v, want %v", c.name, got, c.want)
		}
	}
	owner := "Bearer " + iss.staff(t, []string{"*"}, "owner")
	if _, got := call(a, owner, salesSvc+"UpdateLead", update, &salespbv1.Lead{}); got != codes.OK {
		t.Errorf("an owner of every venue was refused: %v", got)
	}
}

func TestInterceptorRefillsNarrowedPage(t *testing.T) {
	iss := newIssuer(t)
	a := iss.authenticator(t)
	ctx := metadata.NewIncomingContext(context.Background(),
		metadata.Pairs("authorization", "Bearer "+iss.staff(t, []string{"lakeside"}, "sales")))
	pages := map[string]*salespbv1.ListLeadsResponse{
		"":   {Leads: []*salespbv1.Lead{{Name: "leads/h1", Venue: "venues/hilltop"}}, NextPageToken: "p2"},
		"p2": {Leads: []*salespbv1.Lead{{Name: "leads/h2", Venue: "venues/hilltop"}, {Name: "leads/l1", Venue: "venues/lakeside"}}, NextPageToken: "p3"},
		"p3": {Leads: []*salespbv1.Lead{{Name: "leads/h3", Venue: "venues/hilltop"}}},
	}
	handler := func(_ context.Context, req any) (any, error) {
		return proto.Clone(pages[req.(*salespbv1.ListLeadsRequest).GetPageToken()]), nil
	}
	list := func(token string) *salespbv1.ListLeadsResponse {
		out, err := a.Interceptor()(ctx, &salespbv1.ListLeadsRequest{PageToken: token},
			&grpc.UnaryServerInfo{FullMethod: salesSvc + "ListLeads"}, handler)
		if err != nil {
			t.Fatalf("ListLeads %q: %v", token, err)
		}
		return out.(*salespbv1.ListLeadsResponse)
	}
	first := list("")
	if len(first.GetLeads()) != 1 || first.GetLeads()[0].GetName() != "leads/l1" || first.GetNextPageToken() != "p3" {
		t.Errorf("first page = %v; want lakeside's lead, refilled from p2, with p3's token", first)
	}
	last := list("p3")
	if len(last.GetLeads()) != 0 || last.GetNextPageToken() != "" {
		t.Errorf("last page = %v; want empty, and the end", last)
	}
}
