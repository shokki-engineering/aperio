// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package sales_test

// Live test of SummarizePipeline over the dev database: a venue's leads at
// every open stage, one won and one lost this week, an overdue and an
// upcoming follow-up, a lead nobody has called, a sent quote expiring in two
// days beside one expiring in ten -- and another venue's lead left out.

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/shokki-engineering/aperio/internal/database/repository/aperio/guest"
	"github.com/shokki-engineering/aperio/internal/database/repository/aperio/venue"
	"github.com/shokki-engineering/aperio/internal/service/sales"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/guest/v1/guestpbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/sales/v1/salespbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/type/v1/typepbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/venue/v1/venuepbv1"
	"github.com/shokki-engineering/aperio/test/live"
	"google.golang.org/genproto/googleapis/type/date"
	"google.golang.org/genproto/googleapis/type/decimal"
	"google.golang.org/genproto/googleapis/type/postaladdress"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestLiveSummarizePipeline(t *testing.T) {
	ctx, conn := context.Background(), live.Conn(t)
	venues, repos := venue.New(conn), live.SalesRepositories(conn)
	contacts := guest.New(conn).Contacts
	svc := sales.New(repos, venues.Venues, live.CatalogRepositories(conn).Menus, nil, nil, nil)
	svc.ReadContactsFrom(contacts)
	must := func(err error, what string) {
		t.Helper()
		if err != nil {
			t.Fatalf("%s: %v", what, err)
		}
	}
	hall := func(name string) *venuepbv1.Venue {
		v, err := venues.Venues.Create(ctx, &venuepbv1.Venue{DisplayName: name + " " + fmt.Sprint(time.Now().UnixNano()),
			Kind: venuepbv1.VenueKind_VENUE_KIND_EVENTS, TimeZone: "Asia/Kolkata", CurrencyCode: "INR",
			Address: &postaladdress.PostalAddress{RegionCode: "IN"}, Gst: &typepbv1.Gst{Gstin: "29ABCDE1234F1Z5", LegalDisplayName: "Test", GstStateCode: "29"}})
		must(err, "create venue")
		return v
	}
	v, other := hall("Pipeline Hall"), hall("Other Hall")
	c, err := contacts.Create(ctx, &guestpbv1.Contact{Kind: guestpbv1.ContactKind_CONTACT_KIND_PERSON, DisplayName: "John Doe", PhoneNumbers: []string{"+919800000001"}})
	must(err, "create contact")

	now := time.Now()
	lead := func(venueName, title string, stage salespbv1.Stage, followUp time.Duration) *salespbv1.Lead {
		l := &salespbv1.Lead{DisplayName: title, Contact: c.GetName(), Venue: venueName, Kind: salespbv1.LeadKind_LEAD_KIND_EVENT, Stage: stage}
		if followUp != 0 {
			l.NextFollowUpTime = timestamppb.New(now.Add(followUp))
		}
		l, err := svc.CreateLead(ctx, &salespbv1.CreateLeadRequest{Lead: l})
		must(err, "create lead "+title)
		return l
	}
	closeLead := func(title string, stage salespbv1.Stage, ago time.Duration) {
		l := lead(v.GetName(), title, salespbv1.Stage_STAGE_PROPOSAL, 0)
		l.Stage, l.CloseTime = stage, timestamppb.New(now.Add(-ago))
		_, err := repos.Leads.UpdateWithOutputOnly(ctx, l, []string{"stage"}, []string{"close_time"})
		must(err, "close "+title)
	}

	uncalled := lead(v.GetName(), "Doe wedding", salespbv1.Stage_STAGE_NEW, 0)
	overdue := lead(v.GetName(), "Doe reception", salespbv1.Stage_STAGE_CONTACTED, -2*24*time.Hour)
	upcoming := lead(v.GetName(), "Doe anniversary", salespbv1.Stage_STAGE_CONTACTED, 3*24*time.Hour)
	lead(v.GetName(), "Doe birthday", salespbv1.Stage_STAGE_SITE_VISIT, 30*24*time.Hour) // not due this week
	lead(v.GetName(), "Doe engagement", salespbv1.Stage_STAGE_NEGOTIATION, 0)
	closeLead("Doe offsite", salespbv1.Stage_STAGE_WON, 24*time.Hour)
	closeLead("Doe retreat", salespbv1.Stage_STAGE_LOST, 2*24*time.Hour)
	closeLead("Doe gala", salespbv1.Stage_STAGE_WON, 30*24*time.Hour) // won too long ago
	lead(other.GetName(), "Doe sangeet", salespbv1.Stage_STAGE_NEW, -24*time.Hour)

	day := &date.Date{Year: 2027, Month: 12, Day: 12}
	quote := func(title string, expires time.Duration) *salespbv1.Quote {
		q, err := svc.CreateQuote(ctx, &salespbv1.CreateQuoteRequest{Parent: v.GetName(), Quote: &salespbv1.Quote{
			DisplayName: title, Contact: c.GetName(), Guests: 100, Dates: &typepbv1.DateRange{StartDate: day, EndDate: day},
			Lines: []*salespbv1.QuoteLine{{Kind: salespbv1.LineKind_LINE_KIND_SERVICE, Description: "Hall", Quantity: 1, UnitPrice: inr(100000),
				Tax: &typepbv1.Tax{HsnSacCode: "997212", GstRate: &decimal.Decimal{Value: "18"}}}},
		}})
		must(err, "create quote "+title)
		q, err = svc.SendQuote(ctx, &salespbv1.SendQuoteRequest{Name: q.GetName(), Etag: q.GetEtag()})
		must(err, "send "+title)
		q.ExpireTime = timestamppb.New(now.Add(expires))
		q, err = repos.Quotes.UpdateOutputOnly(ctx, q, []string{"expire_time"})
		must(err, "expire "+title)
		return q
	}
	soon := quote("Doe wedding quote", 2*24*time.Hour)
	quote("Doe reception quote", 10*24*time.Hour)

	got, err := svc.SummarizePipeline(ctx, &salespbv1.SummarizePipelineRequest{Venue: v.GetName()})
	must(err, "summarize")

	want := []salespbv1.Stage{salespbv1.Stage_STAGE_NEW, salespbv1.Stage_STAGE_CONTACTED, salespbv1.Stage_STAGE_SITE_VISIT, salespbv1.Stage_STAGE_NEGOTIATION}
	counts := []int32{1, 2, 1, 1}
	if len(got.GetStages()) != len(want) {
		t.Fatalf("stages = %v; want NEW 1, CONTACTED 2, SITE_VISIT 1, NEGOTIATION 1", got.GetStages())
	}
	for i, sc := range got.GetStages() {
		if sc.GetStage() != want[i] || sc.GetLeads() != counts[i] {
			t.Errorf("stages[%d] = %v %d; want %v %d", i, sc.GetStage(), sc.GetLeads(), want[i], counts[i])
		}
	}
	if got.GetWon() != 1 || got.GetLost() != 1 {
		t.Errorf("won %d, lost %d; want 1 and 1", got.GetWon(), got.GetLost())
	}
	if f := got.GetFollowUps(); len(f) != 2 || f[0].GetLead() != overdue.GetName() || f[1].GetLead() != upcoming.GetName() {
		t.Errorf("follow-ups = %v; want the overdue reception, then the anniversary", f)
	} else if f[0].GetContact() != "John Doe" || f[0].GetPhone() != "+919800000001" {
		t.Errorf("follow-up contact = %q %q; want John Doe +919800000001", f[0].GetContact(), f[0].GetPhone())
	}
	if u := got.GetUncontacted(); len(u) != 1 || u[0].GetLead() != uncalled.GetName() {
		t.Errorf("uncontacted = %v; want only the wedding (not the other venue's)", u)
	}
	if e := got.GetExpiring(); len(e) != 1 || e[0].GetQuote() != soon.GetName() || e[0].GetContact() != "John Doe" || e[0].GetTotal().GetUnits() != 118000 {
		t.Errorf("expiring = %v; want only the wedding quote, John Doe's, ₹1,18,000", e)
	}
}
