// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package sales_test

// Live tests of what sales holds in freebusy (kept in memory,
// test/live.Freebusy) and of an offer's clock: John Doe's lead holds its
// rooms and lets them go when lost or lapsed; an accepted wedding holds its
// hall, so a second quote for the same hall and date cannot be accepted; a
// quote sent with a ttl stands that long, and the expiry job lapses it. Run
// as sales_live_test.go says.

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/shokki-engineering/aperio/internal/booking"
	"github.com/shokki-engineering/aperio/internal/database/repository/aperio/guest"
	"github.com/shokki-engineering/aperio/internal/database/repository/aperio/venue"
	"github.com/shokki-engineering/aperio/internal/service/event"
	"github.com/shokki-engineering/aperio/internal/service/generated/aperio/sales/v1/salespbv1svc"
	"github.com/shokki-engineering/aperio/internal/service/pricing"
	"github.com/shokki-engineering/aperio/internal/service/sales"
	"github.com/shokki-engineering/aperio/internal/service/stay"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/event/v1/eventpbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/guest/v1/guestpbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/sales/v1/salespbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/type/v1/typepbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/venue/v1/venuepbv1"
	"github.com/shokki-engineering/aperio/test/live"
	"google.golang.org/genproto/googleapis/type/postaladdress"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// holdFixture is a venue with two rooms and a hall, John Doe, and the sales
// service holding through a fake freebusy.
type holdFixture struct {
	svc     *sales.Server
	events  *event.Server
	repos   salespbv1svc.SalesServiceRepositories
	fb      *live.Freebusy
	venue   string
	rooms   string
	hall    string
	contact string
}

func holdSetup(t *testing.T) *holdFixture {
	t.Helper()
	ctx, conn := context.Background(), live.Conn(t)
	run := fmt.Sprint(time.Now().UnixNano())
	venues, people := venue.New(conn), guest.New(conn)
	v, err := venues.Venues.Create(ctx, &venuepbv1.Venue{DisplayName: "Hold Resort " + run,
		Kind: venuepbv1.VenueKind_VENUE_KIND_STAYS_AND_EVENTS, TimeZone: "Asia/Kolkata", CurrencyCode: "INR",
		Address: &postaladdress.PostalAddress{RegionCode: "IN"}, Gst: &typepbv1.Gst{Gstin: "29ABCDE1234F1Z5", LegalDisplayName: "Test", GstStateCode: "29"}})
	if err != nil {
		t.Fatalf("create venue: %v", err)
	}
	rooms, err := venues.Spaces.Create(ctx, v.GetName(), &venuepbv1.Space{DisplayName: "Deluxe", Kind: venuepbv1.SpaceKind_SPACE_KIND_ROOM_TYPE,
		Resources: []string{"resources/d1-" + run, "resources/d2-" + run}})
	if err != nil {
		t.Fatalf("create rooms: %v", err)
	}
	hall, err := venues.Spaces.Create(ctx, v.GetName(), &venuepbv1.Space{DisplayName: "Ballroom", Kind: venuepbv1.SpaceKind_SPACE_KIND_HALL,
		Resources: []string{"resources/hall-" + run}})
	if err != nil {
		t.Fatalf("create hall: %v", err)
	}
	c, err := people.Contacts.Create(ctx, &guestpbv1.Contact{Kind: guestpbv1.ContactKind_CONTACT_KIND_PERSON, DisplayName: "John Doe"})
	if err != nil {
		t.Fatalf("create contact: %v", err)
	}
	fb := live.NewFreebusy()
	base := booking.NewBase(fb, venues, people.Contacts)
	catalogRepos := live.CatalogRepositories(conn)
	repos := live.SalesRepositories(conn)
	events := event.New(live.EventRepositories(conn), base)
	svc := sales.New(repos, venues.Venues, catalogRepos.Menus, pricing.New(live.PricingRepositories(conn), catalogRepos, venues.Venues),
		events, stay.New(live.StayRepositories(conn), base))
	svc.HoldWith(base)
	return &holdFixture{svc: svc, events: events, repos: repos, fb: fb, venue: v.GetName(), rooms: rooms.GetName(), hall: hall.GetName(), contact: c.GetName()}
}

// wedding is a sent quote for the hall on 18 Dec 2027.
func (f *holdFixture) wedding(t *testing.T, ctx context.Context) *salespbv1.Quote {
	t.Helper()
	q, err := f.svc.CreateQuote(ctx, &salespbv1.CreateQuoteRequest{Parent: f.venue, Quote: &salespbv1.Quote{
		DisplayName: "Doe wedding", Contact: f.contact, EventType: typepbv1.EventType_EVENT_TYPE_WEDDING, Guests: 200,
		Dates: dateRange(2027, 12, 18, 18),
		Lines: []*salespbv1.QuoteLine{{Kind: salespbv1.LineKind_LINE_KIND_SPACE, Item: f.hall, Quantity: 1, PriceOverride: true, UnitPrice: inr(150000)}},
	}})
	if err != nil {
		t.Fatalf("create quote: %v", err)
	}
	if q, err = f.svc.SendQuote(ctx, &salespbv1.SendQuoteRequest{Name: q.GetName()}); err != nil {
		t.Fatalf("send quote: %v", err)
	}
	return q
}

func TestLiveLeadHold(t *testing.T) {
	f, ctx := holdSetup(t), context.Background()
	l, err := f.svc.CreateLead(ctx, &salespbv1.CreateLeadRequest{Lead: &salespbv1.Lead{
		DisplayName: "Doe family stay", Contact: f.contact, Venue: f.venue, Kind: salespbv1.LeadKind_LEAD_KIND_STAY,
		Dates: []*typepbv1.DateRange{dateRange(2027, 11, 5, 7)}, Rooms: 2,
	}})
	if err != nil {
		t.Fatalf("create lead: %v", err)
	}
	if _, n, held := f.fb.HoldOf(l.GetHold()); l.GetHold() == "" || !held || n != 2 {
		t.Fatalf("lead hold %q: %d rooms, active %v; want 2 rooms held", l.GetHold(), n, held)
	}
	first := l.GetHold()

	// Lapsed in freebusy: the webhook's HoldLapsed forgets it.
	f.fb.LapseHold(first)
	if err := f.svc.HoldLapsed(ctx, first); err != nil {
		t.Fatalf("hold lapsed: %v", err)
	}
	if got, _ := f.repos.Leads.Get(ctx, l.GetName()); got.GetHold() != "" {
		t.Errorf("a lapsed hold is still on the lead: %q", got.GetHold())
	}

	// An edit takes it again; losing the lead lets it go.
	l, err = f.svc.UpdateLead(ctx, &salespbv1.UpdateLeadRequest{Lead: &salespbv1.Lead{Name: l.GetName(), Requirements: "sea view"}})
	if err != nil || l.GetHold() == "" || l.GetHold() == first {
		t.Fatalf("after an edit: hold %q, %v; want a new hold", l.GetHold(), err)
	}
	again := l.GetHold()
	l, err = f.svc.UpdateLead(ctx, &salespbv1.UpdateLeadRequest{Lead: &salespbv1.Lead{Name: l.GetName(), Stage: salespbv1.Stage_STAGE_LOST}})
	if err != nil {
		t.Fatalf("lose lead: %v", err)
	}
	if _, _, held := f.fb.HoldOf(again); held || l.GetHold() != "" {
		t.Errorf("a lost lead still holds %q (active %v)", l.GetHold(), held)
	}
}

func TestLiveSecondAcceptanceOfHallFails(t *testing.T) {
	f, ctx := holdSetup(t), context.Background()
	first, second := f.wedding(t, ctx), f.wedding(t, ctx)
	accepted, err := f.svc.AcceptQuote(ctx, &salespbv1.AcceptQuoteRequest{Name: first.GetName()})
	if err != nil {
		t.Fatalf("accept the first: %v", err)
	}
	if _, err := f.svc.AcceptQuote(ctx, &salespbv1.AcceptQuoteRequest{Name: second.GetName()}); err == nil {
		t.Fatal("a second quote for the same hall and date was accepted")
	}
	got, err := f.svc.GetQuote(ctx, &salespbv1.GetQuoteRequest{Name: second.GetName()})
	if err != nil || got.GetState() != salespbv1.Quote_STATE_SENT {
		t.Errorf("the refused quote is %v (%v); want still SENT", got.GetState(), err)
	}
	if n := f.fb.Active(); n != 1 {
		t.Errorf("%d claims in freebusy after the refused accept; want the first wedding's hall only", n)
	}

	// The wedding's session in the hall replaces the hold with its own.
	start := time.Date(2027, 12, 18, 12, 30, 0, 0, time.UTC)
	s, err := f.events.CreateSession(ctx, &eventpbv1.CreateSessionRequest{Parent: accepted.GetBooking(), Session: &eventpbv1.Session{
		DisplayName: "Pheras", Space: f.hall, Guests: 200, StartTime: timestamppb.New(start), EndTime: timestamppb.New(start.Add(5 * time.Hour)),
	}})
	if err != nil {
		t.Fatalf("a session in the hall the event holds: %v", err)
	}
	if n := f.fb.Active(); n != 1 || s.GetFreebusyReservation() == "" {
		t.Errorf("%d claims after the session, which holds %q; want the session's alone", n, s.GetFreebusyReservation())
	}
}
