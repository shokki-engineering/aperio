// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package sales

// accept.go is AcceptQuote: a sent quote the guest takes becomes bookings --
// an event for its menus, packages and spaces, a stay for its rooms, or both
// -- made through the booking service, so freebusy takes the rooms as it does
// for any stay. The lead it answers is won.
//
// The bookings are named after the quote (quote-{id}), so a retry finds
// what a failed attempt made instead of booking twice; the lead is won before
// the quote is accepted, so the quote's state is the last thing to change.
// They start PENDING / TENTATIVE: the advance payment confirms them.

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/shokki-engineering/aperio/internal/service/generated/servicex"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/booking/v1/bookingpbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/crm/v1/crmpbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/sales/v1/salespbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/type/v1/typepbv1"
	"google.golang.org/genproto/googleapis/type/date"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Booker makes the bookings an accepted quote becomes: the booking service
// (booking.Server), whose hooks take rooms and halls from freebusy.
type Booker interface {
	CreateEvent(context.Context, *bookingpbv1.CreateEventRequest) (*bookingpbv1.Event, error)
	GetEvent(context.Context, *bookingpbv1.GetEventRequest) (*bookingpbv1.Event, error)
	DeleteEvent(context.Context, *bookingpbv1.DeleteEventRequest) (*emptypb.Empty, error)
	CreateReservation(context.Context, *bookingpbv1.CreateReservationRequest) (*bookingpbv1.Reservation, error)
	GetReservation(context.Context, *bookingpbv1.GetReservationRequest) (*bookingpbv1.Reservation, error)
}

// AcceptQuote makes a sent quote's bookings, wins its lead, and accepts it.
func (s *Server) AcceptQuote(ctx context.Context, req *salespbv1.AcceptQuoteRequest) (*salespbv1.Quote, error) {
	return s.move(ctx, "AcceptQuote", req.GetName(), req.GetEtag(), []salespbv1.Quote_State{salespbv1.Quote_STATE_SENT},
		func(ctx context.Context, q *salespbv1.Quote) (*salespbv1.Quote, error) {
			if s.booker == nil {
				return nil, status.Error(codes.Unimplemented, "AcceptQuote: no booking service is configured")
			}
			now := s.h.now()
			if q.GetExpireTime() != nil && now.After(q.GetExpireTime().AsTime()) {
				return nil, servicex.Blocked("QUOTE_EXPIRED", quoteType, q.GetName(), "the offer lapsed", "Revise it with ReviseQuote and send it again")
			}
			booking, bookings, err := s.book(ctx, q)
			if err != nil {
				return nil, err
			}
			if err := s.win(ctx, q.GetLead(), bookings, now); err != nil {
				return nil, err
			}
			q.State, q.AcceptTime, q.Booking = salespbv1.Quote_STATE_ACCEPTED, timestamppb.New(now), booking
			return s.Repositories().Quotes.UpdateOutputOnly(ctx, q, []string{"state", "accept_time", "booking"})
		})
}

// book makes q's event and stay, returning the one the quote names -- the
// event when there is one -- and both.
func (s *Server) book(ctx context.Context, q *salespbv1.Quote) (string, []string, error) {
	venueName := servicex.ParentOf(q.GetName())
	id := "quote-" + strings.ToLower(servicex.IDOf(q.GetName()))
	var rooms []*bookingpbv1.RoomLine
	event := q.GetEventType() != typepbv1.EventType_EVENT_TYPE_UNSPECIFIED
	for _, l := range q.GetLines() {
		switch l.GetKind() {
		case salespbv1.LineKind_LINE_KIND_ROOM:
			rooms = append(rooms, &bookingpbv1.RoomLine{Space: l.GetItem(), Rooms: l.GetQuantity()})
		case salespbv1.LineKind_LINE_KIND_MENU, salespbv1.LineKind_LINE_KIND_FIXED_PACKAGE, salespbv1.LineKind_LINE_KIND_SPACE:
			event = true
		}
	}
	if !event && len(rooms) == 0 {
		return "", nil, servicex.Blocked("NOTHING_TO_BOOK", quoteType, q.GetName(), "the quote has no room, menu, package or space lines and no event type",
			"Add what the guest is booking, revise and send it again")
	}

	var eventName string
	if event {
		e, err := s.event(ctx, venueName, id, q)
		if err != nil {
			return "", nil, err
		}
		eventName = e.GetName()
	}
	if len(rooms) == 0 {
		return eventName, []string{eventName}, nil
	}
	r, err := s.stay(ctx, venueName, id, q, eventName, rooms)
	if err != nil {
		if eventName != "" { // a stay that cannot be had undoes the event made with it
			_, _ = s.booker.DeleteEvent(context.WithoutCancel(ctx), &bookingpbv1.DeleteEventRequest{Name: eventName, Force: true})
		}
		return "", nil, err
	}
	if eventName == "" {
		return r.GetName(), []string{r.GetName()}, nil
	}
	return eventName, []string{eventName, r.GetName()}, nil
}

// event makes q's event, or finds the one a failed attempt made.
func (s *Server) event(ctx context.Context, venueName, id string, q *salespbv1.Quote) (*bookingpbv1.Event, error) {
	if q.GetEventType() == typepbv1.EventType_EVENT_TYPE_UNSPECIFIED {
		return nil, servicex.BadField("MISSING_EVENT_TYPE", "quote.event_type", "an event quote has no event type", "Set the occasion, revise and send it again")
	}
	e, err := s.booker.CreateEvent(ctx, &bookingpbv1.CreateEventRequest{Parent: venueName, EventId: id, Event: &bookingpbv1.Event{
		DisplayName: q.GetDisplayName(), Contact: q.GetContact(), Lead: q.GetLead(), EventType: q.GetEventType(),
		Dates: q.GetDates(), Guests: q.GetGuests(), Quote: q.GetName(),
	}})
	if status.Code(err) == codes.AlreadyExists {
		return s.booker.GetEvent(ctx, &bookingpbv1.GetEventRequest{Name: venueName + "/events/" + id})
	}
	return e, err
}

// stay makes q's reservation of rooms, or finds the one a failed attempt
// made. A room line's quantity is room-nights; the stay runs from the
// quote's first date to its last (the departure), a night at least, so a
// line's rooms are its quantity over the nights.
func (s *Server) stay(ctx context.Context, venueName, id string, q *salespbv1.Quote, event string, rooms []*bookingpbv1.RoomLine) (*bookingpbv1.Reservation, error) {
	arrive, depart := q.GetDates().GetStartDate(), q.GetDates().GetEndDate()
	nights := int32(days(arrive, depart))
	if nights < 1 {
		depart, nights = addDays(arrive, 1), 1
	}
	for i, l := range rooms {
		if l.GetRooms()%nights != 0 {
			return nil, servicex.BadField("ROOM_NIGHTS", fmt.Sprintf("quote.lines[%d].quantity", i),
				fmt.Sprintf("%d room-nights is not a whole number of rooms for %d nights", l.GetRooms(), nights), "Quote rooms × nights")
		}
		l.Rooms /= nights
	}
	r, err := s.booker.CreateReservation(ctx, &bookingpbv1.CreateReservationRequest{Parent: venueName, ReservationId: id,
		Reservation: &bookingpbv1.Reservation{
			Contact: q.GetContact(), Lead: q.GetLead(), Event: event, Channel: s.channel(ctx, q.GetLead()),
			ArrivalDate: arrive, DepartureDate: depart, Adults: max(q.GetGuests(), 1), Rooms: rooms, Quote: q.GetName(),
		}})
	if status.Code(err) == codes.AlreadyExists {
		return s.booker.GetReservation(ctx, &bookingpbv1.GetReservationRequest{Name: venueName + "/reservations/" + id})
	}
	return r, err
}

// channel is where the lead came from; OTHER without one.
func (s *Server) channel(ctx context.Context, lead string) typepbv1.Channel {
	if l, err := s.leads.Get(ctx, lead); err == nil && l.GetSource() != typepbv1.Channel_CHANNEL_UNSPECIFIED {
		return l.GetSource()
	}
	return typepbv1.Channel_CHANNEL_OTHER
}

// win marks lead won, with the bookings it became.
func (s *Server) win(ctx context.Context, lead string, bookings []string, now time.Time) error {
	if lead == "" {
		return nil
	}
	l, err := s.leads.Get(ctx, lead)
	if err != nil {
		return servicex.NotFoundAs(err, "aperio.crm.v1/Lead", lead)
	}
	l.Stage, l.CloseTime = crmpbv1.Stage_STAGE_WON, timestamppb.New(now)
	for _, b := range bookings {
		if !slices.Contains(l.GetBookings(), b) {
			l.Bookings = append(l.Bookings, b)
		}
	}
	_, err = s.leads.UpdateWithOutputOnly(ctx, l, []string{"stage"}, []string{"bookings", "close_time"})
	return err
}

// days is the number of days from a to b.
func days(a, b *date.Date) int {
	ta := time.Date(int(a.GetYear()), time.Month(a.GetMonth()), int(a.GetDay()), 0, 0, 0, 0, time.UTC)
	tb := time.Date(int(b.GetYear()), time.Month(b.GetMonth()), int(b.GetDay()), 0, 0, 0, 0, time.UTC)
	return int(tb.Sub(ta).Hours() / 24)
}

// addDays is d moved n days.
func addDays(d *date.Date, n int) *date.Date {
	t := time.Date(int(d.GetYear()), time.Month(d.GetMonth()), int(d.GetDay())+n, 0, 0, 0, 0, time.UTC)
	return &date.Date{Year: int32(t.Year()), Month: int32(t.Month()), Day: int32(t.Day())}
}
