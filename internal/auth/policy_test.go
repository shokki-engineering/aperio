// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package auth

// policy_test.go checks the policy covers every RPC of every service, and
// the roadmap's exit criterion for the kitchen.

import (
	"testing"

	_ "github.com/shokki-engineering/aperio/protobuf/generated/go/catalog/v1/catalogpbv1"
	_ "github.com/shokki-engineering/aperio/protobuf/generated/go/concierge/v1/conciergepbv1"
	_ "github.com/shokki-engineering/aperio/protobuf/generated/go/document/v1/documentpbv1"
	_ "github.com/shokki-engineering/aperio/protobuf/generated/go/event/v1/eventpbv1"
	_ "github.com/shokki-engineering/aperio/protobuf/generated/go/finance/v1/financepbv1"
	_ "github.com/shokki-engineering/aperio/protobuf/generated/go/guest/v1/guestpbv1"
	_ "github.com/shokki-engineering/aperio/protobuf/generated/go/operations/v1/operationspbv1"
	_ "github.com/shokki-engineering/aperio/protobuf/generated/go/pricing/v1/pricingpbv1"
	_ "github.com/shokki-engineering/aperio/protobuf/generated/go/sales/v1/salespbv1"
	_ "github.com/shokki-engineering/aperio/protobuf/generated/go/stay/v1/staypbv1"
	_ "github.com/shokki-engineering/aperio/protobuf/generated/go/venue/v1/venuepbv1"
)

// TestEveryMethodHasAPolicy fails when an RPC has no rule, so a new one
// cannot ship unguarded, and when a rule names an RPC that is gone.
func TestEveryMethodHasAPolicy(t *testing.T) {
	methods := Methods()
	if len(methods) < 100 {
		t.Fatalf("only %d methods registered; are the services' packages imported?", len(methods))
	}
	policy := DefaultPolicy()
	for _, m := range policy.Unguarded() {
		t.Errorf("%s has no access policy: add it to policy_table.go", m)
	}
	known := map[string]bool{}
	for _, m := range methods {
		known[m] = true
	}
	for m := range policy {
		if !known[m] {
			t.Errorf("policy_table.go names %s, which no service has", m)
		}
	}
}

// TestKitchenSeesRequestsAndDietsNotInvoices is the M5 exit criterion: a
// kitchen login sees service requests and dietary needs, not invoices.
func TestKitchenSeesRequestsAndDietsNotInvoices(t *testing.T) {
	policy := DefaultPolicy()
	kitchen := Principal{Roles: []Role{Kitchen}, Venues: []string{"venues/lakeside"}}
	allowed := []string{
		"/aperio.operations.v1.OperationsService/ListServiceRequests",
		"/aperio.operations.v1.OperationsService/AcknowledgeServiceRequest",
		"/aperio.operations.v1.OperationsService/CompleteServiceRequest",
		"/aperio.event.v1.EventService/SummarizeSessions", // diet counts
		"/aperio.event.v1.EventService/GetSession",        // dietary requirements
		"/aperio.stay.v1.StayService/SummarizeDay",        // in-house guests' diets
		"/aperio.catalog.v1.CatalogService/ListMenus",
	}
	denied := []string{
		"/aperio.finance.v1.FinanceService/GetInvoice",
		"/aperio.finance.v1.FinanceService/ListInvoices",
		"/aperio.finance.v1.FinanceService/ListPayments",
		"/aperio.finance.v1.FinanceService/SummarizeReceivables",
		"/aperio.sales.v1.SalesService/ListQuotes",
		"/aperio.catalog.v1.CatalogService/UpdateMenu",
	}
	for _, m := range allowed {
		if r, ok := policy.Rule(m); !ok || !r.Allows(kitchen) {
			t.Errorf("kitchen refused %s", m)
		}
	}
	for _, m := range denied {
		if r, ok := policy.Rule(m); !ok || r.Allows(kitchen) {
			t.Errorf("kitchen allowed %s", m)
		}
	}
}

// TestOwnerMayCallEverything lets the owner through every rule.
func TestOwnerMayCallEverything(t *testing.T) {
	owner := Principal{Roles: []Role{Owner}, Venues: []string{AllVenues}}
	for m, r := range DefaultPolicy() {
		if !r.Public && !r.Allows(owner) {
			t.Errorf("owner refused %s", m)
		}
	}
}
