// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package freebusysync_test

// fixture_live_test.go is what the live tests of freebusysync share: a venue
// with two rooms and a hall, John Doe, the stay and event services over the
// dev database with freebusy faked (test/live.Freebusy), a Syncer over them,
// and deliveries signed as freebusy signs them.
//
//	APERIO_TEST_DATABASE_URL="host=127.0.0.1 port=5433 user=postgres password=postgrespassword dbname=aperio sslmode=disable" \
//	  go test ./internal/freebusysync/ -v

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/shokki-engineering/aperio/internal/booking"
	"github.com/shokki-engineering/aperio/internal/database/repository/aperio/guest"
	"github.com/shokki-engineering/aperio/internal/database/repository/aperio/venue"
	"github.com/shokki-engineering/aperio/internal/freebusysync"
	"github.com/shokki-engineering/aperio/internal/service/event"
	"github.com/shokki-engineering/aperio/internal/service/generated/aperio/event/v1/eventpbv1svc"
	"github.com/shokki-engineering/aperio/internal/service/stay"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/guest/v1/guestpbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/stay/v1/staypbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/type/v1/typepbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/venue/v1/venuepbv1"
	"github.com/shokki-engineering/aperio/test/live"
	"google.golang.org/genproto/googleapis/type/date"
	"google.golang.org/genproto/googleapis/type/postaladdress"
)

// secret is the test webhook's signing secret.
var secret = "whsec_" + base64.StdEncoding.EncodeToString([]byte("aperio-test-webhook-signing-key!"))

type fixture struct {
	fb      *live.Freebusy
	stays   *stay.Server
	events  *event.Server
	sync    *freebusysync.Syncer
	venue   string
	rooms   string // two rooms
	hall    string
	contact string
	run     string
	// base and repos are what events was built from, for a test building
	// another event service over them.
	base  *booking.Base
	repos eventpbv1svc.EventServiceRepositories
}

func setup(t *testing.T) *fixture {
	t.Helper()
	ctx, conn := context.Background(), live.Projected(live.Conn(t))
	run := fmt.Sprint(time.Now().UnixNano())
	venues, people := venue.New(conn), guest.New(conn)
	v, err := venues.Venues.Create(ctx, &venuepbv1.Venue{
		DisplayName: "Test Resort " + run, Kind: venuepbv1.VenueKind_VENUE_KIND_STAYS_AND_EVENTS,
		TimeZone: "Asia/Kolkata", CurrencyCode: "INR", Address: &postaladdress.PostalAddress{RegionCode: "IN"},
		Gst: &typepbv1.Gst{Gstin: "29ABCDE1234F1Z5", LegalDisplayName: "Test Hospitality", GstStateCode: "29"},
	})
	if err != nil {
		t.Fatalf("create venue: %v", err)
	}
	rooms, err := venues.Spaces.Create(ctx, v.GetName(), &venuepbv1.Space{DisplayName: "Sea View", Kind: venuepbv1.SpaceKind_SPACE_KIND_ROOM_TYPE,
		Resources: []string{"resources/sv-1-" + run, "resources/sv-2-" + run}})
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
	base.Search = conn
	repos := live.EventRepositories(conn)
	f := &fixture{fb: fb, venue: v.GetName(), rooms: rooms.GetName(), hall: hall.GetName(), contact: c.GetName(), run: run,
		stays: stay.New(live.StayRepositories(conn), base), events: event.New(repos, base), base: base, repos: repos}
	f.sync = freebusysync.New(fb, f.stays, f.events, nil)
	return f
}

// stay books n rooms for John Doe, 10-12 Dec 2027.
func (f *fixture) stay(t *testing.T, id string, n int32) *staypbv1.Reservation {
	t.Helper()
	r, err := f.stays.CreateReservation(context.Background(), &staypbv1.CreateReservationRequest{
		Parent: f.venue, ReservationId: id + "-" + f.run, Reservation: &staypbv1.Reservation{
			Contact: f.contact, Channel: typepbv1.Channel_CHANNEL_WEBSITE, Adults: 2,
			ArrivalDate:   &date.Date{Year: 2027, Month: 12, Day: 10},
			DepartureDate: &date.Date{Year: 2027, Month: 12, Day: 12},
			Rooms:         []*staypbv1.RoomLine{{Space: f.rooms, Rooms: n}},
		}})
	if err != nil {
		t.Fatalf("create stay: %v", err)
	}
	return r
}

// get reads stay name.
func (f *fixture) get(t *testing.T, name string) *staypbv1.Reservation {
	t.Helper()
	r, err := f.stays.GetReservation(context.Background(), &staypbv1.GetReservationRequest{Name: name})
	if err != nil {
		t.Fatalf("get %s: %v", name, err)
	}
	return r
}

// deliver POSTs a freebusy event of type about subject to h, signed with
// key, and returns the status h answered.
func deliver(h http.Handler, key, id, eventType, subject string) int {
	body := fmt.Sprintf(`{"specversion":"1.0","id":%q,"source":"//freebusy.shokki.engineering","type":%q,"subject":%q,`+
		`"time":%q,"datacontenttype":"application/json","data":{"name":%q}}`, id, eventType, subject, time.Now().UTC().Format(time.RFC3339), subject)
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	raw, _ := base64.StdEncoding.DecodeString(strings.TrimPrefix(key, "whsec_"))
	mac := hmac.New(sha256.New, raw)
	mac.Write([]byte(id + "." + ts + "." + body))
	req := httptest.NewRequest(http.MethodPost, "/webhooks/freebusy", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/cloudevents+json; charset=utf-8")
	req.Header.Set("webhook-id", id)
	req.Header.Set("webhook-timestamp", ts)
	req.Header.Set("webhook-signature", "v1,"+base64.StdEncoding.EncodeToString(mac.Sum(nil)))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return w.Code
}
