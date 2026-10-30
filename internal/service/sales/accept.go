// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package sales

// accept.go is AcceptQuote: a sent quote the guest takes becomes bookings --
// an event for its menus, packages and spaces, a stay for its rooms, or both
// -- made through the booking service, so freebusy takes the rooms as it does
// for any stay. The lead it answers is won.
//
// The bookings are named after the quote (quote-{id}), so a retry finds
// what a failed attempt made instead of booking twice -- unless that attempt
// called it off (its hall or stay could not be had): a cancelled booking is
// not carried on with, and the retry starts over under the next name
// (attempts.go). The lead is won before the quote is accepted, so the
// quote's state is the last thing to change; once it is, its draft tax
// invoice is raised (invoice.go).
// They start PENDING / TENTATIVE: the advance payment confirms them. The
// event holds the halls the quote sells for its days (holds.go), so a second
// quote for the same hall and date cannot be accepted; the lead's own hold
// is let go first, as the stay takes its rooms.

import (
	"context"
	"slices"
	"time"

	"github.com/shokki-engineering/aperio/internal/service/generated/servicex"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/event/v1/eventpbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/sales/v1/salespbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/stay/v1/staypbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/type/v1/typepbv1"
	"google.golang.org/genproto/googleapis/type/date"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// AcceptQuote makes a sent quote's bookings, wins its lead, accepts it, and
// raises its draft tax invoice.
func (s *Server) AcceptQuote(ctx context.Context, req *salespbv1.AcceptQuoteRequest) (*salespbv1.Quote, error) {
	accepted, err := s.move(ctx, "AcceptQuote", req.GetName(), req.GetEtag(), []salespbv1.Quote_State{salespbv1.Quote_STATE_SENT},
		func(ctx context.Context, q *salespbv1.Quote) (*salespbv1.Quote, error) {
			if s.events == nil || s.stays == nil {
				return nil, status.Error(codes.Unimplemented, "AcceptQuote: no booking service is configured")
			}
			now := s.h.now()
			if q.GetExpireTime() != nil && now.After(q.GetExpireTime().AsTime()) {
				return nil, servicex.Blocked("QUOTE_EXPIRED", quoteType, q.GetName(), "the offer lapsed", "Revise it with ReviseQuote and send it again")
			}
			lead := s.releaseLead(ctx, q.GetLead()) // its rooms are the stay's to take
			booking, bookings, err := s.book(ctx, q)
			if err != nil {
				s.holdAgain(ctx, lead)
				return nil, err
			}
			if err := s.win(ctx, q.GetLead(), bookings, now); err != nil {
				return nil, err
			}
			q.State, q.AcceptTime, q.Booking = salespbv1.Quote_STATE_ACCEPTED, timestamppb.New(now), booking
			return s.Repositories().Quotes.UpdateOutputOnly(ctx, q, []string{"state", "accept_time", "booking"})
		})
	if err == nil {
		s.raiseInvoice(ctx, accepted)
	}
	return accepted, err
}

// book makes q's event and stay, returning the one the quote names -- the
// event when there is one -- and both.
func (s *Server) book(ctx context.Context, q *salespbv1.Quote) (string, []string, error) {
	venueName := servicex.ParentOf(q.GetName())
	var rooms []*staypbv1.RoomLine
	event := q.GetEventType() != typepbv1.EventType_EVENT_TYPE_UNSPECIFIED
	for _, l := range q.GetLines() {
		switch l.GetKind() {
		case salespbv1.LineKind_LINE_KIND_ROOM:
			rooms = append(rooms, &staypbv1.RoomLine{Space: l.GetItem(), Rooms: l.GetQuantity()})
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
		e, err := s.event(ctx, venueName, q)
		if err != nil {
			return "", nil, err
		}
		eventName = e.GetName()
		if err := s.holdHalls(ctx, eventName, q); err != nil {
			// The hall is another event's: this one is called off.
			_, _ = s.events.CancelEvent(context.WithoutCancel(ctx), &eventpbv1.CancelEventRequest{Name: eventName, Reason: "its hall could not be held"})
			return "", nil, err
		}
	}
	if len(rooms) == 0 {
		return eventName, []string{eventName}, nil
	}
	r, err := s.stay(ctx, venueName, q, eventName, rooms)
	if err != nil {
		if eventName != "" { // a stay that cannot be had calls off the event made with it
			_, _ = s.events.CancelEvent(context.WithoutCancel(ctx), &eventpbv1.CancelEventRequest{Name: eventName, Reason: "its stay could not be booked"})
		}
		return "", nil, err
	}
	if eventName == "" {
		return r.GetName(), []string{r.GetName()}, nil
	}
	return eventName, []string{eventName, r.GetName()}, nil
}

// channel is where the lead came from; OTHER without one.
func (s *Server) channel(ctx context.Context, lead string) typepbv1.Channel {
	if l, err := s.h.repos.Leads.Get(ctx, lead); err == nil && l.GetSource() != typepbv1.Channel_CHANNEL_UNSPECIFIED {
		return l.GetSource()
	}
	return typepbv1.Channel_CHANNEL_OTHER
}

// win marks lead won, with the bookings it became.
func (s *Server) win(ctx context.Context, lead string, bookings []string, now time.Time) error {
	if lead == "" {
		return nil
	}
	l, err := s.h.repos.Leads.Get(ctx, lead)
	if err != nil {
		return servicex.NotFoundAs(err, "aperio.crm.v1/Lead", lead)
	}
	l.Stage, l.CloseTime = salespbv1.Stage_STAGE_WON, timestamppb.New(now)
	for _, b := range bookings {
		if !slices.Contains(l.GetBookings(), b) {
			l.Bookings = append(l.Bookings, b)
		}
	}
	_, err = s.h.repos.Leads.UpdateWithOutputOnly(ctx, l, []string{"stage"}, []string{"bookings", "close_time"})
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
