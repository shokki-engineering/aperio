// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package concierge

// bookings.go is FindBooking and CancelBooking: a guest's stays, found by the
// phone they booked with and something only they would know -- the booking's
// reference, or its day of arrival -- since anyone can say a phone number.
// A booking's reference is the last six characters of its id, upper-cased:
// nothing is stored for it, and among one guest's stays it is unique enough.

import (
	"context"
	"fmt"
	"strings"

	"github.com/shokki-engineering/aperio/internal/service/generated/servicex"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/concierge/v1/conciergepbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/guest/v1/guestpbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/stay/v1/staypbv1"
	"google.golang.org/protobuf/proto"
)

// FindBooking finds a guest's stays by phone and reference or arrival date.
func (s *Server) FindBooking(ctx context.Context, req *conciergepbv1.FindBookingRequest) (*conciergepbv1.FindBookingResponse, error) {
	return run(ctx, s, "FindBooking", func(ctx context.Context) (*conciergepbv1.FindBookingResponse, error) {
		if req.GetReference() == "" && req.GetArrivalDate() == nil {
			return nil, servicex.BadField("REFERENCE_REQUIRED", "reference", "a booking is found by its reference or its day of arrival, not a phone alone",
				"Ask the guest for the booking's reference, or the day they arrive")
		}
		stays, err := s.stays(ctx, req.GetVenue(), req.GetPhone(), req.GetReference())
		if err != nil {
			return nil, err
		}
		out := &conciergepbv1.FindBookingResponse{}
		for _, r := range stays {
			if a := req.GetArrivalDate(); a != nil && !proto.Equal(a, r.GetArrivalDate()) {
				continue
			}
			out.Bookings = append(out.Bookings, summary(r))
		}
		switch len(out.Bookings) {
		case 0:
			out.Spoken = "I couldn't find a booking with those details."
		case 1:
			out.Spoken = "I found it: " + out.Bookings[0].GetSpoken()
		default:
			out.Spoken = fmt.Sprintf("I found %d bookings.", len(out.Bookings))
		}
		return out, nil
	})
}

// CancelBooking cancels the guest's stay with the reference.
func (s *Server) CancelBooking(ctx context.Context, req *conciergepbv1.CancelBookingRequest) (*conciergepbv1.CancelBookingResponse, error) {
	return run(ctx, s, "CancelBooking", func(ctx context.Context) (*conciergepbv1.CancelBookingResponse, error) {
		stays, err := s.stays(ctx, req.GetVenue(), req.GetPhone(), req.GetReference())
		if err != nil {
			return nil, err
		}
		if len(stays) != 1 {
			return nil, servicex.BadField("BOOKING_NOT_FOUND", "reference", "no booking has that reference for that phone",
				"Check the reference with the guest, or find the booking by its day of arrival")
		}
		if !req.GetConfirmed() {
			return nil, askConsent(fmt.Sprintf("Cancel your stay from %s to %s, reference %s?",
				day(stays[0].GetArrivalDate()), day(stays[0].GetDepartureDate()), spell(reference(stays[0].GetName()))))
		}
		r, err := s.booker.CancelReservation(ctx, &staypbv1.CancelReservationRequest{
			Name: stays[0].GetName(), Reason: strings.TrimSpace("cancelled by the guest. " + req.GetReason()),
		})
		if err != nil {
			return nil, err
		}
		return &conciergepbv1.CancelBookingResponse{
			Reservation: r.GetName(),
			Spoken:      fmt.Sprintf("Your stay from %s, reference %s, is cancelled.", day(r.GetArrivalDate()), spell(reference(r.GetName()))),
		}, nil
	})
}

// stays are the stays at venueName of the guest with phone -- the one with
// reference, when given.
func (s *Server) stays(ctx context.Context, venueName, phone, ref string) ([]*staypbv1.Reservation, error) {
	v, err := s.venues.Get(ctx, venueName)
	if err != nil {
		return nil, servicex.NotFoundAs(err, venueType, venueName)
	}
	contact, err := s.findGuest(ctx, phoneE164(phone, v.GetAddress().GetRegionCode()))
	if err != nil || contact == "" {
		return nil, err
	}
	page, err := s.booker.ListReservations(ctx, &staypbv1.ListReservationsRequest{
		Parent: venueName, PageSize: 100, Filter: fmt.Sprintf("contact = %q", contact), OrderBy: "arrival_date desc",
	})
	if err != nil {
		return nil, err
	}
	var out []*staypbv1.Reservation
	for _, r := range page.GetReservations() {
		if ref == "" || strings.EqualFold(reference(r.GetName()), strings.TrimSpace(ref)) {
			out = append(out, r)
		}
	}
	return out, nil
}

// findGuest is the contact whose phone numbers hold phone; "" when none do.
func (s *Server) findGuest(ctx context.Context, phone string) (string, error) {
	found, err := s.guests.ListContacts(ctx, &guestpbv1.ListContactsRequest{PageSize: 1, Filter: fmt.Sprintf("phone_numbers:%q", phone)})
	if err != nil || len(found.GetContacts()) == 0 {
		return "", err
	}
	return found.GetContacts()[0].GetName(), nil
}

// summary is r as a guest is told it.
func summary(r *staypbv1.Reservation) *conciergepbv1.BookingSummary {
	state := strings.TrimPrefix(r.GetState().String(), "STATE_")
	return &conciergepbv1.BookingSummary{
		Reservation: r.GetName(), Reference: reference(r.GetName()), State: state,
		ArrivalDate: r.GetArrivalDate(), DepartureDate: r.GetDepartureDate(),
		Spoken: fmt.Sprintf("a stay from %s to %s, reference %s, %s.", day(r.GetArrivalDate()), day(r.GetDepartureDate()),
			spell(reference(r.GetName())), strings.ToLower(strings.ReplaceAll(state, "_", " "))),
	}
}

// reference is a stay's short reference: the last six letters and digits of
// its id, upper-cased.
func reference(name string) string {
	id := strings.ReplaceAll(servicex.IDOf(name), "-", "")
	return strings.ToUpper(id[max(0, len(id)-6):])
}

// spell is a reference to say: its characters one by one, "7 F 3 A 9 C".
func spell(ref string) string {
	return strings.Join(strings.Split(ref, ""), " ")
}
