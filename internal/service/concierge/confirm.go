// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package concierge

// confirm.go is ConfirmStay: the held stay booked -- once the guest has said
// yes to it (ask.go) -- for the guest the hold
// names -- found by their phone, or added -- through the stay service,
// whose booking takes its rooms from the hold. The stay is PENDING until its
// advance is paid (finance confirms it then): ConfirmStay asks for it by a
// payment link on the stay's proforma (advance.go), and says so -- or, with
// no link to give, that the team will send the details. A retried
// ConfirmStay finds the stay, the proforma and the link the first one made.

import (
	"context"
	"fmt"

	"github.com/shokki-engineering/aperio/internal/booking"
	"github.com/shokki-engineering/aperio/internal/engine"
	"github.com/shokki-engineering/aperio/internal/service/generated/servicex"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/concierge/v1/conciergepbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/guest/v1/guestpbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/stay/v1/staypbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/type/v1/typepbv1"
	"google.golang.org/genproto/googleapis/type/date"
	"google.golang.org/genproto/googleapis/type/money"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ConfirmStay books the stay a hold token names.
func (s *Server) ConfirmStay(ctx context.Context, req *conciergepbv1.ConfirmStayRequest) (*conciergepbv1.ConfirmStayResponse, error) {
	return run(ctx, s, "ConfirmStay", func(ctx context.Context) (*conciergepbv1.ConfirmStayResponse, error) { return s.confirmStay(ctx, req) })
}

func (s *Server) confirmStay(ctx context.Context, req *conciergepbv1.ConfirmStayRequest) (*conciergepbv1.ConfirmStayResponse, error) {
	h, err := openToken(s.seal, req.GetHoldToken())
	if err != nil || h.Venue != req.GetVenue() {
		return nil, servicex.BadField("INVALID_HOLD_TOKEN", "hold_token", "the hold token is not one this venue gave",
			"Hold the stay again with HoldStay, and pass the token it returns")
	}
	if s.now().After(h.Expire) {
		return nil, servicex.Blocked("HOLD_EXPIRED", "aperio.booking.v1/Reservation", h.Venue+"/reservations/"+h.Reservation,
			"the hold lapsed before the stay was confirmed", "Hold the stay again; the rooms may have gone")
	}
	arrive := &date.Date{Year: h.Arrival[0], Month: h.Arrival[1], Day: h.Arrival[2]}
	total := &money.Money{CurrencyCode: h.Currency, Units: h.Total, Nanos: h.TotalNanos}
	if !req.GetConfirmed() {
		room := h.Space
		if sp, err := s.spaces.Get(ctx, h.Space); err == nil {
			room = sp.GetDisplayName()
		}
		return nil, askConsent(fmt.Sprintf("Book %s of %s for %s from %s for %s, %s plus taxes?",
			count(h.Rooms, "room"), room, count(h.Nights, "night"), day(arrive), h.Guest, amount(total)))
	}
	contact, err := s.guest(ctx, h)
	if err != nil {
		return nil, err
	}
	r := &staypbv1.Reservation{
		Contact: contact, Channel: channel(h.Channel), Adults: h.Adults, Children: h.Children,
		ArrivalDate: arrive, DepartureDate: engine.AddDays(arrive, int(h.Nights)),
		Rooms: []*staypbv1.RoomLine{{Space: h.Space, Rooms: h.Rooms}},
	}
	if h.Requests != "" {
		r.SpecialRequests = []*typepbv1.SpecialRequest{{Summary: h.Requests}}
	}
	name := h.Venue + "/reservations/" + h.Reservation
	out, err := s.booker.CreateReservation(ctx, &staypbv1.CreateReservationRequest{Parent: h.Venue, ReservationId: h.Reservation, Reservation: r})
	if status.Code(err) == codes.AlreadyExists {
		out, err = s.booker.GetReservation(ctx, &staypbv1.GetReservationRequest{Name: name})
	}
	if err != nil {
		return nil, err
	}
	url := s.requestAdvance(ctx, h, contact, out.GetName(), total)
	pay := fmt.Sprintf("Our team will send you the payment details at %s", h.Phone)
	switch {
	case url != "" && s.sender != nil:
		pay = fmt.Sprintf("I've sent a link to pay the advance to your phone, %s", h.Phone)
	case url != "":
		pay = "You can pay the advance on the payment link"
	}
	return &conciergepbv1.ConfirmStayResponse{
		Reservation: out.GetName(), Total: total, Reference: booking.Reference(out.GetName()), PaymentUrl: url,
		Spoken: fmt.Sprintf("Done, %s: %s from %s is booked, %s plus taxes; your reference is %s. %s; "+
			"the booking is confirmed once the advance is paid.",
			h.Guest, count(h.Nights, "night"), day(arrive), amount(total), spell(booking.Reference(out.GetName())), pay),
	}, nil
}

// guest is the contact the stay is for: the one whose phone numbers hold the
// hold's phone (findGuest), else one added with the hold's name, phone and
// email.
func (s *Server) guest(ctx context.Context, h held) (string, error) {
	if name, err := s.findGuest(ctx, h.Phone); err != nil || name != "" {
		return name, err
	}
	c := &guestpbv1.Contact{
		Kind: guestpbv1.ContactKind_CONTACT_KIND_PERSON, DisplayName: h.Guest,
		Phones: []*typepbv1.Phone{{Number: h.Phone, Kind: typepbv1.PhoneKind_PHONE_KIND_MOBILE}},
	}
	if h.Email != "" {
		c.Emails = []string{h.Email}
	}
	made, err := s.guests.CreateContact(ctx, &guestpbv1.CreateContactRequest{Contact: c})
	if err != nil {
		return "", err
	}
	return made.GetName(), nil
}

// channel is where the guest asked from; the website when unsaid.
func channel(c int32) typepbv1.Channel {
	if c == 0 {
		return typepbv1.Channel_CHANNEL_WEBSITE
	}
	return typepbv1.Channel(c)
}
