// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package finance_test

// A live test of the refund voucher (CGST Rules, rule 51): an advance paid
// against a booking's proforma gets a receipt voucher, and its refund a
// credit note against that voucher -- once, however often the refund's
// webhook comes. Run as billing_live_test.go says.

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
)

func TestLiveRefundVoucher(t *testing.T) {
	ctx, conn := context.Background(), live.Conn(t)
	must := func(err error, what string) {
		t.Helper()
		if err != nil {
			t.Fatalf("%s: %v", what, err)
		}
	}
	venues := venue.New(conn)
	svc := finance.New(live.FinanceRepositories(conn), venues.Venues, &desk{}, []finance.Gateway{testpay{}})
	prefix := strings.ToUpper(strconv.FormatInt(time.Now().UnixNano(), 36))
	prefix = prefix[len(prefix)-4:]
	v, err := venues.Venues.Create(ctx, &venuepbv1.Venue{DisplayName: "Refunds Resort " + prefix, Kind: venuepbv1.VenueKind_VENUE_KIND_STAYS,
		TimeZone: "Asia/Kolkata", CurrencyCode: "INR", InvoicePrefix: prefix, Address: &postaladdress.PostalAddress{RegionCode: "IN"},
		Gst: &typepbv1.Gst{Gstin: "29ABCDE1234F1Z5", LegalDisplayName: "Test", GstStateCode: "29"}})
	must(err, "create venue")
	c, err := guest.New(conn).Contacts.Create(ctx, &guestpbv1.Contact{Kind: guestpbv1.ContactKind_CONTACT_KIND_PERSON, DisplayName: "John Doe"})
	must(err, "create contact")
	stay := v.GetName() + "/reservations/doe"

	// The proforma: ₹10,000 at 12%, ₹11,200; half of it paid as an advance.
	pf, err := svc.CreateInvoice(ctx, &financepbv1.CreateInvoiceRequest{Parent: v.GetName(), Invoice: &financepbv1.Invoice{
		Kind: financepbv1.InvoiceKind_INVOICE_KIND_PROFORMA, Contact: c.GetName(), SupplyStateCode: "29", Booking: stay,
		Lines: []*financepbv1.InvoiceLine{{Description: "Garden room, 2 nights", Quantity: 2, UnitPrice: inr(5000, 0),
			Tax: &typepbv1.Tax{HsnSacCode: "996311", GstRate: &decimal.Decimal{Value: "12"}}}},
	}})
	must(err, "create proforma")
	pf, err = svc.IssueInvoice(ctx, &financepbv1.IssueInvoiceRequest{Name: pf.GetName()})
	must(err, "issue proforma")
	gw, err := svc.CreatePayment(ctx, &financepbv1.CreatePaymentRequest{Parent: v.GetName(), Payment: &financepbv1.Payment{
		Contact: c.GetName(), Invoice: pf.GetName(), Kind: financepbv1.PaymentKind_PAYMENT_KIND_RECEIPT,
		Method: financepbv1.PaymentMethod_PAYMENT_METHOD_GATEWAY, Amount: inr(5600, 0), Gateway: "testpay", GatewayReference: "pay_rv_" + prefix,
	}})
	must(err, "create payment")
	_, err = svc.Settle(ctx, v.GetName(), finance.Outcome{Gateway: "testpay", Reference: "pay_rv_" + prefix, Succeeded: true, At: time.Now()})
	must(err, "settle payment")

	ofKind := func(kind string) []*financepbv1.Invoice {
		t.Helper()
		out, err := svc.ListInvoices(ctx, &financepbv1.ListInvoicesRequest{Parent: v.GetName(),
			Filter: `booking = "` + stay + `" AND kind = ` + kind})
		must(err, "list "+kind)
		return out.GetInvoices()
	}
	rvs := ofKind("RECEIPT_VOUCHER")
	if len(rvs) != 1 || rupees(rvs[0].GetTotal()) != "5600.00" {
		t.Fatalf("receipt vouchers %v; want one of 5600.00", rvs)
	}

	refund, err := svc.RefundPayment(ctx, &financepbv1.RefundPaymentRequest{Name: gw.GetName(), Reason: "stay called off"})
	must(err, "refund")
	if notes := ofKind("CREDIT_NOTE"); len(notes) != 0 {
		t.Errorf("a refund voucher before the refund went through: %v", notes)
	}
	for range 2 { // a retried webhook issues no second voucher
		_, err = svc.Settle(ctx, v.GetName(), finance.Outcome{Gateway: "testpay", Reference: refund.GetGatewayReference(), Succeeded: true, At: time.Now()})
		must(err, "settle refund")
	}
	notes := ofKind("CREDIT_NOTE")
	if len(notes) != 1 {
		t.Fatalf("refund vouchers %d; want one", len(notes))
	}
	n := notes[0]
	if n.GetCorrects() != rvs[0].GetName() || n.GetState() != financepbv1.Invoice_STATE_PAID || rupees(n.GetTotal()) != "5600.00" ||
		rupees(n.GetCgst()) != rupees(rvs[0].GetCgst()) || !strings.HasPrefix(n.GetNumber(), prefix+"/CN") ||
		!strings.Contains(n.GetNotes(), "rule 51") || !strings.Contains(n.GetNotes(), refund.GetName()) {
		t.Errorf("refund voucher %s: corrects %q, %v, total %s, CGST %s, notes %q; want the receipt voucher reversed, PAID, 5600.00",
			n.GetNumber(), n.GetCorrects(), n.GetState(), rupees(n.GetTotal()), rupees(n.GetCgst()), n.GetNotes())
	}
	if got, _ := svc.GetInvoice(ctx, &financepbv1.GetInvoiceRequest{Name: rvs[0].GetName()}); got.GetState() != financepbv1.Invoice_STATE_PAID {
		t.Errorf("the receipt voucher is %v; want left PAID", got.GetState())
	}

	// A cash advance on no invoice, refunded at once: its voucher too.
	cash, err := svc.CreatePayment(ctx, &financepbv1.CreatePaymentRequest{Parent: v.GetName(), Payment: &financepbv1.Payment{
		Contact: c.GetName(), Booking: stay, Kind: financepbv1.PaymentKind_PAYMENT_KIND_RECEIPT,
		Method: financepbv1.PaymentMethod_PAYMENT_METHOD_CASH, Amount: inr(1180, 0),
	}})
	must(err, "cash advance")
	_, err = svc.RefundPayment(ctx, &financepbv1.RefundPaymentRequest{Name: cash.GetName()})
	must(err, "refund cash")
	if got := ofKind("CREDIT_NOTE"); len(got) != 2 {
		t.Errorf("refund vouchers after the cash refund: %d; want 2", len(got))
	}
}
