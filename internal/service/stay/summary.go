// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package stay

// summary.go is SummarizeDay: a day at a venue as the front desk reads it --
// who arrives, who leaves, who stays the night -- and how many of its rooms
// are taken. One call answers "who's arriving today" for a person or an
// agent. It reads the read model (nights.go), not every reservation: the
// day's nights and the eve's last nights, each under the (venue, day) index,
// then their stays and guests in one read each.

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/shokki-engineering/aperio/internal/booking"
	"github.com/shokki-engineering/aperio/internal/database/gorm/filterx"
	"github.com/shokki-engineering/aperio/internal/database/repository/repox"
	"github.com/shokki-engineering/aperio/internal/service/generated/servicex"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/guest/v1/guestpbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/stay/v1/staypbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/type/v1/typepbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/venue/v1/venuepbv1"
	"google.golang.org/genproto/googleapis/type/date"
)

// SummarizeDay summarizes req.Date -- today, when unset -- at req.Venue.
func (s *Server) SummarizeDay(ctx context.Context, req *staypbv1.SummarizeDayRequest) (*staypbv1.SummarizeDayResponse, error) {
	var out *staypbv1.SummarizeDayResponse
	err := s.Options().Run(ctx, "StayService", "SummarizeDay", func(ctx context.Context) error {
		var err error
		out, err = s.h.summarizeDay(ctx, req)
		return err
	})
	return out, err
}

func (h *hooks) summarizeDay(ctx context.Context, req *staypbv1.SummarizeDayRequest) (*staypbv1.SummarizeDayResponse, error) {
	zone, err := h.Zone(ctx, req.GetVenue())
	if err != nil {
		return nil, err
	}
	day := h.Now().In(zone)
	if d := req.GetDate(); d != nil {
		day = time.Date(int(d.GetYear()), time.Month(d.GetMonth()), int(d.GetDay()), 0, 0, 0, 0, time.UTC)
	}
	tonight, err := h.nightsOn(ctx, req.GetVenue(), day.Format(time.DateOnly), false)
	if err != nil {
		return nil, err
	}
	leaving, err := h.nightsOn(ctx, req.GetVenue(), day.AddDate(0, 0, -1).Format(time.DateOnly), true)
	if err != nil {
		return nil, err
	}
	briefs, err := h.briefs(ctx, req.GetVenue(), append(tonight, leaving...))
	if err != nil {
		return nil, err
	}
	out := &staypbv1.SummarizeDayResponse{Date: &date.Date{Year: int32(day.Year()), Month: int32(day.Month()), Day: int32(day.Day())}}
	if out.RoomsTotal, err = h.roomCount(ctx, req.GetVenue()); err != nil {
		return nil, err
	}
	for _, n := range tonight {
		out.Staying = append(out.Staying, briefs[n.GetReservation()])
		out.RoomsOccupied += n.GetRooms()
		if n.GetFirstNight() {
			out.Arrivals = append(out.Arrivals, briefs[n.GetReservation()])
		}
	}
	for _, n := range leaving {
		out.Departures = append(out.Departures, briefs[n.GetReservation()])
	}
	return out, nil
}

// briefs are the stays of nights, by name, as the desk reads them: the stays
// read in one call, their guests in another, their rooms named from the
// venue's spaces.
func (h *hooks) briefs(ctx context.Context, venueName string, nights []*staypbv1.StayNight) (map[string]*staypbv1.StayBrief, error) {
	out := map[string]*staypbv1.StayBrief{}
	if len(nights) == 0 {
		return out, nil
	}
	var names, contacts []string
	for _, n := range nights {
		names, contacts = append(names, n.GetReservation()), append(contacts, n.GetContact())
	}
	stays, _, err := h.repos.Reservations.List(ctx, venueName, repox.ListInput{PageSize: int32(len(names)), Where: []filterx.Condition{filterx.In("name", names...)}})
	if err != nil {
		return nil, err
	}
	guests, _, err := h.Contacts.List(ctx, repox.ListInput{PageSize: int32(len(contacts)), Where: []filterx.Condition{filterx.In("name", contacts...)}})
	if err != nil {
		return nil, err
	}
	byName := map[string]*guestpbv1.Contact{}
	for _, c := range guests {
		byName[c.GetName()] = c
	}
	spaces, err := h.spaceNames(ctx, venueName)
	if err != nil {
		return nil, err
	}
	for _, r := range stays {
		out[r.GetName()] = brief(r, byName[r.GetContact()], spaces)
	}
	return out, nil
}

// spaceNames are the display names of venueName's spaces, by name.
func (h *hooks) spaceNames(ctx context.Context, venueName string) (map[string]string, error) {
	names := map[string]string{}
	err := h.eachSpace(ctx, venueName, func(sp *venuepbv1.Space) { names[sp.GetName()] = sp.GetDisplayName() })
	return names, err
}

// roomCount is how many rooms venueName has: the freebusy resources of its
// room types and units.
func (h *hooks) roomCount(ctx context.Context, venueName string) (int32, error) {
	var total int32
	err := h.eachSpace(ctx, venueName, func(sp *venuepbv1.Space) {
		if k := sp.GetKind(); k == venuepbv1.SpaceKind_SPACE_KIND_ROOM_TYPE || k == venuepbv1.SpaceKind_SPACE_KIND_UNIT {
			total += int32(len(sp.GetResources()))
		}
	})
	return total, err
}

// eachSpace calls f with every space of venueName.
func (h *hooks) eachSpace(ctx context.Context, venueName string, f func(*venuepbv1.Space)) error {
	in := repox.ListInput{PageSize: 500}
	for {
		page, next, err := h.Spaces.List(ctx, venueName, in)
		if err != nil {
			return err
		}
		for _, sp := range page {
			f(sp)
		}
		if next == "" {
			return nil
		}
		in.PageToken = next
	}
}

// brief is r as the desk reads it, its guest c (nil when unread), its rooms
// named from spaces.
func brief(r *staypbv1.Reservation, c *guestpbv1.Contact, spaces map[string]string) *staypbv1.StayBrief {
	b := &staypbv1.StayBrief{
		Reservation: r.GetName(), Reference: booking.Reference(r.GetName()), Guest: r.GetContact(),
		Nights: int32(dayOf(r.GetDepartureDate()).Sub(dayOf(r.GetArrivalDate())).Hours() / 24),
		Adults: r.GetAdults(), Children: r.GetChildren(), State: r.GetState(),
	}
	if c != nil {
		b.Guest = c.GetDisplayName()
		if len(c.GetPhoneNumbers()) > 0 {
			b.Phone = c.GetPhoneNumbers()[0]
		}
	}
	var said []string
	for _, line := range r.GetRooms() {
		name := spaces[line.GetSpace()]
		if name == "" {
			name = servicex.IDOf(line.GetSpace())
		}
		said = append(said, fmt.Sprintf("%d %s", line.GetRooms(), name))
	}
	b.Rooms = strings.Join(said, ", ")
	for _, sr := range r.GetSpecialRequests() {
		b.Requests = append(b.Requests, sr.GetSummary())
	}
	for _, dr := range r.GetDietaryRequirements() {
		b.Requests = append(b.Requests, dietSaid(dr.GetDiet().String(), dr.GetAllergens(), dr.GetNotes()))
	}
	return b
}

// dietSaid is a dietary requirement as the desk says it: "vegetarian,
// allergic to peanut, no onion".
func dietSaid(diet string, allergens []typepbv1.Allergen, notes string) string {
	var said []string
	if d := strings.ToLower(strings.ReplaceAll(strings.TrimPrefix(diet, "DIET_"), "_", " ")); d != "unspecified" {
		said = append(said, d)
	}
	for _, a := range allergens {
		said = append(said, "allergic to "+strings.ToLower(strings.ReplaceAll(strings.TrimPrefix(a.String(), "ALLERGEN_"), "_", " ")))
	}
	if notes = strings.TrimSpace(notes); notes != "" {
		said = append(said, notes)
	}
	return strings.Join(said, ", ")
}
