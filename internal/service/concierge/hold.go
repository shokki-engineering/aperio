// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package concierge

// hold.go is HoldStay: rooms held in freebusy while the guest decides, and
// the token that books them.
//
// The stay's reservation id is fixed now, from the request id, and the hold
// is made under the very key the stay service books that reservation's
// rooms with (booking.RoomsKey). freebusy's holds are idempotent on their
// key, so when ConfirmStay books, the booking finds this hold and reserves
// from it: nothing is released between the two, and nobody else can take the
// rooms in the gap. A retried HoldStay holds the same rooms, not more.

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/shokki-engineering/aperio/internal/booking"
	"github.com/shokki-engineering/aperio/internal/engine"
	"github.com/shokki-engineering/aperio/internal/service/generated/servicex"
	"github.com/shokki-engineering/aperio/internal/service/guest"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/concierge/v1/conciergepbv1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// HoldStay holds rooms of a room type for the stay and prices them.
func (s *Server) HoldStay(ctx context.Context, req *conciergepbv1.HoldStayRequest) (*conciergepbv1.HoldStayResponse, error) {
	return run(ctx, s, "HoldStay", func(ctx context.Context) (*conciergepbv1.HoldStayResponse, error) { return s.holdStay(ctx, req) })
}

func (s *Server) holdStay(ctx context.Context, req *conciergepbv1.HoldStayRequest) (*conciergepbv1.HoldStayResponse, error) {
	if err := guestDetails(req.GetGuestDisplayName(), req.GetPhone()); err != nil {
		return nil, err
	}
	v, w, err := s.window(ctx, req.GetVenue(), req.GetArrivalDate(), req.GetNights())
	if err != nil {
		return nil, err
	}
	sp, err := s.spaces.Get(ctx, req.GetSpace())
	if err != nil || servicex.ParentOf(sp.GetName()) != v.GetName() {
		return nil, servicex.NotFoundAs(errors.Join(err, errNotHere), spaceType, req.GetSpace())
	}
	rooms := max(req.GetRooms(), 1)
	if !sleeps(sp, rooms, req.GetAdults(), req.GetChildren()) {
		return nil, servicex.BadField("TOO_MANY_GUESTS", "adults", fmt.Sprintf("%s of %s do not sleep the party",
			count(rooms, "room"), sp.GetDisplayName()), "Hold more rooms, or another room type")
	}
	total, err := s.price(ctx, v, sp.GetName(), req.GetArrivalDate(), req.GetNights(), rooms, req.GetAdults()+req.GetChildren())
	if err != nil {
		return nil, err
	}

	id := "stay-" + strings.ToLower(req.GetRequestId())
	h, err := s.engine.Hold(ctx, engine.Booking{
		Key: booking.RoomsKey(v.GetName()+"/reservations/"+id, 0), Resources: sp.GetResources(), Window: w, Quantity: rooms,
	})
	if errors.Is(err, engine.ErrUnavailable) {
		return nil, servicex.Blocked("NOT_AVAILABLE", spaceType, sp.GetName(),
			fmt.Sprintf("%s of %s are no longer free for those dates", count(rooms, "room"), sp.GetDisplayName()),
			"Find the stay again: another room type or other dates")
	}
	if err != nil {
		return nil, err
	}

	a := req.GetArrivalDate()
	token, err := sealToken(s.seal, held{
		Venue: v.GetName(), Reservation: id, Space: sp.GetName(),
		Arrival: [3]int32{a.GetYear(), a.GetMonth(), a.GetDay()}, Nights: req.GetNights(), Rooms: rooms,
		Adults: req.GetAdults(), Children: req.GetChildren(),
		Guest: strings.TrimSpace(req.GetGuestDisplayName()), Phone: phoneE164(req.GetPhone(), v.GetAddress().GetRegionCode()),
		Email: req.GetEmail(), Requests: req.GetSpecialRequests(), Channel: int32(req.GetChannel()),
		Total: total.GetUnits(), TotalNanos: total.GetNanos(), Currency: total.GetCurrencyCode(), Expire: h.Expire,
	})
	if err != nil {
		return nil, err
	}
	return &conciergepbv1.HoldStayResponse{
		HoldToken: token, ExpireTime: timestamppb.New(h.Expire), Total: total,
		Spoken: fmt.Sprintf("I've held %s of %s for %s from %s, for %s plus taxes, for the next %s. Shall I book it?",
			count(rooms, "room"), sp.GetDisplayName(), count(req.GetNights(), "night"), day(a), amount(total), minutesLeft(h.Expire, s.now())),
	}, nil
}

// errNotHere is a space of another venue, which this venue does not have.
var errNotHere = errors.New("concierge: not a space of the venue")

// guestDetails fails when there is no one to book for: the guest's name and
// phone, which an agent then asks for (and a form shows).
func guestDetails(name, phone string) error {
	switch {
	case strings.TrimSpace(name) == "":
		return servicex.BadField("GUEST_DETAILS_REQUIRED", "guest_display_name",
			"the booking needs the guest's name and phone number", "Ask the guest for their name, and their phone number if it is not known")
	case guest.Normalize(phone) == "":
		return servicex.BadField("GUEST_DETAILS_REQUIRED", "phone",
			"the booking needs the guest's phone number", "Ask the guest for their phone number")
	}
	return nil
}

// phoneE164 is number in E.164: as given when it has a country code, else
// with region's -- India's, for a ten-digit number at an Indian venue.
func phoneE164(number, region string) string {
	n := guest.Normalize(number)
	switch {
	case strings.HasPrefix(n, "+"):
		return n
	case region == "IN" && len(n) == 11 && n[0] == '0':
		return "+91" + n[1:]
	case region == "IN" && len(n) == 10:
		return "+91" + n
	}
	return n
}
