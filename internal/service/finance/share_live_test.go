// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package finance_test

// A live test of ShareInvoice: Unimplemented with no link signer; with one,
// the guest site's page for the invoice with a token that verifies to that
// invoice alone, for 14 days unless the request says otherwise.

import (
	"context"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/shokki-engineering/aperio/internal/auth"
	"github.com/shokki-engineering/aperio/internal/database/repository/aperio/guest"
	"github.com/shokki-engineering/aperio/internal/database/repository/aperio/venue"
	"github.com/shokki-engineering/aperio/internal/service/finance"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/finance/v1/financepbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/guest/v1/guestpbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/type/v1/typepbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/venue/v1/venuepbv1"
	"github.com/shokki-engineering/aperio/test/live"
	"google.golang.org/genproto/googleapis/type/decimal"
	"google.golang.org/genproto/googleapis/type/postaladdress"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/durationpb"
)

func TestLiveShareInvoice(t *testing.T) {
	ctx, conn := context.Background(), live.Conn(t)
	venues := venue.New(conn)
	svc := finance.New(live.FinanceRepositories(conn), venues.Venues, nil, nil)
	run := strconv.FormatInt(time.Now().UnixNano(), 10)
	v, err := venues.Venues.Create(ctx, &venuepbv1.Venue{DisplayName: "Share Resort " + run, Kind: venuepbv1.VenueKind_VENUE_KIND_STAYS,
		TimeZone: "Asia/Kolkata", CurrencyCode: "INR", Address: &postaladdress.PostalAddress{RegionCode: "IN"},
		Gst: &typepbv1.Gst{Gstin: "29ABCDE1234F1Z5", LegalDisplayName: "Test", GstStateCode: "29"}})
	if err != nil {
		t.Fatalf("create venue: %v", err)
	}
	c, err := guest.New(conn).Contacts.Create(ctx, &guestpbv1.Contact{Kind: guestpbv1.ContactKind_CONTACT_KIND_PERSON, DisplayName: "John Doe"})
	if err != nil {
		t.Fatalf("create contact: %v", err)
	}
	inv, err := svc.CreateInvoice(ctx, &financepbv1.CreateInvoiceRequest{Parent: v.GetName(), Invoice: &financepbv1.Invoice{
		Kind: financepbv1.InvoiceKind_INVOICE_KIND_PROFORMA, SupplyStateCode: "29", Contact: c.GetName(),
		Lines: []*financepbv1.InvoiceLine{{Description: "Garden room, 1 night", Quantity: 1, UnitPrice: inr(5000, 0),
			Tax: &typepbv1.Tax{HsnSacCode: "996311", GstRate: &decimal.Decimal{Value: "12"}}}},
	}})
	if err != nil {
		t.Fatalf("create invoice: %v", err)
	}

	req := &financepbv1.ShareInvoiceRequest{Name: inv.GetName()}
	if _, err := svc.ShareInvoice(ctx, req); status.Code(err) != codes.Unimplemented {
		t.Errorf("share with no signer = %v; want Unimplemented", err)
	}
	links := auth.NewLinks([]byte(strings.Repeat("s", 32)), nil)
	svc.ShareLinksWith(links, "https://guest.example/")
	got, err := svc.ShareInvoice(ctx, req)
	if err != nil {
		t.Fatalf("share: %v", err)
	}
	page, token, ok := strings.Cut(got.GetUrl(), "?link=")
	if !ok || page != "https://guest.example/"+inv.GetName() {
		t.Fatalf("url %q; want the guest site's page for %s with ?link=", got.GetUrl(), inv.GetName())
	}
	if granted, err := links.VerifyLink(token); err != nil || granted != inv.GetName() {
		t.Errorf("the token grants %q, %v; want %s", granted, err, inv.GetName())
	}
	if left := time.Until(got.GetExpireTime().AsTime()); left < 14*24*time.Hour-time.Minute || left > 14*24*time.Hour {
		t.Errorf("the link lapses in %v; want 14 days", left)
	}
	req.Ttl = durationpb.New(time.Hour)
	if short, err := svc.ShareInvoice(ctx, req); err != nil || time.Until(short.GetExpireTime().AsTime()) > time.Hour {
		t.Errorf("a one-hour link = %v, %v; want it to lapse within the hour", short.GetExpireTime(), err)
	}
	if _, err := svc.ShareInvoice(ctx, &financepbv1.ShareInvoiceRequest{Name: v.GetName() + "/invoices/missing"}); status.Code(err) != codes.NotFound {
		t.Errorf("sharing a missing invoice = %v; want NotFound", err)
	}
}
