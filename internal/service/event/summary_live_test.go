// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package event_test

// A live test of SummarizeSessions: three days of a venue's sessions across
// two events, a cancelled event's left out, covers and plates by diet per
// session and per day, an empty day still listed, and sessions near midnight
// put on their day at the venue (Asia/Kolkata), not in UTC. Run as test/live
// says.

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/shokki-engineering/aperio/internal/database/repository/aperio/guest"
	"github.com/shokki-engineering/aperio/internal/database/repository/aperio/venue"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/catalog/v1/catalogpbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/event/v1/eventpbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/guest/v1/guestpbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/type/v1/typepbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/venue/v1/venuepbv1"
	"github.com/shokki-engineering/aperio/test/live"
	"google.golang.org/genproto/googleapis/type/date"
	"google.golang.org/genproto/googleapis/type/decimal"
	"google.golang.org/genproto/googleapis/type/money"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestLiveSummarizeSessions(t *testing.T) {
	f, ctx, conn := setup(t), context.Background(), live.Projected(live.Conn(t))
	hall, err := venue.New(conn).Spaces.Create(ctx, f.venue, &venuepbv1.Space{
		DisplayName: "Grand Ballroom", Kind: venuepbv1.SpaceKind_SPACE_KIND_HALL,
		Resources: []string{"resources/gb-1-" + f.run, "resources/gb-2-" + f.run},
	})
	if err != nil {
		t.Fatalf("create hall: %v", err)
	}
	host, err := guest.New(conn).Contacts.Create(ctx, &guestpbv1.Contact{Kind: guestpbv1.ContactKind_CONTACT_KIND_PERSON, DisplayName: "John Doe"})
	if err != nil {
		t.Fatalf("create contact: %v", err)
	}
	menus := live.CatalogRepositories(conn).Menus
	menu, err := menus.Create(ctx, f.venue, &catalogpbv1.Menu{
		DisplayName: "Silver Vegetarian", PlatePrice: &money.Money{CurrencyCode: "INR", Units: 1200}, MinPlates: 100,
		Tax:     &typepbv1.Tax{HsnSacCode: "996334", GstRate: &decimal.Decimal{Value: "5"}},
		Courses: []*catalogpbv1.MenuCourse{{Course: catalogpbv1.Course_COURSE_STARTER}},
	})
	if err != nil {
		t.Fatalf("create menu: %v", err)
	}
	f.svc.ReadMenusFrom(menus)

	ist, _ := time.LoadLocation("Asia/Kolkata")
	at := func(day, hour, minute int) *timestamppb.Timestamp {
		return timestamppb.New(time.Date(2026, 12, day, hour, minute, 0, 0, ist))
	}
	newEvent := func(title string) string {
		t.Helper()
		e, err := f.svc.CreateEvent(ctx, &eventpbv1.CreateEventRequest{Parent: f.venue, Event: &eventpbv1.Event{
			DisplayName: title, Contact: host.GetName(), EventType: typepbv1.EventType_EVENT_TYPE_WEDDING, Guests: 200,
			Dates: &typepbv1.DateRange{StartDate: &date.Date{Year: 2026, Month: 12, Day: 9}, EndDate: &date.Date{Year: 2026, Month: 12, Day: 13}},
		}})
		if err != nil {
			t.Fatalf("create event %s: %v", title, err)
		}
		return e.GetName()
	}
	session := func(event, title string, start *timestamppb.Timestamp, guests int32, diets ...*typepbv1.DietaryRequirement) {
		t.Helper()
		s := &eventpbv1.Session{
			DisplayName: title, Space: hall.GetName(), Guests: guests, GuaranteedGuests: guests - 10,
			StartTime: start, EndTime: timestamppb.New(start.AsTime().Add(3 * time.Hour)), Menu: menu.GetName(),
			DietaryRequirements: diets, SpecialRequests: []*typepbv1.SpecialRequest{{Summary: "Stage at the north end"}},
		}
		if _, err := f.svc.CreateSession(ctx, &eventpbv1.CreateSessionRequest{Parent: event, Session: s}); err != nil {
			t.Fatalf("create session %s: %v", title, err)
		}
	}
	diet := func(d typepbv1.Diet, guests int32, allergens ...typepbv1.Allergen) *typepbv1.DietaryRequirement {
		return &typepbv1.DietaryRequirement{Diet: d, Guests: guests, Allergens: allergens}
	}

	wedding, offsite, party := newEvent("Doe wedding"), newEvent("Doe offsite"), newEvent("Doe party")
	session(wedding, "Before", at(9, 23, 0), 20)                                      // 9 Dec: before the range
	session(offsite, "Keynote", at(10, 10, 0), 40, diet(typepbv1.Diet_DIET_JAIN, 10)) // 10 Dec
	session(party, "Party", at(10, 12, 0), 90)                                        // cancelled below
	session(wedding, "Sangeet", at(10, 19, 0), 200, diet(typepbv1.Diet_DIET_VEGETARIAN, 150),
		diet(typepbv1.Diet_DIET_JAIN, 30, typepbv1.Allergen_ALLERGEN_PEANUT))
	session(wedding, "Late supper", at(10, 23, 30), 30)                                      // 10 Dec here, 10 Dec UTC
	session(wedding, "Midnight", at(11, 0, 30), 50, diet(typepbv1.Diet_DIET_VEGETARIAN, 50)) // 11 Dec here, 10 Dec UTC
	session(wedding, "After", at(13, 0, 30), 60)                                             // 13 Dec here, 12 Dec UTC
	if _, err := f.svc.CancelEvent(ctx, &eventpbv1.CancelEventRequest{Name: party}); err != nil {
		t.Fatalf("cancel party: %v", err)
	}

	got, err := f.svc.SummarizeSessions(ctx, &eventpbv1.SummarizeSessionsRequest{
		Venue: f.venue, StartDate: &date.Date{Year: 2026, Month: 12, Day: 10}, Days: 3,
	})
	if err != nil {
		t.Fatalf("summarize: %v", err)
	}
	if len(got.GetDays()) != 3 {
		t.Fatalf("%d days; want 3", len(got.GetDays()))
	}
	titles := func(d *eventpbv1.SessionDay) string {
		var out []string
		for _, s := range d.GetSessions() {
			out = append(out, s.GetTitle())
		}
		return fmt.Sprint(out)
	}
	diets := func(cs []*eventpbv1.DietCount) string {
		var out []string
		for _, c := range cs {
			out = append(out, fmt.Sprintf("%d %s", c.GetGuests(), c.GetDiet()))
		}
		return fmt.Sprint(out)
	}

	d10, d11, d12 := got.GetDays()[0], got.GetDays()[1], got.GetDays()[2]
	if d10.GetDate().GetDay() != 10 || d11.GetDate().GetDay() != 11 || d12.GetDate().GetDay() != 12 {
		t.Errorf("dates %v, %v, %v; want 10, 11, 12 Dec", d10.GetDate(), d11.GetDate(), d12.GetDate())
	}
	if want := "[Keynote Sangeet Late supper]"; titles(d10) != want || d10.GetCovers() != 270 {
		t.Errorf("10 Dec: %s, %d covers; want %s, 270", titles(d10), d10.GetCovers(), want)
	}
	if want := "[150 vegetarian 40 jain 30 allergic to peanut]"; diets(d10.GetDiets()) != want {
		t.Errorf("10 Dec diets %s; want %s", diets(d10.GetDiets()), want)
	}
	if titles(d11) != "[Midnight]" || d11.GetCovers() != 50 || diets(d11.GetDiets()) != "[50 vegetarian]" {
		t.Errorf("11 Dec: %s, %d covers, %s; want Midnight, 50, 50 vegetarian", titles(d11), d11.GetCovers(), diets(d11.GetDiets()))
	}
	if len(d12.GetSessions()) != 0 || d12.GetCovers() != 0 {
		t.Errorf("12 Dec: %s, %d covers; want none", titles(d12), d12.GetCovers())
	}

	s := d10.GetSessions()[1]
	if s.GetEvent() != "Doe wedding" || s.GetHost() != "John Doe" || s.GetSpace() != "Grand Ballroom" ||
		s.GetMenu() != "Silver Vegetarian" || s.GetGuests() != 200 || s.GetGuaranteedGuests() != 190 ||
		s.GetState() != eventpbv1.Event_STATE_TENTATIVE || fmt.Sprint(s.GetRequests()) != "[Stage at the north end]" ||
		!s.GetStartTime().AsTime().Equal(at(10, 19, 0).AsTime()) {
		t.Errorf("Sangeet as the kitchen reads it: %+v", s)
	}
	if want := "[150 vegetarian 30 allergic to peanut 30 jain]"; diets(s.GetDiets()) != want {
		t.Errorf("Sangeet diets %s; want %s", diets(s.GetDiets()), want)
	}
}
