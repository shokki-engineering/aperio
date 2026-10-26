// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package finance_test

// Live tests of GST invoices over the dev database: the CGST/SGST and IGST
// split, a discount, rounding to the rupee, consecutive numbers, an issued
// invoice locked against edits, cancelled, and credited in its own series.
//
//	APERIO_TEST_DATABASE_URL="host=127.0.0.1 port=5433 user=postgres password=postgrespassword dbname=aperio sslmode=disable" \
//	  go test ./internal/service/finance/ -v

import (
	"context"
	"fmt"
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
	"google.golang.org/genproto/googleapis/type/money"
	"google.golang.org/genproto/googleapis/type/postaladdress"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
)

func inr(units int64, nanos int32) *money.Money {
	return &money.Money{CurrencyCode: "INR", Units: units, Nanos: nanos}
}

// rupees is m as "1234.50".
func rupees(m *money.Money) string {
	return fmt.Sprintf("%d.%02d", m.GetUnits(), m.GetNanos()/10_000_000)
}

func TestLiveInvoices(t *testing.T) {
	ctx, conn := context.Background(), live.Conn(t)
	must := func(err error, what string) {
		t.Helper()
		if err != nil {
			t.Fatalf("%s: %v", what, err)
		}
	}
	venues := venue.New(conn)
	svc := finance.New(live.FinanceRepositories(conn), venues.Venues, nil, nil)

	// A series of its own for each run: prefixes and numbers are unique.
	prefix := strings.ToUpper(strconv.FormatInt(time.Now().UnixNano(), 36))
	prefix = prefix[len(prefix)-4:]
	v, err := venues.Venues.Create(ctx, &venuepbv1.Venue{DisplayName: "Billing Resort " + prefix, Kind: venuepbv1.VenueKind_VENUE_KIND_STAYS,
		TimeZone: "Asia/Kolkata", CurrencyCode: "INR", InvoicePrefix: prefix, Address: &postaladdress.PostalAddress{RegionCode: "IN"},
		Gst: &typepbv1.Gst{Gstin: "29ABCDE1234F1Z5", LegalDisplayName: "Test", GstStateCode: "29"}})
	must(err, "create venue")
	c, err := guest.New(conn).Contacts.Create(ctx, &guestpbv1.Contact{Kind: guestpbv1.ContactKind_CONTACT_KIND_PERSON, DisplayName: "John Doe"})
	must(err, "create contact")

	gst18 := &typepbv1.Tax{HsnSacCode: "996311", GstRate: &decimal.Decimal{Value: "18"}}
	draft := func(supply string) *financepbv1.Invoice {
		inv, err := svc.CreateInvoice(ctx, &financepbv1.CreateInvoiceRequest{Parent: v.GetName(), Invoice: &financepbv1.Invoice{
			Kind: financepbv1.InvoiceKind_INVOICE_KIND_TAX_INVOICE, Contact: c.GetName(), SupplyStateCode: supply,
			Lines: []*financepbv1.InvoiceLine{
				{Description: "Deluxe room, 2 nights", Tax: gst18, Quantity: 2, UnitPrice: inr(5000, 0), Discount: inr(500, 0)},
				{Description: "Airport transfer", Tax: gst18, Quantity: 1, UnitPrice: inr(999, 990_000_000)},
			},
		}})
		must(err, "create invoice")
		return inv
	}

	// Within Karnataka: 9500 + 999.99 = 10499.99; CGST = SGST = 855.00 + 90.00;
	// 10499.99 + 1890.00 = 12389.99, to the rupee 12390.
	in := draft("29")
	if rupees(in.GetTaxableTotal()) != "10499.99" || rupees(in.GetCgst()) != "945.00" || rupees(in.GetSgst()) != "945.00" ||
		in.GetIgst().GetUnits() != 0 || rupees(in.GetTotal()) != "12390.00" || in.GetSupplyStateCode() != "29" {
		t.Errorf("intra-state: taxable %s, CGST %s, SGST %s, IGST %s, total %s; want 10499.99, 945.00, 945.00, 0, 12390.00",
			rupees(in.GetTaxableTotal()), rupees(in.GetCgst()), rupees(in.GetSgst()), rupees(in.GetIgst()), rupees(in.GetTotal()))
	}
	// Supplied into Maharashtra: IGST 1710.00 + 180.00.
	out := draft("27")
	if rupees(out.GetIgst()) != "1890.00" || out.GetCgst().GetUnits() != 0 {
		t.Errorf("inter-state: IGST %s, CGST %s; want 1890.00, 0", rupees(out.GetIgst()), rupees(out.GetCgst()))
	}

	day := time.Now().In(must2(time.LoadLocation("Asia/Kolkata")))
	y := day.Year()
	if day.Month() < time.April {
		y--
	}
	fy := fmt.Sprintf("%02d%02d", y%100, (y+1)%100)

	first, err := svc.IssueInvoice(ctx, &financepbv1.IssueInvoiceRequest{Name: in.GetName(), Etag: in.GetEtag()})
	must(err, "issue")
	second, err := svc.IssueInvoice(ctx, &financepbv1.IssueInvoiceRequest{Name: out.GetName()})
	must(err, "issue second")
	if want := prefix + "/" + fy + "/0001"; first.GetNumber() != want || first.GetIssueDate() == nil || first.GetState() != financepbv1.Invoice_STATE_ISSUED {
		t.Errorf("first: %q on %v, %v; want %q, issued", first.GetNumber(), first.GetIssueDate(), first.GetState(), want)
	}
	if want := prefix + "/" + fy + "/0002"; second.GetNumber() != want {
		t.Errorf("second number = %q; want %q", second.GetNumber(), want)
	}

	first.Notes = "changed"
	if _, err := svc.UpdateInvoice(ctx, &financepbv1.UpdateInvoiceRequest{Invoice: first, UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"notes"}}}); err == nil {
		t.Error("an issued invoice was edited")
	}

	note, err := svc.CreditInvoice(ctx, &financepbv1.CreditInvoiceRequest{Name: first.GetName(), Reason: "stay shortened"})
	must(err, "credit")
	if note.GetKind() != financepbv1.InvoiceKind_INVOICE_KIND_CREDIT_NOTE || note.GetCorrects() != first.GetName() || note.GetState() != financepbv1.Invoice_STATE_DRAFT {
		t.Errorf("credit note: %v correcting %q, %v", note.GetKind(), note.GetCorrects(), note.GetState())
	}
	note, err = svc.IssueInvoice(ctx, &financepbv1.IssueInvoiceRequest{Name: note.GetName()})
	must(err, "issue credit note")
	if want := prefix + "/CN" + fy + "/0001"; note.GetNumber() != want {
		t.Errorf("credit note number = %q; want %q", note.GetNumber(), want)
	}

	cancelled, err := svc.CancelInvoice(ctx, &financepbv1.CancelInvoiceRequest{Name: second.GetName(), Reason: "raised in error"})
	must(err, "cancel")
	if cancelled.GetState() != financepbv1.Invoice_STATE_CANCELLED || cancelled.GetNumber() != second.GetNumber() {
		t.Errorf("cancelled: %v, number %q; want CANCELLED keeping %q", cancelled.GetState(), cancelled.GetNumber(), second.GetNumber())
	}
	third, err := svc.IssueInvoice(ctx, &financepbv1.IssueInvoiceRequest{Name: draft("29").GetName()})
	must(err, "issue third")
	if want := prefix + "/" + fy + "/0003"; third.GetNumber() != want {
		t.Errorf("after a cancellation, number = %q; want %q (no reuse)", third.GetNumber(), want)
	}
}

func must2[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}
