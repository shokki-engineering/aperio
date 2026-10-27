// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package finance_test

// The money loop end to end (roadmap M2's exit), over the dev database with
// the real sales, stay and event services and a fake Razorpay
// (fakes_live_test.go): a quote for 3 rooms × 2 nights is accepted; its
// proforma's advance is asked for by a payment link; Razorpay's signed
// payment_link.paid webhook settles it, which confirms the stay and issues a
// receipt voucher; the tax invoice takes the advance over, and a second
// link pays the rest. Run as invoices_live_test.go says.

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shokki-engineering/aperio/internal/booking"
	"github.com/shokki-engineering/aperio/internal/database/repository/aperio/guest"
	"github.com/shokki-engineering/aperio/internal/database/repository/aperio/venue"
	"github.com/shokki-engineering/aperio/internal/gateway/razorpay"
	"github.com/shokki-engineering/aperio/internal/service/event"
	"github.com/shokki-engineering/aperio/internal/service/finance"
	"github.com/shokki-engineering/aperio/internal/service/pricing"
	"github.com/shokki-engineering/aperio/internal/service/sales"
	"github.com/shokki-engineering/aperio/internal/service/stay"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/finance/v1/financepbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/guest/v1/guestpbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/sales/v1/salespbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/stay/v1/staypbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/type/v1/typepbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/venue/v1/venuepbv1"
	"github.com/shokki-engineering/aperio/test/live"
	"google.golang.org/genproto/googleapis/type/date"
	"google.golang.org/genproto/googleapis/type/decimal"
	"google.golang.org/genproto/googleapis/type/postaladdress"
)

func TestLiveMoneyLoop(t *testing.T) {
	ctx, conn := context.Background(), live.Projected(live.Conn(t))
	must := func(err error, what string) {
		t.Helper()
		if err != nil {
			t.Fatalf("%s: %v", what, err)
		}
	}
	venues, people := venue.New(conn), guest.New(conn)
	base := booking.NewBase(live.NewFreebusy(), venues, people.Contacts)
	stays, events := stay.New(live.StayRepositories(conn), base), event.New(live.EventRepositories(conn), base)
	catalogRepos, salesRepos := live.CatalogRepositories(conn), live.SalesRepositories(conn)
	prices := pricing.New(live.PricingRepositories(conn), catalogRepos, venues.Venues)
	quotes := sales.New(salesRepos, venues.Venues, catalogRepos.Menus, prices, events, stays)
	rzp := razorpay.New(razorpay.Options{BaseURL: newFakeRazorpay(t).URL + "/v1", KeyID: "rzp_test_key", KeySecret: "key-secret"})
	money := finance.New(live.FinanceRepositories(conn), venues.Venues, confirmer{stays: stays, events: events}, []finance.Gateway{rzp})
	money.LinkPaymentsWith(rzp)
	money.ReadContactsFrom(people.Contacts)
	webhook := razorpay.NewWebhook(webhookSecret, money)

	run := strconv.FormatInt(time.Now().UnixNano(), 36)
	prefix := strings.ToUpper(run[len(run)-4:])
	v, err := venues.Venues.Create(ctx, &venuepbv1.Venue{DisplayName: "Loop Resort " + run, Kind: venuepbv1.VenueKind_VENUE_KIND_STAYS,
		TimeZone: "Asia/Kolkata", CurrencyCode: "INR", InvoicePrefix: prefix, Address: &postaladdress.PostalAddress{RegionCode: "IN"},
		Gst: &typepbv1.Gst{Gstin: "29ABCDE1234F1Z5", LegalDisplayName: "Test", GstStateCode: "29"}})
	must(err, "create venue")
	rooms, err := venues.Spaces.Create(ctx, v.GetName(), &venuepbv1.Space{DisplayName: "Deluxe", Kind: venuepbv1.SpaceKind_SPACE_KIND_ROOM_TYPE,
		Resources: []string{"resources/l1-" + run, "resources/l2-" + run, "resources/l3-" + run}})
	must(err, "create space")
	c, err := people.Contacts.Create(ctx, &guestpbv1.Contact{Kind: guestpbv1.ContactKind_CONTACT_KIND_PERSON, DisplayName: "John Doe",
		PhoneNumbers: []string{"+15555550100"}})
	must(err, "create contact")

	// Quote -> accept: 3 rooms × 2 nights at ₹6,500 + 12% = ₹43,680, a PENDING stay.
	tax := &typepbv1.Tax{HsnSacCode: "996311", GstRate: &decimal.Decimal{Value: "12"}}
	q, err := quotes.CreateQuote(ctx, &salespbv1.CreateQuoteRequest{Parent: v.GetName(), Quote: &salespbv1.Quote{
		DisplayName: "Doe stay", Contact: c.GetName(), Guests: 6,
		Dates: &typepbv1.DateRange{StartDate: &date.Date{Year: 2027, Month: 11, Day: 2}, EndDate: &date.Date{Year: 2027, Month: 11, Day: 4}},
		Lines: []*salespbv1.QuoteLine{{Kind: salespbv1.LineKind_LINE_KIND_ROOM, Item: rooms.GetName(), Quantity: 6, PriceOverride: true, UnitPrice: inr(6500, 0), Tax: tax}},
	}})
	must(err, "create quote")
	q, err = quotes.SendQuote(ctx, &salespbv1.SendQuoteRequest{Name: q.GetName()})
	must(err, "send quote")
	q, err = quotes.AcceptQuote(ctx, &salespbv1.AcceptQuoteRequest{Name: q.GetName()})
	must(err, "accept quote")
	stayState := func() staypbv1.Reservation_State {
		r, err := stays.GetReservation(ctx, &staypbv1.GetReservationRequest{Name: q.GetBooking()})
		must(err, "get stay "+q.GetBooking())
		return r.GetState()
	}
	if got := stayState(); got != staypbv1.Reservation_STATE_PENDING {
		t.Fatalf("accepted stay is %v; want PENDING", got)
	}
	invoice := func(kind financepbv1.InvoiceKind) *financepbv1.Invoice {
		t.Helper()
		inv, err := money.CreateInvoice(ctx, &financepbv1.CreateInvoiceRequest{Parent: v.GetName(), Invoice: &financepbv1.Invoice{
			Kind: kind, Contact: c.GetName(), Booking: q.GetBooking(), Quote: q.GetName(), SupplyStateCode: "29",
			Lines: []*financepbv1.InvoiceLine{{Description: "Deluxe, 3 rooms × 2 nights", Item: rooms.GetName(), Quantity: 6, UnitPrice: inr(6500, 0), Tax: tax}},
		}})
		must(err, "create "+kind.String())
		inv, err = money.IssueInvoice(ctx, &financepbv1.IssueInvoiceRequest{Name: inv.GetName()})
		must(err, "issue "+kind.String())
		return inv
	}
	get := func(name string) *financepbv1.Invoice {
		t.Helper()
		inv, err := money.GetInvoice(ctx, &financepbv1.GetInvoiceRequest{Name: name})
		must(err, "get "+name)
		return inv
	}
	// pay asks for amount (nil: all due) on inv by a link, and delivers the
	// link's signed payment_link.paid webhook, twice.
	pay := func(inv *financepbv1.Invoice, amount int64) *financepbv1.RequestPaymentResponse {
		t.Helper()
		req := &financepbv1.RequestPaymentRequest{Name: inv.GetName(), RequestId: uuid.NewString()}
		if amount > 0 {
			req.Amount = inr(amount, 0)
		}
		link, err := money.RequestPayment(ctx, req)
		must(err, "request payment")
		again, err := money.RequestPayment(ctx, req)
		must(err, "request payment again")
		if again.GetGatewayReference() != link.GetGatewayReference() || !strings.HasPrefix(link.GetUrl(), "https://rzp.io/i/") {
			t.Errorf("link %v, retried %v; want one link", link, again)
		}
		body, sig := paidEvent(link.GetGatewayReference(), inv.GetName())
		for range 2 { // a redelivery is harmless
			r := httptest.NewRequest(http.MethodPost, "/webhooks/razorpay", strings.NewReader(body))
			r.Header.Set("X-Razorpay-Signature", sig)
			w := httptest.NewRecorder()
			webhook.ServeHTTP(w, r)
			if w.Code != http.StatusOK {
				t.Fatalf("webhook: %d %s", w.Code, w.Body)
			}
		}
		return link
	}

	// The advance: ₹10,000 on the proforma, paid online.
	proforma := invoice(financepbv1.InvoiceKind_INVOICE_KIND_PROFORMA)
	if rupees(proforma.GetTotal()) != "43680.00" {
		t.Fatalf("proforma total %s; want 43680.00", rupees(proforma.GetTotal()))
	}
	advance := pay(proforma, 10000)
	payments, _, err := live.FinanceRepositories(conn).Payments.List(ctx, v.GetName(), listOf(`gateway_reference = "`+advance.GetGatewayReference()+`"`))
	must(err, "list payments")
	if len(payments) != 1 || payments[0].GetState() != financepbv1.Payment_STATE_SUCCEEDED || rupees(payments[0].GetAmount()) != "10000.00" {
		t.Fatalf("advance payments %v; want one SUCCEEDED of 10000.00", payments)
	}
	if got := stayState(); got != staypbv1.Reservation_STATE_CONFIRMED {
		t.Errorf("after the advance the stay is %v; want CONFIRMED", got)
	}
	vouchers, err := money.ListInvoices(ctx, &financepbv1.ListInvoicesRequest{Parent: v.GetName(),
		Filter: `booking = "` + q.GetBooking() + `" AND kind = RECEIPT_VOUCHER`})
	must(err, "list vouchers")
	if vs := vouchers.GetInvoices(); len(vs) != 1 || vs[0].GetState() != financepbv1.Invoice_STATE_PAID ||
		!strings.HasPrefix(vs[0].GetNumber(), prefix+"/RV") || rupees(vs[0].GetTotal()) != "10000.00" {
		t.Errorf("receipt vouchers %v; want one PAID of 10000.00 in the RV series", vs)
	}

	// The tax invoice takes the advance over and supersedes the proforma.
	taxInvoice := invoice(financepbv1.InvoiceKind_INVOICE_KIND_TAX_INVOICE)
	if taxInvoice.GetState() != financepbv1.Invoice_STATE_ISSUED || rupees(taxInvoice.GetPaid()) != "10000.00" || rupees(taxInvoice.GetDue()) != "33680.00" {
		t.Errorf("tax invoice: %v, paid %s, due %s; want ISSUED, 10000.00, 33680.00", taxInvoice.GetState(), rupees(taxInvoice.GetPaid()), rupees(taxInvoice.GetDue()))
	}
	if got := get(proforma.GetName()); got.GetState() != financepbv1.Invoice_STATE_CANCELLED {
		t.Errorf("proforma is %v; want CANCELLED, superseded", got.GetState())
	}

	// The rest, by a second link: the tax invoice is paid.
	if rest := pay(taxInvoice, 0); rupees(rest.GetAmount()) != "33680.00" {
		t.Errorf("rest asked %s; want 33680.00", rupees(rest.GetAmount()))
	}
	if got := get(taxInvoice.GetName()); got.GetState() != financepbv1.Invoice_STATE_PAID || rupees(got.GetDue()) != "0.00" {
		t.Errorf("tax invoice: %v, due %s; want PAID, 0.00", got.GetState(), rupees(got.GetDue()))
	}
	if _, err := money.RequestPayment(ctx, &financepbv1.RequestPaymentRequest{Name: taxInvoice.GetName()}); err == nil {
		t.Error("payment was requested on a paid invoice")
	}
}
