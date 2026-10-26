// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package finance_test

// Live test of SummarizeReceivables over the dev database: two debtors, an
// invoice not yet due, one 10 and one 45 days overdue, a paid and a
// cancelled invoice and another venue's left out, and a refund netted out of
// what was collected. Run as invoices_live_test.go says.

import (
	"context"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/shokki-engineering/aperio/internal/database/repository/aperio/guest"
	"github.com/shokki-engineering/aperio/internal/database/repository/aperio/venue"
	"github.com/shokki-engineering/aperio/internal/service/finance"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/finance/v1/financepbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/guest/v1/guestpbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/type/v1/typepbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/venue/v1/venuepbv1"
	"github.com/shokki-engineering/aperio/test/live"
	"google.golang.org/genproto/googleapis/type/date"
	"google.golang.org/genproto/googleapis/type/decimal"
	"google.golang.org/genproto/googleapis/type/postaladdress"
)

func TestLiveReceivables(t *testing.T) {
	ctx, conn := context.Background(), live.Conn(t)
	must := func(err error, what string) {
		t.Helper()
		if err != nil {
			t.Fatalf("%s: %v", what, err)
		}
	}
	venues, contacts := venue.New(conn), guest.New(conn).Contacts
	svc := finance.New(live.FinanceRepositories(conn), venues.Venues, nil, nil)
	svc.ReadContactsFrom(contacts)

	newVenue := func() *venuepbv1.Venue {
		t.Helper()
		prefix := strings.ToUpper(strconv.FormatInt(time.Now().UnixNano(), 36))
		prefix = prefix[len(prefix)-4:]
		v, err := venues.Venues.Create(ctx, &venuepbv1.Venue{DisplayName: "Receivables Resort " + prefix, Kind: venuepbv1.VenueKind_VENUE_KIND_STAYS,
			TimeZone: "Asia/Kolkata", CurrencyCode: "INR", InvoicePrefix: prefix, Address: &postaladdress.PostalAddress{RegionCode: "IN"},
			Gst: &typepbv1.Gst{Gstin: "29ABCDE1234F1Z5", LegalDisplayName: "Test", GstStateCode: "29"}})
		must(err, "create venue")
		return v
	}
	v, other := newVenue(), newVenue()
	john, err := contacts.Create(ctx, &guestpbv1.Contact{Kind: guestpbv1.ContactKind_CONTACT_KIND_PERSON, DisplayName: "John Doe",
		PhoneNumbers: []string{"+15555550100"}})
	must(err, "create John")
	jane, err := contacts.Create(ctx, &guestpbv1.Contact{Kind: guestpbv1.ContactKind_CONTACT_KIND_PERSON, DisplayName: "Jane Doe"})
	must(err, "create Jane")

	today := time.Now().In(must2(time.LoadLocation("Asia/Kolkata")))
	daysAgo := func(n int) *date.Date {
		d := today.AddDate(0, 0, -n)
		return &date.Date{Year: int32(d.Year()), Month: int32(d.Month()), Day: int32(d.Day())}
	}
	// issued is an issued invoice of venue to contact for units rupees plus
	// 18% GST, due on due (nil: no due date).
	issued := func(venue *venuepbv1.Venue, contact *guestpbv1.Contact, units int64, due *date.Date) *financepbv1.Invoice {
		t.Helper()
		inv, err := svc.CreateInvoice(ctx, &financepbv1.CreateInvoiceRequest{Parent: venue.GetName(), Invoice: &financepbv1.Invoice{
			Kind: financepbv1.InvoiceKind_INVOICE_KIND_TAX_INVOICE, Contact: contact.GetName(), SupplyStateCode: "29", DueDate: due,
			Lines: []*financepbv1.InvoiceLine{{Description: "Room", Quantity: 1, UnitPrice: inr(units, 0),
				Tax: &typepbv1.Tax{HsnSacCode: "996311", GstRate: &decimal.Decimal{Value: "18"}}}},
		}})
		must(err, "create invoice")
		inv, err = svc.IssueInvoice(ctx, &financepbv1.IssueInvoiceRequest{Name: inv.GetName()})
		must(err, "issue invoice")
		return inv
	}
	pay := func(inv *financepbv1.Invoice, units int64) *financepbv1.Payment {
		t.Helper()
		p, err := svc.CreatePayment(ctx, &financepbv1.CreatePaymentRequest{Parent: v.GetName(), Payment: &financepbv1.Payment{
			Contact: inv.GetContact(), Invoice: inv.GetName(), Kind: financepbv1.PaymentKind_PAYMENT_KIND_RECEIPT,
			Method: financepbv1.PaymentMethod_PAYMENT_METHOD_CASH, Amount: inr(units, 0)}})
		must(err, "create payment")
		return p
	}

	current := issued(v, john, 1000, nil)           // 1180, John, not due
	late10 := issued(v, john, 2000, daysAgo(10))    // 2360, John, 10 days late
	late45 := issued(v, jane, 5000, daysAgo(45))    // 5900, Jane, 45 days late
	paid := issued(v, john, 500, daysAgo(60))       // 590, paid in full: left out
	cancelled := issued(v, jane, 9000, daysAgo(70)) // cancelled: left out
	issued(other, john, 7000, daysAgo(20))          // another venue's: left out
	pay(paid, 590)                                  // collected +590
	pay(late10, 360)                                // collected +360; late10 due 2000
	refunded := pay(current, 180)                   // collected +180, then -180
	_, err = svc.RefundPayment(ctx, &financepbv1.RefundPaymentRequest{Name: refunded.GetName(), Reason: "test"})
	must(err, "refund")
	_, err = svc.CancelInvoice(ctx, &financepbv1.CancelInvoiceRequest{Name: cancelled.GetName(), Reason: "raised in error"})
	must(err, "cancel")

	got, err := svc.SummarizeReceivables(ctx, &financepbv1.SummarizeReceivablesRequest{Venue: v.GetName()})
	must(err, "summarize")
	// Outstanding 1180 + 2000 + 5900 = 9080; overdue 2000 + 5900 = 7900.
	if rupees(got.GetOutstanding()) != "9080.00" || rupees(got.GetOverdue()) != "7900.00" || got.GetOutstanding().GetCurrencyCode() != "INR" {
		t.Errorf("outstanding %v, overdue %v; want INR 9080.00, 7900.00", got.GetOutstanding(), got.GetOverdue())
	}
	if d := got.GetDate(); d.GetYear() != int32(today.Year()) || d.GetMonth() != int32(today.Month()) || d.GetDay() != int32(today.Day()) {
		t.Errorf("date %v; want today at the venue", d)
	}
	wantAging := []struct {
		age, amount string
		n           int32
	}{{"current", "1180.00", 1}, {"1-30", "2000.00", 1}, {"31-60", "5900.00", 1}, {"61-90", "0.00", 0}, {"90+", "0.00", 0}}
	if len(got.GetAging()) != len(wantAging) {
		t.Fatalf("aging %v; want 5 buckets", got.GetAging())
	}
	for i, w := range wantAging {
		if b := got.GetAging()[i]; b.GetAge() != w.age || rupees(b.GetAmount()) != w.amount || b.GetInvoices() != w.n {
			t.Errorf("aging[%d] = %s %s x%d; want %s %s x%d", i, b.GetAge(), rupees(b.GetAmount()), b.GetInvoices(), w.age, w.amount, w.n)
		}
	}
	// Jane owes 5900, John 3180: Jane first.
	if ds := got.GetDebtors(); len(ds) != 2 || ds[0].GetContact() != jane.GetName() || ds[0].GetDisplayName() != "Jane Doe" ||
		rupees(ds[0].GetDue()) != "5900.00" || ds[0].GetInvoices() != 1 || ds[0].GetOldestDueDate().GetDay() != late45.GetDueDate().GetDay() ||
		ds[1].GetContact() != john.GetName() || ds[1].GetPhone() != "+15555550100" || rupees(ds[1].GetDue()) != "3180.00" ||
		ds[1].GetInvoices() != 2 || ds[1].GetOldestDueDate().GetDay() != late10.GetDueDate().GetDay() {
		t.Errorf("debtors %v; want Jane 5900 x1, then John 3180 x2", ds)
	}
	// 590 + 360 + 180 - 180.
	if rupees(got.GetCollected()) != "950.00" {
		t.Errorf("collected %s; want 950.00", rupees(got.GetCollected()))
	}
	one, err := svc.SummarizeReceivables(ctx, &financepbv1.SummarizeReceivablesRequest{Venue: v.GetName(), Debtors: 1})
	must(err, "summarize, one debtor")
	if len(one.GetDebtors()) != 1 || one.GetDebtors()[0].GetContact() != jane.GetName() {
		t.Errorf("one debtor: %v; want Jane", one.GetDebtors())
	}
}
