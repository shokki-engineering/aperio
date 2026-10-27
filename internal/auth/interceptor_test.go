// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package auth

// interceptor_test.go drives the interceptor as the gRPC server would:
// tokens on the authorization metadata, requests of aperio's own types.

import (
	"context"
	"testing"
	"time"

	"github.com/shokki-engineering/aperio/internal/config"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/event/v1/eventpbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/finance/v1/financepbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/operations/v1/operationspbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/sales/v1/salespbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/stay/v1/staypbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/venue/v1/venuepbv1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// call runs method through a's interceptor with authorization (none when
// empty), answering resp; it returns the response and the status code.
func call(a *Authenticator, authorization, method string, req, resp any) (any, codes.Code) {
	ctx := context.Background()
	if authorization != "" {
		ctx = metadata.NewIncomingContext(ctx, metadata.Pairs("authorization", authorization))
	}
	out, err := a.Interceptor()(ctx, req, &grpc.UnaryServerInfo{FullMethod: method},
		func(context.Context, any) (any, error) { return resp, nil })
	return out, status.Code(err)
}

const (
	stay    = "/aperio.stay.v1.StayService/"
	ops     = "/aperio.operations.v1.OperationsService/"
	finance = "/aperio.finance.v1.FinanceService/"
)

// TestInterceptorRolesAndVenues covers sign-in, role and venue denials.
func TestInterceptorRolesAndVenues(t *testing.T) {
	iss := newIssuer(t)
	a := iss.authenticator(t)
	lakeside := []string{"lakeside"}
	kitchen := "Bearer " + iss.staff(t, lakeside, "kitchen")
	desk := "Bearer " + iss.staff(t, lakeside, "front_desk")
	expired := "Bearer " + iss.token(t, []string{"owner"}, []string{"*"}, time.Now().Add(-time.Hour), nil)
	lakesideRequests := &operationspbv1.ListServiceRequestsRequest{Parent: "venues/lakeside"}
	cases := []struct {
		name, auth, method string
		req                any
		want               codes.Code
	}{
		{"no token", "", ops + "ListServiceRequests", lakesideRequests, codes.Unauthenticated},
		{"expired", expired, ops + "ListServiceRequests", lakesideRequests, codes.Unauthenticated},
		{"wrong scheme", "Basic am9objpkb2U=", ops + "ListServiceRequests", lakesideRequests, codes.Unauthenticated},
		{"kitchen requests", kitchen, ops + "ListServiceRequests", lakesideRequests, codes.OK},
		{"kitchen diets", kitchen, "/aperio.event.v1.EventService/SummarizeSessions",
			&eventpbv1.SummarizeSessionsRequest{Venue: "venues/lakeside"}, codes.OK},
		{"kitchen invoices", kitchen, finance + "ListInvoices",
			&financepbv1.ListInvoicesRequest{Parent: "venues/lakeside"}, codes.PermissionDenied},
		{"other venue", kitchen, ops + "ListServiceRequests",
			&operationspbv1.ListServiceRequestsRequest{Parent: "venues/hilltop"}, codes.PermissionDenied},
		{"every venue", kitchen, ops + "ListServiceRequests",
			&operationspbv1.ListServiceRequestsRequest{Parent: "venues/-"}, codes.PermissionDenied},
		{"no venue", desk, stay + "GetReservation", &staypbv1.GetReservationRequest{}, codes.PermissionDenied},
		{"smuggled venue", desk, stay + "CreateReservation", &staypbv1.CreateReservationRequest{
			Parent: "venues/lakeside", Reservation: &staypbv1.Reservation{Name: "venues/hilltop/reservations/r1"},
		}, codes.PermissionDenied},
		{"referenced venue", desk, stay + "CreateReservation", &staypbv1.CreateReservationRequest{
			Parent: "venues/lakeside", Reservation: &staypbv1.Reservation{Event: "venues/hilltop/events/e1"},
		}, codes.PermissionDenied},
		{"desk books", desk, stay + "CreateReservation", &staypbv1.CreateReservationRequest{
			Parent: "venues/lakeside", Reservation: &staypbv1.Reservation{},
		}, codes.OK},
		{"unknown method", desk, stay + "Teleport", &staypbv1.GetReservationRequest{}, codes.PermissionDenied},
		{"concierge public", "", "/aperio.concierge.v1.ConciergeService/FindStay", nil, codes.OK},
		{"health open", "", "/grpc.health.v1.Health/Check", nil, codes.OK},
	}
	for _, c := range cases {
		if _, got := call(a, c.auth, c.method, c.req, &staypbv1.Reservation{}); got != c.want {
			t.Errorf("%s: %v, want %v", c.name, got, c.want)
		}
	}
}

// TestInterceptorNarrowsGlobalLists drops other venues' items from a
// global list, and refuses a single result from another venue.
func TestInterceptorNarrowsGlobalLists(t *testing.T) {
	iss := newIssuer(t)
	a := iss.authenticator(t)
	sales := "Bearer " + iss.staff(t, []string{"lakeside"}, "sales")
	leads := &salespbv1.ListLeadsResponse{Leads: []*salespbv1.Lead{
		{Name: "leads/l1", Venue: "venues/lakeside"},
		{Name: "leads/l2", Venue: "venues/hilltop"},
		{Name: "leads/l3"}, // still choosing a venue
	}}
	out, code := call(a, sales, "/aperio.sales.v1.SalesService/ListLeads", &salespbv1.ListLeadsRequest{}, leads)
	if code != codes.OK || len(out.(*salespbv1.ListLeadsResponse).GetLeads()) != 2 {
		t.Errorf("ListLeads = %v, %v; want lakeside's and the undecided lead", out, code)
	}
	hilltop := &salespbv1.Lead{Name: "leads/l2", Venue: "venues/hilltop"}
	if _, code := call(a, sales, "/aperio.sales.v1.SalesService/GetLead", &salespbv1.GetLeadRequest{Name: "leads/l2"}, hilltop); code != codes.PermissionDenied {
		t.Errorf("GetLead of another venue's lead: %v", code)
	}
	venues := &venuepbv1.ListVenuesResponse{Venues: []*venuepbv1.Venue{{Name: "venues/lakeside"}, {Name: "venues/hilltop"}}}
	out, _ = call(a, sales, "/aperio.venue.v1.VenueService/ListVenues", &venuepbv1.ListVenuesRequest{}, venues)
	if got := out.(*venuepbv1.ListVenuesResponse).GetVenues(); len(got) != 1 || got[0].GetName() != "venues/lakeside" {
		t.Errorf("ListVenues = %v", got)
	}
	if _, code := call(a, "Bearer "+iss.staff(t, []string{"lakeside"}, "owner"), "/aperio.venue.v1.VenueService/CreateVenue",
		&venuepbv1.CreateVenueRequest{}, &venuepbv1.Venue{}); code != codes.PermissionDenied {
		t.Errorf("a one-venue owner created a venue: %v", code)
	}
}

// TestInterceptorLinks lets a link read its resource through its Get, and
// nothing else.
func TestInterceptorLinks(t *testing.T) {
	iss := newIssuer(t)
	a := iss.authenticator(t)
	const invoice = "venues/lakeside/invoices/i1"
	token, err := a.Links().SignLink(invoice, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	link := "Link " + token
	cases := []struct {
		name, method string
		req          any
		want         codes.Code
	}{
		{"its Get", finance + "GetInvoice", &financepbv1.GetInvoiceRequest{Name: invoice}, codes.OK},
		{"another invoice", finance + "GetInvoice", &financepbv1.GetInvoiceRequest{Name: "venues/lakeside/invoices/i2"}, codes.PermissionDenied},
		{"a write", finance + "CancelInvoice", &financepbv1.CancelInvoiceRequest{Name: invoice}, codes.PermissionDenied},
		{"a list", finance + "ListInvoices", &financepbv1.ListInvoicesRequest{Parent: "venues/lakeside"}, codes.PermissionDenied},
	}
	for _, c := range cases {
		if _, got := call(a, link, c.method, c.req, &financepbv1.Invoice{}); got != c.want {
			t.Errorf("%s: %v, want %v", c.name, got, c.want)
		}
	}
	if _, got := call(a, "Link forged.token", finance+"GetInvoice", &financepbv1.GetInvoiceRequest{Name: invoice}, nil); got != codes.Unauthenticated {
		t.Errorf("forged link: %v", got)
	}
}

// TestAuthOff lets everything through when no issuer is configured.
func TestAuthOff(t *testing.T) {
	a, err := New(context.Background(), config.AuthConfig{})
	if err != nil {
		t.Fatal(err)
	}
	if a.Enabled() {
		t.Fatal("enabled without an issuer")
	}
	if _, got := call(a, "", finance+"ListInvoices", &financepbv1.ListInvoicesRequest{}, nil); got != codes.OK {
		t.Errorf("auth off refused a call: %v", got)
	}
}
