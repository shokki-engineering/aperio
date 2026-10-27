// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package operations_test

// fixture_live_test.go is what the state methods' live test runs over: a
// venue of its own, John Doe with a phone and a stay, a dish with its own
// GST, and the operations server calling the real finance, stay, event,
// catalog and guest servers.

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/shokki-engineering/aperio/internal/booking"
	"github.com/shokki-engineering/aperio/internal/database/repository/aperio/venue"
	"github.com/shokki-engineering/aperio/internal/service/event"
	"github.com/shokki-engineering/aperio/internal/service/finance"
	"github.com/shokki-engineering/aperio/internal/service/generated/aperio/catalog/v1/catalogpbv1svc"
	"github.com/shokki-engineering/aperio/internal/service/generated/aperio/operations/v1/operationspbv1svc"
	"github.com/shokki-engineering/aperio/internal/service/guest"
	"github.com/shokki-engineering/aperio/internal/service/operations"
	"github.com/shokki-engineering/aperio/internal/service/stay"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/catalog/v1/catalogpbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/event/v1/eventpbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/guest/v1/guestpbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/operations/v1/operationspbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/stay/v1/staypbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/type/v1/typepbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/venue/v1/venuepbv1"
	"github.com/shokki-engineering/aperio/test/live"
	"google.golang.org/genproto/googleapis/type/date"
	"google.golang.org/genproto/googleapis/type/decimal"
	"google.golang.org/genproto/googleapis/type/postaladdress"
)

// fixture is a venue with a guest, a stay and a dish, and the servers.
type fixture struct {
	svc   *operations.Server
	repos operationspbv1svc.OperationsServiceRepositories
	money *finance.Server
	must  func(error, string)

	venue, contact, phone, booking, dish string
}

// bookings are the stay and event servers, as operations.Bookings.
type bookings struct {
	*stay.Server
	events *event.Server
}

func (b bookings) GetEvent(ctx context.Context, r *eventpbv1.GetEventRequest) (*eventpbv1.Event, error) {
	return b.events.GetEvent(ctx, r)
}

func newFixture(t *testing.T) *fixture {
	ctx, conn := context.Background(), live.Conn(t)
	f := &fixture{repos: live.OperationsRepositories(conn), phone: "+919800000042"}
	f.must = func(err error, what string) {
		t.Helper()
		if err != nil {
			t.Fatalf("%s: %v", what, err)
		}
	}
	venues := venue.New(conn)
	v, err := venues.Venues.Create(ctx, &venuepbv1.Venue{DisplayName: fmt.Sprint("States Resort ", time.Now().UnixNano()),
		Kind: venuepbv1.VenueKind_VENUE_KIND_STAYS, TimeZone: "Asia/Kolkata", CurrencyCode: "INR",
		Address: &postaladdress.PostalAddress{RegionCode: "IN"}, Gst: &typepbv1.Gst{Gstin: "29ABCDE1234F1Z5", LegalDisplayName: "Test", GstStateCode: "29"}})
	f.must(err, "create venue")
	f.venue = v.GetName()

	guests := guest.New(live.GuestRepositories(conn))
	c, err := guests.CreateContact(ctx, &guestpbv1.CreateContactRequest{Contact: &guestpbv1.Contact{Kind: guestpbv1.ContactKind_CONTACT_KIND_PERSON,
		DisplayName: "John Doe", Phones: []*typepbv1.Phone{{Number: f.phone, Kind: typepbv1.PhoneKind_PHONE_KIND_WHATSAPP}}}})
	f.must(err, "create contact")
	f.contact = c.GetName()

	base := &booking.Base{Venues: venues.Venues, Spaces: venues.Spaces, Now: time.Now, Search: conn}
	stays := stay.New(live.StayRepositories(conn), base)
	sp, err := venues.Spaces.Create(ctx, f.venue, &venuepbv1.Space{DisplayName: "Deluxe", Kind: venuepbv1.SpaceKind_SPACE_KIND_ROOM_TYPE})
	f.must(err, "create space")
	r, err := live.StayRepositories(conn).Reservations.Create(ctx, f.venue, &staypbv1.Reservation{Contact: f.contact, Adults: 2,
		Rooms:       []*staypbv1.RoomLine{{Space: sp.GetName(), Rooms: 1}},
		ArrivalDate: &date.Date{Year: 2026, Month: 12, Day: 1}, DepartureDate: &date.Date{Year: 2026, Month: 12, Day: 3}})
	f.must(err, "create stay")
	f.booking = r.GetName()

	catalog := catalogpbv1svc.NewCatalogServiceServer(live.CatalogRepositories(conn), catalogpbv1svc.NopCatalogServiceHooks{})
	d, err := catalog.CreateDish(ctx, &catalogpbv1.CreateDishRequest{Parent: f.venue, Dish: &catalogpbv1.Dish{DisplayName: "Cake",
		Course: catalogpbv1.Course_COURSE_DESSERT, Tax: &typepbv1.Tax{HsnSacCode: "996334", GstRate: &decimal.Decimal{Value: "18"}}}})
	f.must(err, "create dish")
	f.dish = d.GetName()

	f.money = finance.New(live.FinanceRepositories(conn), venues.Venues, nil, nil)
	f.svc = operations.New(f.repos, venues.Venues)
	f.svc.CallServices(operations.Services{Finance: f.money, Bookings: bookings{Server: stays, events: event.New(live.EventRepositories(conn), base)},
		Dishes: catalog, Guests: guests})
	return f
}

// chargeAgain moves done request r back to IN_PROGRESS behind the API's back
// -- as if a completion had charged it and then failed to write its state --
// and completes it again: the charge must not be posted twice.
func (f *fixture) chargeAgain(t *testing.T, r *operationspbv1.ServiceRequest) {
	t.Helper()
	ctx := context.Background()
	cur, err := f.repos.ServiceRequests.Get(ctx, r.GetName())
	f.must(err, "read "+r.GetName())
	cur.State = operationspbv1.ServiceRequest_STATE_IN_PROGRESS
	_, err = f.repos.ServiceRequests.UpdateOutputOnly(ctx, cur, []string{"state"})
	f.must(err, "reopen "+r.GetName())
	_, err = f.svc.CompleteServiceRequest(ctx, &operationspbv1.CompleteServiceRequestRequest{Name: r.GetName()})
	f.must(err, "complete again "+r.GetName())
}
