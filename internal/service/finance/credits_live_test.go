// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package finance_test

// Live tests over the dev database of a credit note bringing down the due
// of the invoice it corrects, a cheque cleared, and a cash advance against
// no invoice recorded on a receipt voucher at 18%. Run as
// invoices_live_test.go says.

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
	"google.golang.org/genproto/googleapis/type/decimal"
	"google.golang.org/genproto/googleapis/type/postaladdress"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
)

func TestLiveCreditsAndClearing(t *testing.T) {
	ctx, conn := context.Background(), live.Conn(t)
	must := func(err error, what string) {
		t.Helper()
		if err != nil {
			t.Fatalf("%s: %v", what, err)
		}
	}
	venues, d := venue.New(conn), &desk{}
	svc := finance.New(live.FinanceRepositories(conn), venues.Venues, d, nil)
	run := strconv.FormatInt(time.Now().UnixNano(), 36)
	prefix := strings.ToUpper(run[len(run)-4:])
	v, err := venues.Venues.Create(ctx, &venuepbv1.Venue{DisplayName: "Credits Resort " + prefix, Kind: venuepbv1.VenueKind_VENUE_KIND_STAYS,
		TimeZone: "Asia/Kolkata", CurrencyCode: "INR", InvoicePrefix: prefix, Address: &postaladdress.PostalAddress{RegionCode: "IN"},
		Gst: &typepbv1.Gst{Gstin: "29ABCDE1234F1Z5", LegalDisplayName: "Test", GstStateCode: "29"}})
	must(err, "create venue")
	c, err := guest.New(conn).Contacts.Create(ctx, &guestpbv1.Contact{Kind: guestpbv1.ContactKind_CONTACT_KIND_PERSON, DisplayName: "John Doe"})
	must(err, "create contact")
	get := func(name string) *financepbv1.Invoice {
		t.Helper()
		inv, err := svc.GetInvoice(ctx, &financepbv1.GetInvoiceRequest{Name: name})
		must(err, "get "+name)
		return inv
	}

	// ₹10,000 at 18% = ₹11,800, ₹1,000 of it paid by cheque.
	inv, err := svc.CreateInvoice(ctx, &financepbv1.CreateInvoiceRequest{Parent: v.GetName(), Invoice: &financepbv1.Invoice{
		Kind: financepbv1.InvoiceKind_INVOICE_KIND_TAX_INVOICE, Contact: c.GetName(), SupplyStateCode: "29",
		Lines: []*financepbv1.InvoiceLine{
			{Description: "Sea view, 1 night", Quantity: 1, UnitPrice: inr(5000, 0), Tax: &typepbv1.Tax{HsnSacCode: "996311", GstRate: &decimal.Decimal{Value: "18"}}},
			{Description: "Sea view, 1 night", Quantity: 1, UnitPrice: inr(5000, 0), Tax: &typepbv1.Tax{HsnSacCode: "996311", GstRate: &decimal.Decimal{Value: "18"}}},
		},
	}})
	must(err, "create invoice")
	inv, err = svc.IssueInvoice(ctx, &financepbv1.IssueInvoiceRequest{Name: inv.GetName()})
	must(err, "issue")
	cheque, err := svc.CreatePayment(ctx, &financepbv1.CreatePaymentRequest{Parent: v.GetName(), Payment: &financepbv1.Payment{
		Contact: c.GetName(), Invoice: inv.GetName(), Kind: financepbv1.PaymentKind_PAYMENT_KIND_RECEIPT,
		Method: financepbv1.PaymentMethod_PAYMENT_METHOD_CHEQUE, Amount: inr(1000, 0), Reference: "000123"}})
	must(err, "record cheque")
	if cheque.GetState() != financepbv1.Payment_STATE_PENDING || rupees(get(inv.GetName()).GetDue()) != "11800.00" {
		t.Errorf("uncleared cheque: %v, due %s; want PENDING, 11800.00", cheque.GetState(), rupees(get(inv.GetName()).GetDue()))
	}
	cleared, err := svc.ClearPayment(ctx, &financepbv1.ClearPaymentRequest{Name: cheque.GetName()})
	must(err, "clear")
	if cleared.GetState() != financepbv1.Payment_STATE_SUCCEEDED || cleared.GetReceiveTime() == nil || rupees(get(inv.GetName()).GetDue()) != "10800.00" {
		t.Errorf("cleared: %v at %v, due %s; want SUCCEEDED, 10800.00", cleared.GetState(), cleared.GetReceiveTime(), rupees(get(inv.GetName()).GetDue()))
	}
	if _, err := svc.ClearPayment(ctx, &financepbv1.ClearPaymentRequest{Name: cheque.GetName()}); err == nil {
		t.Error("a cleared cheque was cleared again")
	}

	// A credit note for one night: ₹5,900 off what is due, settled at issue.
	note, err := svc.CreditInvoice(ctx, &financepbv1.CreditInvoiceRequest{Name: inv.GetName(), Reason: "stay shortened"})
	must(err, "credit")
	note.Lines = note.GetLines()[:1]
	_, err = svc.UpdateInvoice(ctx, &financepbv1.UpdateInvoiceRequest{Invoice: note, UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"lines"}}})
	must(err, "cut the credit note down")
	note, err = svc.IssueInvoice(ctx, &financepbv1.IssueInvoiceRequest{Name: note.GetName()})
	must(err, "issue credit note")
	if note.GetState() != financepbv1.Invoice_STATE_PAID || rupees(note.GetDue()) != "0.00" || rupees(note.GetTotal()) != "5900.00" {
		t.Errorf("credit note: %v, total %s, due %s; want PAID, 5900.00, 0.00", note.GetState(), rupees(note.GetTotal()), rupees(note.GetDue()))
	}
	if got := get(inv.GetName()); got.GetState() != financepbv1.Invoice_STATE_ISSUED || rupees(got.GetDue()) != "4900.00" || rupees(got.GetPaid()) != "1000.00" {
		t.Errorf("credited invoice: %v, paid %s, due %s; want ISSUED, 1000.00, 4900.00", got.GetState(), rupees(got.GetPaid()), rupees(got.GetDue()))
	}
	sum, err := svc.SummarizeReceivables(ctx, &financepbv1.SummarizeReceivablesRequest{Venue: v.GetName()})
	must(err, "summarize")
	if rupees(sum.GetOutstanding()) != "4900.00" {
		t.Errorf("outstanding %s; want 4900.00, the credit note not counted", rupees(sum.GetOutstanding()))
	}

	// ₹11,800 cash for a booking with no invoice: an advance, ₹10,000 + 18%.
	stay := v.GetName() + "/reservations/doe"
	_, err = svc.CreatePayment(ctx, &financepbv1.CreatePaymentRequest{Parent: v.GetName(), Payment: &financepbv1.Payment{
		Contact: c.GetName(), Booking: stay, Kind: financepbv1.PaymentKind_PAYMENT_KIND_RECEIPT,
		Method: financepbv1.PaymentMethod_PAYMENT_METHOD_CASH, Amount: inr(11800, 0)}})
	must(err, "cash advance")
	vouchers, err := svc.ListInvoices(ctx, &financepbv1.ListInvoicesRequest{Parent: v.GetName(), Filter: `booking = "` + stay + `"`})
	must(err, "list vouchers")
	if vs := vouchers.GetInvoices(); len(vs) != 1 || vs[0].GetKind() != financepbv1.InvoiceKind_INVOICE_KIND_RECEIPT_VOUCHER ||
		rupees(vs[0].GetTaxableTotal()) != "10000.00" || rupees(vs[0].GetTotal()) != "11800.00" || vs[0].GetLines()[0].GetTax().GetHsnSacCode() != "9963" {
		t.Errorf("vouchers %v; want one, 10000.00 + 18%% under 9963", vs)
	}
	if len(d.confirmed) != 1 || d.confirmed[0] != stay {
		t.Errorf("confirmed %v; want the stay", d.confirmed)
	}
}
