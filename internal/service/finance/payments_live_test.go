// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package finance_test

// Live tests of payments over the dev database, with a fake gateway and a
// fake stay and event service: cash and a dollar payment settling an invoice, a
// gateway payment settled by its webhook (twice, harmlessly), refunded
// through the gateway, a void, and the advance confirming the booking. Run
// as billing_live_test.go says.

import (
	"context"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/shokki-engineering/aperio/internal/database/repository/aperio/guest"
	"github.com/shokki-engineering/aperio/internal/database/repository/aperio/venue"
	"github.com/shokki-engineering/aperio/internal/service/finance"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/event/v1/eventpbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/finance/v1/financepbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/guest/v1/guestpbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/stay/v1/staypbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/type/v1/typepbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/venue/v1/venuepbv1"
	"github.com/shokki-engineering/aperio/test/live"
	"google.golang.org/genproto/googleapis/type/decimal"
	"google.golang.org/genproto/googleapis/type/money"
	"google.golang.org/genproto/googleapis/type/postaladdress"
)

// testpay is a gateway whose refunds are named after what they refund.
type testpay struct{}

func (testpay) Name() string { return "testpay" }
func (testpay) Refund(_ context.Context, ref string, _ *money.Money) (string, error) {
	return "rf_" + ref, nil
}

// desk is a stay and event service holding one pending stay, recording confirms.
type desk struct{ confirmed []string }

func (d *desk) GetReservation(_ context.Context, r *staypbv1.GetReservationRequest) (*staypbv1.Reservation, error) {
	state := staypbv1.Reservation_STATE_PENDING
	if len(d.confirmed) > 0 {
		state = staypbv1.Reservation_STATE_CONFIRMED
	}
	return &staypbv1.Reservation{Name: r.GetName(), State: state}, nil
}
func (d *desk) ConfirmReservation(_ context.Context, r *staypbv1.ConfirmReservationRequest) (*staypbv1.Reservation, error) {
	d.confirmed = append(d.confirmed, r.GetName())
	return &staypbv1.Reservation{Name: r.GetName()}, nil
}
func (d *desk) GetEvent(context.Context, *eventpbv1.GetEventRequest) (*eventpbv1.Event, error) {
	return &eventpbv1.Event{}, nil
}
func (d *desk) ConfirmEvent(context.Context, *eventpbv1.ConfirmEventRequest) (*eventpbv1.Event, error) {
	return &eventpbv1.Event{}, nil
}

func TestLivePayments(t *testing.T) {
	ctx, conn := context.Background(), live.Conn(t)
	must := func(err error, what string) {
		t.Helper()
		if err != nil {
			t.Fatalf("%s: %v", what, err)
		}
	}
	venues, d := venue.New(conn), &desk{}
	svc := finance.New(live.FinanceRepositories(conn), venues.Venues, d, []finance.Gateway{testpay{}})

	prefix := strings.ToUpper(strconv.FormatInt(time.Now().UnixNano(), 36))
	prefix = prefix[len(prefix)-4:]
	v, err := venues.Venues.Create(ctx, &venuepbv1.Venue{DisplayName: "Payments Resort " + prefix, Kind: venuepbv1.VenueKind_VENUE_KIND_STAYS,
		TimeZone: "Asia/Kolkata", CurrencyCode: "INR", InvoicePrefix: prefix, Address: &postaladdress.PostalAddress{RegionCode: "IN"},
		Gst: &typepbv1.Gst{Gstin: "29ABCDE1234F1Z5", LegalDisplayName: "Test", GstStateCode: "29"}})
	must(err, "create venue")
	c, err := guest.New(conn).Contacts.Create(ctx, &guestpbv1.Contact{Kind: guestpbv1.ContactKind_CONTACT_KIND_PERSON, DisplayName: "John Doe"})
	must(err, "create contact")
	stay := v.GetName() + "/reservations/klein"

	// ₹10,000 at 18%: ₹11,800.
	inv, err := svc.CreateInvoice(ctx, &financepbv1.CreateInvoiceRequest{Parent: v.GetName(), Invoice: &financepbv1.Invoice{
		Kind: financepbv1.InvoiceKind_INVOICE_KIND_TAX_INVOICE, Contact: c.GetName(), SupplyStateCode: "29", Booking: stay,
		Lines: []*financepbv1.InvoiceLine{{Description: "Sea view, 2 nights", Quantity: 2, UnitPrice: inr(5000, 0),
			Tax: &typepbv1.Tax{HsnSacCode: "996311", GstRate: &decimal.Decimal{Value: "18"}}}},
	}})
	must(err, "create invoice")
	inv, err = svc.IssueInvoice(ctx, &financepbv1.IssueInvoiceRequest{Name: inv.GetName()})
	must(err, "issue")
	pay := func(p *financepbv1.Payment) *financepbv1.Payment {
		t.Helper()
		p.Contact, p.Invoice, p.Kind = c.GetName(), inv.GetName(), financepbv1.PaymentKind_PAYMENT_KIND_RECEIPT
		out, err := svc.CreatePayment(ctx, &financepbv1.CreatePaymentRequest{Parent: v.GetName(), Payment: p})
		must(err, "create payment")
		return out
	}
	invoice := func() *financepbv1.Invoice {
		t.Helper()
		out, err := svc.GetInvoice(ctx, &financepbv1.GetInvoiceRequest{Name: inv.GetName()})
		must(err, "get invoice")
		return out
	}

	// ₹2,000 cash: succeeded at once, the advance confirms the stay.
	cash := pay(&financepbv1.Payment{Method: financepbv1.PaymentMethod_PAYMENT_METHOD_CASH, Amount: inr(2000, 0)})
	if cash.GetState() != financepbv1.Payment_STATE_SUCCEEDED || rupees(invoice().GetDue()) != "9800.00" {
		t.Errorf("after cash: %v, due %s; want SUCCEEDED, 9800.00", cash.GetState(), rupees(invoice().GetDue()))
	}
	if len(d.confirmed) != 1 || d.confirmed[0] != stay {
		t.Errorf("confirmed %v; want the stay, once", d.confirmed)
	}
	// $60 at 83.25 = ₹4,995.
	usd := pay(&financepbv1.Payment{Method: financepbv1.PaymentMethod_PAYMENT_METHOD_CARD,
		Amount: &money.Money{CurrencyCode: "USD", Units: 60}, ExchangeRate: &decimal.Decimal{Value: "83.25"}})
	if rupees(usd.GetBaseAmount()) != "4995.00" {
		t.Errorf("$60 counted as %s; want 4995.00", rupees(usd.GetBaseAmount()))
	}
	// ₹4,805 through the gateway: pending until its webhook, which settles the invoice.
	gw := pay(&financepbv1.Payment{Method: financepbv1.PaymentMethod_PAYMENT_METHOD_GATEWAY, Amount: inr(4805, 0),
		Gateway: "testpay", GatewayReference: "pay_" + prefix})
	if gw.GetState() != financepbv1.Payment_STATE_PENDING || rupees(invoice().GetDue()) != "4805.00" {
		t.Errorf("gateway before webhook: %v, due %s; want PENDING, 4805.00", gw.GetState(), rupees(invoice().GetDue()))
	}
	for range 2 { // a retried webhook changes nothing
		_, err := svc.Settle(ctx, v.GetName(), finance.Outcome{Gateway: "testpay", Reference: "pay_" + prefix, Succeeded: true, At: time.Now()})
		must(err, "settle")
	}
	if got := invoice(); got.GetState() != financepbv1.Invoice_STATE_PAID || rupees(got.GetPaid()) != "11800.00" {
		t.Errorf("after webhook: %v, paid %s; want PAID, 11800.00", got.GetState(), rupees(got.GetPaid()))
	}

	// Refund through the gateway: pending until the refund's webhook.
	refund, err := svc.RefundPayment(ctx, &financepbv1.RefundPaymentRequest{Name: gw.GetName(), Reason: "room change"})
	must(err, "refund")
	if refund.GetState() != financepbv1.Payment_STATE_PENDING || refund.GetRefunds() != gw.GetName() || refund.GetGatewayReference() != "rf_pay_"+prefix {
		t.Errorf("refund: %v of %q, ref %q", refund.GetState(), refund.GetRefunds(), refund.GetGatewayReference())
	}
	_, err = svc.Settle(ctx, v.GetName(), finance.Outcome{Gateway: "testpay", Reference: "rf_pay_" + prefix, Succeeded: true, At: time.Now()})
	must(err, "settle refund")
	if got := invoice(); got.GetState() != financepbv1.Invoice_STATE_ISSUED || rupees(got.GetDue()) != "4805.00" {
		t.Errorf("after refund: %v, due %s; want ISSUED, 4805.00", got.GetState(), rupees(got.GetDue()))
	}
	if _, err := svc.RefundPayment(ctx, &financepbv1.RefundPaymentRequest{Name: gw.GetName()}); err == nil {
		t.Error("a payment was refunded twice")
	}

	// Voids: the cash was recorded in error; a gateway payment is not voided.
	_, err = svc.VoidPayment(ctx, &financepbv1.VoidPaymentRequest{Name: cash.GetName(), Reason: "entered twice"})
	must(err, "void")
	if rupees(invoice().GetDue()) != "6805.00" {
		t.Errorf("after void, due %s; want 6805.00", rupees(invoice().GetDue()))
	}
	if _, err := svc.VoidPayment(ctx, &financepbv1.VoidPaymentRequest{Name: gw.GetName()}); err == nil {
		t.Error("a gateway payment was voided")
	}
	if len(d.confirmed) != 1 {
		t.Errorf("the stay was confirmed %d times; want once", len(d.confirmed))
	}
}
