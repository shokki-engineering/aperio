// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package sales

// attempts.go names the bookings an accept makes so that a retry is safe.
// The first attempt names them quote-{id}; a retry asking for that name
// gets AlreadyExists and carries on with what the failed attempt made --
// unless that attempt called it off. A failed accept cancels the event it
// made (its hall was another's, or its stay could not be booked), and a
// cancelled event has given its halls back: carrying on with it would hold
// halls for a cancelled event, under freebusy keys whose reservations are
// already cancelled. So a cancelled booking is passed over and the attempt
// starts over under the next name, quote-{id}-2, -3, ...: every name is
// still the quote's own, so retries find each other's bookings, and the
// freebusy keys are new, so nothing is booked twice.

import (
	"context"
	"fmt"
	"strings"

	"github.com/shokki-engineering/aperio/internal/service/generated/servicex"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/event/v1/eventpbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/sales/v1/salespbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/stay/v1/staypbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/type/v1/typepbv1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// maxAttempts is how many times an accept starts over before it gives up.
const maxAttempts = 20

// bookingID is the id attempt n (from 1) of accepting q names its bookings.
func bookingID(q *salespbv1.Quote, n int) string {
	id := "quote-" + strings.ToLower(servicex.IDOf(q.GetName()))
	if n > 1 {
		id = fmt.Sprintf("%s-%d", id, n)
	}
	return id
}

// tooManyAttempts is the error for a quote whose every name was called off.
func tooManyAttempts(q *salespbv1.Quote) error {
	return servicex.Blocked("TOO_MANY_ATTEMPTS", quoteType, q.GetName(),
		fmt.Sprintf("%d attempts to accept the quote were called off", maxAttempts),
		"Revise the quote with ReviseQuote, send it again and accept the revision")
}

// event makes q's event, or finds the live one a failed attempt made.
func (s *Server) event(ctx context.Context, venueName string, q *salespbv1.Quote) (*eventpbv1.Event, error) {
	if q.GetEventType() == typepbv1.EventType_EVENT_TYPE_UNSPECIFIED {
		return nil, servicex.BadField("MISSING_EVENT_TYPE", "quote.event_type", "an event quote has no event type", "Set the occasion, revise and send it again")
	}
	for n := 1; n <= maxAttempts; n++ {
		id := bookingID(q, n)
		e, err := s.events.CreateEvent(ctx, &eventpbv1.CreateEventRequest{Parent: venueName, EventId: id, Event: &eventpbv1.Event{
			DisplayName: q.GetDisplayName(), Contact: q.GetContact(), Lead: q.GetLead(), EventType: q.GetEventType(),
			Dates: q.GetDates(), Guests: q.GetGuests(), Quote: q.GetName(),
		}})
		if status.Code(err) != codes.AlreadyExists {
			return e, err
		}
		if e, err = s.events.GetEvent(ctx, &eventpbv1.GetEventRequest{Name: venueName + "/events/" + id}); err != nil {
			return nil, err
		}
		if e.GetState() != eventpbv1.Event_STATE_CANCELLED {
			return e, nil
		}
	}
	return nil, tooManyAttempts(q)
}

// stay makes q's reservation of rooms, or finds the live one a failed
// attempt made. A room line's quantity is room-nights; the stay runs from
// the quote's first date to its last (the departure), a night at least, so
// a line's rooms are its quantity over the nights.
func (s *Server) stay(ctx context.Context, venueName string, q *salespbv1.Quote, event string, rooms []*staypbv1.RoomLine) (*staypbv1.Reservation, error) {
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
	for n := 1; n <= maxAttempts; n++ {
		id := bookingID(q, n)
		r, err := s.stays.CreateReservation(ctx, &staypbv1.CreateReservationRequest{Parent: venueName, ReservationId: id,
			Reservation: &staypbv1.Reservation{
				Contact: q.GetContact(), Lead: q.GetLead(), Event: event, Channel: s.channel(ctx, q.GetLead()),
				ArrivalDate: arrive, DepartureDate: depart, Adults: max(q.GetGuests(), 1), Rooms: rooms, Quote: q.GetName(),
			}})
		if status.Code(err) != codes.AlreadyExists {
			return r, err
		}
		if r, err = s.stays.GetReservation(ctx, &staypbv1.GetReservationRequest{Name: venueName + "/reservations/" + id}); err != nil {
			return nil, err
		}
		if r.GetState() != staypbv1.Reservation_STATE_CANCELLED {
			return r, nil
		}
	}
	return nil, tooManyAttempts(q)
}
