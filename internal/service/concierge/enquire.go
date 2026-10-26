// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package concierge

// enquire.go is EnquireEvent: an event enquiry -- a wedding for 300 on 12
// December, vegetarian -- taken as a lead for the venue's sales team, with a
// draft quote of the cheapest menu that suits, the menus that suit priced for
// the day, and the halls that seat the party and are free. The quote is the
// team's to review and send; nothing is promised to the guest but a call
// back. The lead and the quote are named after the request id, so a retried
// enquiry finds them rather than making more.

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/shokki-engineering/aperio/internal/pricing"
	"github.com/shokki-engineering/aperio/internal/service/generated/servicex"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/catalog/v1/catalogpbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/concierge/v1/conciergepbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/pricing/v1/pricingpbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/sales/v1/salespbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/type/v1/typepbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/venue/v1/venuepbv1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// EnquireEvent takes an event enquiry.
func (s *Server) EnquireEvent(ctx context.Context, req *conciergepbv1.EnquireEventRequest) (*conciergepbv1.EnquireEventResponse, error) {
	return run(ctx, s, "EnquireEvent", func(ctx context.Context) (*conciergepbv1.EnquireEventResponse, error) {
		return s.enquireEvent(ctx, req)
	})
}

func (s *Server) enquireEvent(ctx context.Context, req *conciergepbv1.EnquireEventRequest) (*conciergepbv1.EnquireEventResponse, error) {
	if err := askGuest(req.GetGuestDisplayName(), req.GetPhone()); err != nil {
		return nil, err
	}
	v, err := s.venues.Get(ctx, req.GetVenue())
	if err != nil {
		return nil, servicex.NotFoundAs(err, venueType, req.GetVenue())
	}
	name := strings.TrimSpace(req.GetGuestDisplayName())
	contact, err := s.guest(ctx, held{Guest: name, Phone: phoneE164(req.GetPhone(), v.GetAddress().GetRegionCode()), Email: req.GetEmail()})
	if err != nil {
		return nil, err
	}
	occasion := strings.ToLower(strings.ReplaceAll(strings.TrimPrefix(req.GetEventType().String(), "EVENT_TYPE_"), "_", " "))
	dates := &typepbv1.DateRange{StartDate: req.GetEventDate(), EndDate: req.GetEventDate()}
	id := "enq-" + strings.ToLower(req.GetRequestId())
	lead, err := s.sales.CreateLead(ctx, &salespbv1.CreateLeadRequest{LeadId: id, Lead: &salespbv1.Lead{
		DisplayName: fmt.Sprintf("%s's %s", name, occasion), Contact: contact, Venue: v.GetName(),
		Kind: salespbv1.LeadKind_LEAD_KIND_EVENT, EventType: req.GetEventType(), Source: channel(int32(req.GetChannel())),
		Stage: salespbv1.Stage_STAGE_NEW, Dates: []*typepbv1.DateRange{dates}, Guests: req.GetGuests(),
		Requirements: strings.TrimSpace(req.GetNotes() + dietNote(req.GetDiet())),
	}})
	if status.Code(err) == codes.AlreadyExists {
		lead, err = s.sales.GetLead(ctx, &salespbv1.GetLeadRequest{Name: "leads/" + id})
	}
	if err != nil {
		return nil, err
	}
	out := &conciergepbv1.EnquireEventResponse{Lead: lead.GetName()}
	if out.Menus, err = s.menuOptions(ctx, v, req); err != nil {
		return nil, err
	}
	if len(out.Menus) > 0 {
		q, err := s.sales.CreateQuote(ctx, &salespbv1.CreateQuoteRequest{Parent: v.GetName(), QuoteId: id, Quote: &salespbv1.Quote{
			DisplayName: lead.GetDisplayName(), Contact: contact, Lead: lead.GetName(), EventType: req.GetEventType(),
			Dates: dates, Guests: req.GetGuests(),
			Lines: []*salespbv1.QuoteLine{{Kind: salespbv1.LineKind_LINE_KIND_MENU, Item: out.Menus[0].GetMenu(), Quantity: req.GetGuests()}},
		}})
		if status.Code(err) == codes.AlreadyExists {
			q, err = s.sales.GetQuote(ctx, &salespbv1.GetQuoteRequest{Name: v.GetName() + "/quotes/" + id})
		}
		if err != nil {
			return nil, err
		}
		out.Quote = q.GetName()
	}
	if out.Halls, err = s.freeHalls(ctx, v, req); err != nil {
		return nil, err
	}
	out.Spoken = enquirySpoken(name, occasion, req, out)
	return out, nil
}

// menuOptions are the active menus that suit the diet and are sold for the
// guests, priced for the day, cheapest first; at most three.
func (s *Server) menuOptions(ctx context.Context, v *venuepbv1.Venue, req *conciergepbv1.EnquireEventRequest) ([]*conciergepbv1.MenuOption, error) {
	menus, err := s.menus.ListMenus(ctx, &catalogpbv1.ListMenusRequest{Parent: v.GetName(), PageSize: 100})
	if err != nil {
		return nil, err
	}
	var out []*conciergepbv1.MenuOption
	for _, m := range menus.GetMenus() {
		diet := req.GetDiet() != typepbv1.Diet_DIET_UNSPECIFIED
		if !m.GetActive() || (diet && !slices.Contains(m.GetDiets(), req.GetDiet())) || m.GetMinPlates() > req.GetGuests() {
			continue
		}
		p, err := s.prices.CalculatePrice(ctx, &pricingpbv1.CalculatePriceRequest{
			Venue: v.GetName(), Item: m.GetName(), ServiceDate: req.GetEventDate(), Quantity: req.GetGuests(), Guests: req.GetGuests(),
		})
		if err != nil {
			continue // a menu with no price for the day is not offered
		}
		out = append(out, &conciergepbv1.MenuOption{
			Menu: m.GetName(), DisplayName: m.GetDisplayName(), PlatePrice: p.GetUnitPrice(), Total: p.GetAmount(),
			Spoken: fmt.Sprintf("%s is %s a plate, %s before taxes", m.GetDisplayName(), amount(p.GetUnitPrice()), amount(p.GetAmount())),
		})
	}
	slices.SortFunc(out, func(a, b *conciergepbv1.MenuOption) int {
		return pricing.Rat(a.GetTotal()).Cmp(pricing.Rat(b.GetTotal()))
	})
	return out[:min(len(out), shown)], nil
}

// enquirySpoken words the enquiry's answer.
func enquirySpoken(name, occasion string, req *conciergepbv1.EnquireEventRequest, out *conciergepbv1.EnquireEventResponse) string {
	said := []string{fmt.Sprintf("Thank you, %s. For your %s for %d guests on %s:", name, occasion, req.GetGuests(), day(req.GetEventDate()))}
	if len(out.GetMenus()) == 0 {
		said = append(said, "our team will put menus together for you.")
	}
	for _, m := range out.GetMenus() {
		said = append(said, m.GetSpoken()+".")
	}
	switch len(out.GetHalls()) {
	case 0:
		said = append(said, "No hall that seats everyone is free that day; the team will suggest options.")
	case 1:
		said = append(said, "The "+out.GetHalls()[0]+" is free that day.")
	default:
		said = append(said, "The "+strings.Join(out.GetHalls(), " and the ")+" are free that day.")
	}
	return strings.Join(append(said, "Our events team will call you with a quote."), " ")
}

// dietNote is the diet as the lead's requirements say it.
func dietNote(d typepbv1.Diet) string {
	if d == typepbv1.Diet_DIET_UNSPECIFIED {
		return ""
	}
	return " Diet: " + strings.ToLower(strings.TrimPrefix(d.String(), "DIET_")) + "."
}
