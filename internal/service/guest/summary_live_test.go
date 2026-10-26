// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package guest_test

// A live test of a guest's history over the real stay, event, sales and
// finance services, with freebusy faked (test/live.Freebusy): stays past and
// to come, a cancelled one left out, counts by room type and from a day, an
// open lead, and paid less refunded beside what is still due.
//
//	APERIO_TEST_DATABASE_URL="host=127.0.0.1 port=5433 user=postgres password=postgrespassword dbname=aperio sslmode=disable" \
//	  go test ./internal/service/guest/ -v

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/shokki-engineering/aperio/internal/booking"
	"github.com/shokki-engineering/aperio/internal/database/repository/aperio/venue"
	"github.com/shokki-engineering/aperio/internal/service/event"
	"github.com/shokki-engineering/aperio/internal/service/generated/aperio/finance/v1/financepbv1svc"
	"github.com/shokki-engineering/aperio/internal/service/generated/aperio/sales/v1/salespbv1svc"
	"github.com/shokki-engineering/aperio/internal/service/guest"
	"github.com/shokki-engineering/aperio/internal/service/stay"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/finance/v1/financepbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/guest/v1/guestpbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/sales/v1/salespbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/stay/v1/staypbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/type/v1/typepbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/venue/v1/venuepbv1"
	"github.com/shokki-engineering/aperio/test/live"
	"google.golang.org/genproto/googleapis/type/date"
	"google.golang.org/genproto/googleapis/type/money"
	"google.golang.org/genproto/googleapis/type/postaladdress"
)

func TestLiveSummarizeContact(t *testing.T) {
	ctx, conn := context.Background(), live.Projected(live.Conn(t))
	must := func(err error, what string) {
		t.Helper()
		if err != nil {
			t.Fatalf("%s: %v", what, err)
		}
	}
	run, year := fmt.Sprint(time.Now().UnixNano()), int32(time.Now().Year())
	venues := venue.New(conn)
	v, err := venues.Venues.Create(ctx, &venuepbv1.Venue{DisplayName: "History Resort " + run, Kind: venuepbv1.VenueKind_VENUE_KIND_STAYS,
		TimeZone: "Asia/Kolkata", CurrencyCode: "INR", Address: &postaladdress.PostalAddress{RegionCode: "IN"},
		Gst: &typepbv1.Gst{Gstin: "29ABCDE1234F1Z5", LegalDisplayName: "Test", GstStateCode: "29"}})
	must(err, "create venue")
	space := func(name string) string {
		sp, err := venues.Spaces.Create(ctx, v.GetName(), &venuepbv1.Space{DisplayName: name, Kind: venuepbv1.SpaceKind_SPACE_KIND_UNIT, Resources: []string{"resources/" + name + "-" + run}})
		must(err, "create "+name)
		return sp.GetName()
	}
	villa, cottage := space("Garden Villa"), space("Cottage")
	guests := guest.New(live.GuestRepositories(conn))
	john, err := guests.CreateContact(ctx, &guestpbv1.CreateContactRequest{Contact: &guestpbv1.Contact{Kind: guestpbv1.ContactKind_CONTACT_KIND_PERSON, DisplayName: "John Doe"}})
	must(err, "create contact")

	base := booking.NewBase(live.NewFreebusy(), venues, live.GuestRepositories(conn).Contacts)
	base.Search = conn
	stays := stay.New(live.StayRepositories(conn), base)
	sales := salespbv1svc.NewSalesServiceServer(live.SalesRepositories(conn), salespbv1svc.NopSalesServiceHooks{})
	finance := financepbv1svc.NewFinanceServiceServer(live.FinanceRepositories(conn), financepbv1svc.NopFinanceServiceHooks{})
	guests.ReadHistoryFrom(guest.History{Stays: stays, Events: event.New(live.EventRepositories(conn), base), Sales: sales, Finance: finance, Spaces: venues.Spaces})

	book := func(id, sp string, y, m, d, nights int32) *staypbv1.Reservation {
		r, err := stays.CreateReservation(ctx, &staypbv1.CreateReservationRequest{Parent: v.GetName(), ReservationId: id + "-" + run, Reservation: &staypbv1.Reservation{
			Contact: john.GetName(), Channel: typepbv1.Channel_CHANNEL_WEBSITE, Adults: 2,
			ArrivalDate: &date.Date{Year: y, Month: m, Day: d}, DepartureDate: &date.Date{Year: y, Month: m, Day: d + nights},
			Rooms:           []*staypbv1.RoomLine{{Space: sp, Rooms: 1}},
			SpecialRequests: []*typepbv1.SpecialRequest{{Summary: "a sea-facing table"}},
		}})
		must(err, "book "+id)
		return r
	}
	book("last-year", villa, year-1, 3, 10, 2)
	book("spring", villa, year, 2, 1, 3)
	book("cottage", cottage, year, 3, 1, 1)
	next := book("next", villa, year+1, 1, 5, 2)
	gone := book("gone", villa, year, 4, 1, 1)
	_, err = stays.CancelReservation(ctx, &staypbv1.CancelReservationRequest{Name: gone.GetName()})
	must(err, "cancel")

	_, err = live.SalesRepositories(conn).Leads.Create(ctx, &salespbv1.Lead{DisplayName: "John Doe's anniversary", Contact: john.GetName(), Stage: salespbv1.Stage_STAGE_CONTACTED})
	must(err, "create lead")
	fin := live.FinanceRepositories(conn)
	inr := func(units int64) *money.Money { return &money.Money{CurrencyCode: "INR", Units: units} }
	_, err = fin.Invoices.Create(ctx, v.GetName(), &financepbv1.Invoice{Contact: john.GetName(), State: financepbv1.Invoice_STATE_ISSUED, SupplyStateCode: "29",
		Lines: []*financepbv1.InvoiceLine{{Description: "Garden Villa, 3 nights", Quantity: 1, UnitPrice: inr(20000)}}, Due: inr(5000), Total: inr(20000)})
	must(err, "create invoice")
	for _, p := range []*financepbv1.Payment{
		{Contact: john.GetName(), Kind: financepbv1.PaymentKind_PAYMENT_KIND_RECEIPT, Amount: inr(20000), BaseAmount: inr(20000), State: financepbv1.Payment_STATE_SUCCEEDED},
		{Contact: john.GetName(), Kind: financepbv1.PaymentKind_PAYMENT_KIND_REFUND, Amount: inr(3000), BaseAmount: inr(3000), State: financepbv1.Payment_STATE_SUCCEEDED},
		{Contact: john.GetName(), Kind: financepbv1.PaymentKind_PAYMENT_KIND_RECEIPT, Amount: inr(9000), BaseAmount: inr(9000), State: financepbv1.Payment_STATE_FAILED},
	} {
		_, err = fin.Payments.Create(ctx, v.GetName(), p)
		must(err, "create payment")
	}

	all, err := guests.SummarizeContact(ctx, &guestpbv1.SummarizeContactRequest{Name: john.GetName()})
	must(err, "summarize")
	if all.GetStays() != 4 || all.GetNights() != 8 || all.GetDisplayName() != "John Doe" {
		t.Errorf("all time: %d stays, %d nights, %q; want 4 stays, 8 nights, John Doe", all.GetStays(), all.GetNights(), all.GetDisplayName())
	}
	if rt := all.GetRoomTypes(); len(rt) != 2 || rt[0].GetDisplayName() != "Garden Villa" || rt[0].GetStays() != 3 || rt[1].GetStays() != 1 {
		t.Errorf("room types = %v; want the villa 3 times, the cottage once", rt)
	}
	if fmt.Sprint(all.GetNextArrival()) != fmt.Sprint(next.GetArrivalDate()) || all.GetLastDeparture().GetYear() != year || all.GetLastDeparture().GetMonth() != 3 {
		t.Errorf("next arrival %v, last departure %v; want %v and 2 March", all.GetNextArrival(), all.GetLastDeparture(), next.GetArrivalDate())
	}
	if all.GetOpenLeads() != 1 || fmt.Sprint(all.GetPaid()) != fmt.Sprint([]*money.Money{inr(17000)}) || fmt.Sprint(all.GetDue()) != fmt.Sprint([]*money.Money{inr(5000)}) {
		t.Errorf("leads %d, paid %v, due %v; want 1 lead, 17,000 paid, 5,000 due", all.GetOpenLeads(), all.GetPaid(), all.GetDue())
	}
	if fmt.Sprint(all.GetRequests()) != "[a sea-facing table]" {
		t.Errorf("requests = %v; want the table, once", all.GetRequests())
	}

	thisYear, err := guests.SummarizeContact(ctx, &guestpbv1.SummarizeContactRequest{Name: john.GetName(), StartDate: &date.Date{Year: year, Month: 1, Day: 1}})
	must(err, "summarize this year")
	if rt := thisYear.GetRoomTypes(); thisYear.GetStays() != 3 || len(rt) != 2 || rt[0].GetSpace() != villa || rt[0].GetStays() != 2 {
		t.Errorf("this year: %d stays, room types %v; want 3 stays, the villa twice", thisYear.GetStays(), rt)
	}
}
