// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package concierge_test

// Live tests of an event enquiry -- a lead, a draft quote of the cheapest
// menu that suits, the menus priced and the halls free -- and of a guest's
// request sent to its department. Run as test/live says.

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/catalog/v1/catalogpbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/concierge/v1/conciergepbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/operations/v1/operationspbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/sales/v1/salespbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/type/v1/typepbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/venue/v1/venuepbv1"
	"google.golang.org/genproto/googleapis/type/date"
	"google.golang.org/genproto/googleapis/type/decimal"
	"google.golang.org/genproto/googleapis/type/money"
)

func TestLiveEnquireEvent(t *testing.T) {
	r, ctx := newResort(t), context.Background()
	must := func(err error, what string) {
		t.Helper()
		if err != nil {
			t.Fatalf("%s: %v", what, err)
		}
	}
	_, err := r.venues.Spaces.Create(ctx, r.venue.GetName(), &venuepbv1.Space{DisplayName: "Grand Ballroom", Kind: venuepbv1.SpaceKind_SPACE_KIND_HALL,
		Resources: []string{"resources/ballroom-" + r.run}, Capacity: &venuepbv1.Capacity{Layouts: []*venuepbv1.LayoutCapacity{{Layout: venuepbv1.Layout_LAYOUT_BANQUET, Guests: 400}}}})
	must(err, "create hall")
	dish, err := r.catalog.CreateDish(ctx, &catalogpbv1.CreateDishRequest{Parent: r.venue.GetName(), Dish: &catalogpbv1.Dish{DisplayName: "Paneer tikka", Course: catalogpbv1.Course_COURSE_STARTER}})
	must(err, "create dish")
	_, err = r.catalog.CreateMenu(ctx, &catalogpbv1.CreateMenuRequest{Parent: r.venue.GetName(), Menu: &catalogpbv1.Menu{
		DisplayName: "Silver Vegetarian", Diets: []typepbv1.Diet{typepbv1.Diet_DIET_VEGETARIAN}, PlatePrice: &money.Money{CurrencyCode: "INR", Units: 1200},
		MinPlates: 100, Active: true, Tax: &typepbv1.Tax{HsnSacCode: "996334", GstRate: &decimal.Decimal{Value: "5"}},
		Courses: []*catalogpbv1.MenuCourse{{Course: catalogpbv1.Course_COURSE_STARTER, Dishes: []string{dish.GetName()}}},
	}})
	must(err, "create menu")

	day := &date.Date{Year: int32(time.Now().Year()) + 1, Month: 12, Day: 20}
	enq := &conciergepbv1.EnquireEventRequest{Venue: r.venue.GetName(), EventType: typepbv1.EventType_EVENT_TYPE_WEDDING, EventDate: day,
		Guests: 300, Diet: typepbv1.Diet_DIET_VEGETARIAN, GuestDisplayName: "John Doe", Phone: "97400" + r.run[len(r.run)-5:], RequestId: uuid.NewString()}
	got, err := r.svc.EnquireEvent(ctx, enq)
	must(err, "enquire")
	if len(got.GetMenus()) != 1 || got.GetMenus()[0].GetTotal().GetUnits() != 360000 || got.GetQuote() == "" {
		t.Fatalf("enquiry: menus %v, quote %q; want Silver Vegetarian at 3,60,000 rupees, quoted", got.GetMenus(), got.GetQuote())
	}
	if len(got.GetHalls()) != 1 || got.GetHalls()[0] != "Grand Ballroom" || !strings.Contains(got.GetSpoken(), "3,60,000 rupees") {
		t.Errorf("halls %v, spoken %q; want the ballroom, and the price said", got.GetHalls(), got.GetSpoken())
	}
	lead, err := r.sales.GetLead(ctx, &salespbv1.GetLeadRequest{Name: got.GetLead()})
	must(err, "get lead")
	if lead.GetStage() != salespbv1.Stage_STAGE_NEW || lead.GetGuests() != 300 || !strings.Contains(lead.GetRequirements(), "vegetarian") {
		t.Errorf("lead: %v, %d guests, %q", lead.GetStage(), lead.GetGuests(), lead.GetRequirements())
	}
	q, err := r.sales.GetQuote(ctx, &salespbv1.GetQuoteRequest{Name: got.GetQuote()})
	must(err, "get quote")
	if q.GetState() != salespbv1.Quote_STATE_DRAFT || q.GetLead() != lead.GetName() || q.GetSubtotal().GetUnits() != 360000 {
		t.Errorf("quote: %v for %q, subtotal %v", q.GetState(), q.GetLead(), q.GetSubtotal())
	}
	if again, err := r.svc.EnquireEvent(ctx, enq); err != nil || again.GetLead() != got.GetLead() || again.GetQuote() != got.GetQuote() {
		t.Errorf("a retried enquiry = %v, %v; want the same lead and quote", again, err)
	}

	enq.Diet, enq.RequestId = typepbv1.Diet_DIET_JAIN, uuid.NewString()
	jain, err := r.svc.EnquireEvent(ctx, enq)
	must(err, "jain enquiry")
	if len(jain.GetMenus()) != 0 || jain.GetQuote() != "" || jain.GetLead() == "" {
		t.Errorf("a Jain enquiry with no Jain menu: menus %v, quote %q, lead %q; want a lead only", jain.GetMenus(), jain.GetQuote(), jain.GetLead())
	}
}

func TestLiveRequestService(t *testing.T) {
	r, ctx := newResort(t), context.Background()
	req := &conciergepbv1.RequestServiceRequest{Venue: r.venue.GetName(), Kind: conciergepbv1.ServiceKind_SERVICE_KIND_DRINKS,
		What: "black coffee", Quantity: 2, Location: "Room 204", Notes: "no sugar", RequestId: uuid.NewString()}
	got, err := r.svc.RequestService(ctx, req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if got.GetSpoken() != "Done: 2 black coffee to Room 204. The pantry has it." {
		t.Errorf("spoken = %q", got.GetSpoken())
	}
	sr, err := r.ops.GetServiceRequest(ctx, &operationspbv1.GetServiceRequestRequest{Name: got.GetServiceRequest()})
	if err != nil {
		t.Fatalf("get request: %v", err)
	}
	if sr.GetDepartment() != operationspbv1.ServiceRequest_DEPARTMENT_PANTRY || sr.GetItems()[0].GetQuantity() != 2 || sr.GetLocation() != "Room 204" {
		t.Errorf("request: %v, %v, %q", sr.GetDepartment(), sr.GetItems(), sr.GetLocation())
	}
	if again, err := r.svc.RequestService(ctx, req); err != nil || again.GetServiceRequest() != got.GetServiceRequest() {
		t.Errorf("a retried request = %v, %v; want the same one", again, err)
	}
}
