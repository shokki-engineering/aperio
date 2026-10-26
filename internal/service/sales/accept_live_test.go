// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package sales_test

// A live test of AcceptQuote over the dev database, with the booking service
// in front of a freebusy that always has room: a wedding quote with 3 rooms
// for 2 nights becomes an event and a stay of 3 rooms, its lead is won, and
// a second accept is refused. Run as sales_live_test.go says.

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/shokki-engineering/aperio/internal/database/repository/aperio/crm"
	"github.com/shokki-engineering/aperio/internal/database/repository/aperio/venue"
	"github.com/shokki-engineering/aperio/internal/engine"
	"github.com/shokki-engineering/aperio/internal/service/booking"
	"github.com/shokki-engineering/aperio/internal/service/sales"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/booking/v1/bookingpbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/crm/v1/crmpbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/sales/v1/salespbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/type/v1/typepbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/venue/v1/venuepbv1"
	"github.com/shokki-engineering/aperio/test/live"
	"google.golang.org/genproto/googleapis/type/date"
	"google.golang.org/genproto/googleapis/type/decimal"
	"google.golang.org/genproto/googleapis/type/postaladdress"
)

// roomy is a freebusy with room for everything.
type roomy struct{}

func (roomy) Free(context.Context, []string, engine.Window, int32) (bool, error) { return true, nil }
func (roomy) Book(_ context.Context, b engine.Booking) (string, error) {
	return "reservations/" + b.Key, nil
}
func (roomy) Cancel(context.Context, string, string) error    { return nil }
func (roomy) Note(context.Context, string, engine.Note) error { return nil }

func TestLiveAcceptQuote(t *testing.T) {
	ctx, conn := context.Background(), live.Conn(t)
	must := func(err error, what string) {
		t.Helper()
		if err != nil {
			t.Fatalf("%s: %v", what, err)
		}
	}
	venues, people := venue.New(conn), crm.New(conn)
	booker := booking.New(live.BookingRepositories(conn), venues, people.Contacts, roomy{})
	svc := sales.New(live.SalesRepositories(conn), venues.Venues, people.Leads, booker)

	run := fmt.Sprint(time.Now().UnixNano())
	v, err := venues.Venues.Create(ctx, &venuepbv1.Venue{DisplayName: "Accept Resort " + run,
		Kind: venuepbv1.VenueKind_VENUE_KIND_STAYS_AND_EVENTS, TimeZone: "Asia/Kolkata", CurrencyCode: "INR",
		Address: &postaladdress.PostalAddress{RegionCode: "IN"}, Gst: &typepbv1.Gst{Gstin: "29ABCDE1234F1Z5", LegalDisplayName: "Test", GstStateCode: "29"}})
	must(err, "create venue")
	rooms, err := venues.Spaces.Create(ctx, v.GetName(), &venuepbv1.Space{DisplayName: "Deluxe", Kind: venuepbv1.SpaceKind_SPACE_KIND_ROOM_TYPE,
		Resources: []string{"resources/d1-" + run, "resources/d2-" + run, "resources/d3-" + run}})
	must(err, "create space")
	c, err := people.Contacts.Create(ctx, &crmpbv1.Contact{Kind: crmpbv1.ContactKind_CONTACT_KIND_PERSON, DisplayName: "Rahul Menon"})
	must(err, "create contact")
	lead, err := people.Leads.Create(ctx, &crmpbv1.Lead{DisplayName: "Menon wedding", Contact: c.GetName(), Venue: v.GetName(),
		Kind: crmpbv1.LeadKind_LEAD_KIND_EVENT_WITH_STAY, Source: typepbv1.Channel_CHANNEL_WHATSAPP, Stage: crmpbv1.Stage_STAGE_PROPOSAL})
	must(err, "create lead")

	// 3 rooms × 2 nights, 12-14 Dec, and the hall's decor.
	dates := &typepbv1.DateRange{StartDate: &date.Date{Year: 2027, Month: 12, Day: 12}, EndDate: &date.Date{Year: 2027, Month: 12, Day: 14}}
	stayTax := &typepbv1.Tax{HsnSacCode: "996311", GstRate: &decimal.Decimal{Value: "12"}}
	q, err := svc.CreateQuote(ctx, &salespbv1.CreateQuoteRequest{Parent: v.GetName(), Quote: &salespbv1.Quote{
		DisplayName: "Menon wedding", Contact: c.GetName(), Lead: lead.GetName(), EventType: typepbv1.EventType_EVENT_TYPE_WEDDING,
		Dates: dates, Guests: 6,
		Lines: []*salespbv1.QuoteLine{
			{Kind: salespbv1.LineKind_LINE_KIND_ROOM, Item: rooms.GetName(), Quantity: 6, PriceOverride: true, UnitPrice: inr(6500), Tax: stayTax},
			{Kind: salespbv1.LineKind_LINE_KIND_SERVICE, Description: "Mandap decor", Quantity: 1, UnitPrice: inr(80000)},
		},
	}})
	must(err, "create quote")
	q, err = svc.SendQuote(ctx, &salespbv1.SendQuoteRequest{Name: q.GetName()})
	must(err, "send quote")

	q, err = svc.AcceptQuote(ctx, &salespbv1.AcceptQuoteRequest{Name: q.GetName(), Etag: q.GetEtag()})
	must(err, "accept quote")
	if q.GetState() != salespbv1.Quote_STATE_ACCEPTED || q.GetAcceptTime() == nil {
		t.Errorf("quote: state %v, accept_time %v", q.GetState(), q.GetAcceptTime())
	}
	ev, err := booker.GetEvent(ctx, &bookingpbv1.GetEventRequest{Name: q.GetBooking()})
	must(err, "get event "+q.GetBooking())
	if ev.GetQuote() != q.GetName() || ev.GetEventType() != typepbv1.EventType_EVENT_TYPE_WEDDING {
		t.Errorf("event: quote %q, type %v", ev.GetQuote(), ev.GetEventType())
	}
	l, err := people.Leads.Get(ctx, lead.GetName())
	must(err, "get lead")
	if l.GetStage() != crmpbv1.Stage_STAGE_WON || len(l.GetBookings()) != 2 || l.GetCloseTime() == nil {
		t.Fatalf("lead: stage %v, bookings %v, close_time %v; want WON with the event and the stay", l.GetStage(), l.GetBookings(), l.GetCloseTime())
	}
	stay, err := booker.GetReservation(ctx, &bookingpbv1.GetReservationRequest{Name: l.GetBookings()[1]})
	must(err, "get stay")
	if got := stay.GetRooms()[0].GetRooms(); got != 3 || stay.GetEvent() != ev.GetName() || stay.GetChannel() != typepbv1.Channel_CHANNEL_WHATSAPP {
		t.Errorf("stay: %d rooms, event %q, channel %v; want 3 rooms of the event, from WhatsApp", got, stay.GetEvent(), stay.GetChannel())
	}
	if len(stay.GetFreebusyReservations()) != 1 {
		t.Errorf("stay holds %v in freebusy; want one reservation", stay.GetFreebusyReservations())
	}

	if _, err := svc.AcceptQuote(ctx, &salespbv1.AcceptQuoteRequest{Name: q.GetName()}); err == nil {
		t.Error("an accepted quote was accepted again")
	}
}
