// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package sales_test

// Live tests of a retried AcceptQuote and of the invoice an accept raises:
// the Doe wedding's first accept fails, as its rooms are another lead's, and
// calls its event off; the retry starts over under the next name instead of
// carrying on with the cancelled event, holds the hall once, and records
// the hold on the event; the accept then raises one DRAFT tax invoice from
// the quote's lines. Run as sales_live_test.go says.

import (
	"context"
	"testing"

	"github.com/shokki-engineering/aperio/internal/service/finance"
	"github.com/shokki-engineering/aperio/internal/service/sales"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/event/v1/eventpbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/finance/v1/financepbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/sales/v1/salespbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/type/v1/typepbv1"
	"github.com/shokki-engineering/aperio/test/live"
	"google.golang.org/genproto/googleapis/type/decimal"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestLiveAcceptRetryAfterCancelledEvent(t *testing.T) {
	f, ctx := holdSetup(t), context.Background()
	invoices := finance.New(live.FinanceRepositories(live.Conn(t)), f.venues, nil, nil)
	f.svc.InvoiceWith(invoices)

	// Another lead holds both rooms for the wedding night.
	other, err := f.svc.CreateLead(ctx, &salespbv1.CreateLeadRequest{Lead: &salespbv1.Lead{
		DisplayName: "Doe family stay", Contact: f.contact, Venue: f.venue, Kind: salespbv1.LeadKind_LEAD_KIND_STAY,
		Dates: []*typepbv1.DateRange{dateRange(2027, 12, 18, 19)}, Rooms: 2,
	}})
	if err != nil || other.GetHold() == "" {
		t.Fatalf("create the other lead: hold %q, %v", other.GetHold(), err)
	}

	gst := &typepbv1.Tax{HsnSacCode: "996311", GstRate: &decimal.Decimal{Value: "18"}}
	q, err := f.svc.CreateQuote(ctx, &salespbv1.CreateQuoteRequest{Parent: f.venue, Quote: &salespbv1.Quote{
		DisplayName: "Doe wedding", Contact: f.contact, EventType: typepbv1.EventType_EVENT_TYPE_WEDDING, Guests: 4,
		Dates: dateRange(2027, 12, 18, 18),
		Lines: []*salespbv1.QuoteLine{
			{Kind: salespbv1.LineKind_LINE_KIND_SPACE, Item: f.hall, Quantity: 1, PriceOverride: true, UnitPrice: inr(150000), Tax: gst},
			{Kind: salespbv1.LineKind_LINE_KIND_ROOM, Item: f.rooms, Quantity: 2, PriceOverride: true, UnitPrice: inr(6500)},
			{Kind: salespbv1.LineKind_LINE_KIND_DISCOUNT, Description: "Early booking", Quantity: 1, PriceOverride: true, UnitPrice: inr(-10000), Tax: gst},
		},
	}})
	if err != nil {
		t.Fatalf("create quote: %v", err)
	}
	if q, err = f.svc.SendQuote(ctx, &salespbv1.SendQuoteRequest{Name: q.GetName()}); err != nil {
		t.Fatalf("send quote: %v", err)
	}

	// The first accept fails on the rooms, and calls its event off.
	if _, err := f.svc.AcceptQuote(ctx, &salespbv1.AcceptQuoteRequest{Name: q.GetName()}); err == nil {
		t.Fatal("the wedding was accepted with its rooms another lead's")
	}
	first := f.venue + "/events/" + sales.InvoiceID(q.GetName())
	if e, err := f.events.GetEvent(ctx, &eventpbv1.GetEventRequest{Name: first}); err != nil || e.GetState() != eventpbv1.Event_STATE_CANCELLED {
		t.Fatalf("the failed attempt's event %s: %v, %v; want CANCELLED", first, e.GetState(), err)
	}
	if n := f.fb.Active(); n != 1 {
		t.Errorf("%d claims after the failed accept; want the other lead's hold alone", n)
	}

	// The rooms come free; the retry starts over, not with the cancelled event.
	if _, err := f.svc.UpdateLead(ctx, &salespbv1.UpdateLeadRequest{Lead: &salespbv1.Lead{Name: other.GetName(), Stage: salespbv1.Stage_STAGE_LOST}}); err != nil {
		t.Fatalf("lose the other lead: %v", err)
	}
	accepted, err := f.svc.AcceptQuote(ctx, &salespbv1.AcceptQuoteRequest{Name: q.GetName()})
	if err != nil {
		t.Fatalf("retry the accept: %v", err)
	}
	if accepted.GetBooking() != first+"-2" {
		t.Errorf("the retry booked %s; want %s-2", accepted.GetBooking(), first)
	}
	e, err := f.events.GetEvent(ctx, &eventpbv1.GetEventRequest{Name: accepted.GetBooking()})
	if err != nil || e.GetState() == eventpbv1.Event_STATE_CANCELLED || len(e.GetFreebusyHolds()) != 1 {
		t.Fatalf("the retry's event: %v, holds %v, %v; want live, holding the hall", e.GetState(), e.GetFreebusyHolds(), err)
	}
	if n := f.fb.Active(); n != 2 {
		t.Errorf("%d claims after the retry; want the hall and the stay, once each", n)
	}

	// Its draft tax invoice, from the quote's lines, raised once.
	inv, err := invoices.GetInvoice(ctx, &financepbv1.GetInvoiceRequest{Name: f.venue + "/invoices/" + sales.InvoiceID(q.GetName())})
	if err != nil {
		t.Fatalf("the accepted quote's invoice: %v", err)
	}
	if inv.GetState() != financepbv1.Invoice_STATE_DRAFT || inv.GetKind() != financepbv1.InvoiceKind_INVOICE_KIND_TAX_INVOICE ||
		inv.GetBooking() != accepted.GetBooking() || inv.GetQuote() != q.GetName() || inv.GetContact() != f.contact {
		t.Errorf("invoice: %v %v, booking %q, quote %q, contact %q", inv.GetState(), inv.GetKind(), inv.GetBooking(), inv.GetQuote(), inv.GetContact())
	}
	lines := inv.GetLines()
	if len(lines) != 2 || lines[0].GetItem() != f.hall || lines[0].GetDiscount().GetUnits() != 10000 ||
		lines[1].GetQuantity() != 2 || lines[1].GetTax().GetGstRate().GetValue() != "0" {
		t.Errorf("invoice lines: %v; want the hall less the discount, and 2 rooms at no GST", lines)
	}
	_, err = invoices.CreateInvoice(ctx, &financepbv1.CreateInvoiceRequest{Parent: f.venue, InvoiceId: sales.InvoiceID(q.GetName()), Invoice: inv})
	if status.Code(err) != codes.AlreadyExists {
		t.Errorf("a second invoice under the quote's id: %v; want AlreadyExists", err)
	}
}
