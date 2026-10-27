// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package freebusysync keeps aperio's bookings in step with freebusy when
// freebusy changes without aperio asking: an operator cancelling or moving a
// reservation (in freebusy's API, or over CalDAV), a hold lapsing, a link
// found broken. Two ways in find the change -- freebusy's signed webhook
// (Handler), at once, and the Reconciler, which walks the live bookings in
// case a delivery was missed -- and one Syncer settles it, through the stay,
// event and sales services' own methods, so their rules hold:
//
//   - a reservation cancelled in freebusy cancels the stay it backs
//     (CancelReservation), or the event when every session lost its hall
//     (CancelEvent); a stay or event that keeps other rooms or halls -- or a
//     guest already in -- is flagged instead, for a person to settle;
//   - a reservation moved in freebusy is followed -- the booking names the
//     replacement, so cancelling it in aperio frees the right one -- and
//     flagged, as aperio's dates and prices are not moved with it;
//   - one freebusy no longer has is booked again when the rooms are free,
//     and flagged either way; a window or count that differs is flagged;
//   - a lead's hold that lapsed or was released is forgotten.
//
// A flag is a line on the booking's notes, "[freebusy <reservation>] ...",
// written once however often the drift is found.
package freebusysync

import (
	"context"
	"fmt"
	"time"

	"github.com/shokki-engineering/aperio/internal/engine"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/event/v1/eventpbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/stay/v1/staypbv1"
)

// Stays is the stay service, as the Syncer uses it.
type Stays interface {
	ListReservations(context.Context, *staypbv1.ListReservationsRequest) (*staypbv1.ListReservationsResponse, error)
	CancelReservation(context.Context, *staypbv1.CancelReservationRequest) (*staypbv1.Reservation, error)
	Relink(ctx context.Context, name, from, to, note string) (*staypbv1.Reservation, error)
	Rebook(ctx context.Context, name string, line int, lost string) (*staypbv1.Reservation, error)
	Window(context.Context, *staypbv1.Reservation) (engine.Window, error)
}

// Events is the event service, as the Syncer uses it.
type Events interface {
	GetEvent(context.Context, *eventpbv1.GetEventRequest) (*eventpbv1.Event, error)
	ListSessions(context.Context, *eventpbv1.ListSessionsRequest) (*eventpbv1.ListSessionsResponse, error)
	CancelEvent(context.Context, *eventpbv1.CancelEventRequest) (*eventpbv1.Event, error)
	RelinkSession(ctx context.Context, name, from, to, note string) (*eventpbv1.Session, error)
	RebookSession(ctx context.Context, name, lost string) (*eventpbv1.Session, error)
	SessionWindow(*eventpbv1.Session) (engine.Window, error)
}

// Leads is the sales service, as the Syncer uses it.
type Leads interface {
	HoldLapsed(ctx context.Context, hold string) error
}

// Syncer settles what freebusy changed on aperio's bookings.
type Syncer struct {
	engine engine.Engine
	stays  Stays
	events Events
	leads  Leads
	now    func() time.Time
}

// New is a Syncer reading freebusy through eng and settling through the
// services.
func New(eng engine.Engine, stays Stays, events Events, leads Leads) *Syncer {
	return &Syncer{engine: eng, stays: stays, events: events, leads: leads, now: time.Now}
}

// Reservation settles whatever freebusy did to reservation on the bookings
// that name it: a stay, or a session.
func (s *Syncer) Reservation(ctx context.Context, reservation string) error {
	stays, err := s.stays.ListReservations(ctx, &staypbv1.ListReservationsRequest{
		Parent: "venues/-", PageSize: 10, Filter: "freebusy_reservations:" + quote(reservation),
	})
	if err != nil {
		return err
	}
	if err := s.settleStays(ctx, stays.GetReservations()); err != nil {
		return err
	}
	sessions, err := s.events.ListSessions(ctx, &eventpbv1.ListSessionsRequest{
		Parent: "venues/-/events/-", PageSize: 10, Filter: "freebusy_reservation = " + quote(reservation),
	})
	if err != nil {
		return err
	}
	return s.settleSessions(ctx, sessions.GetSessions())
}

// suspect is the error for a batch most of whose reservations freebusy does
// not know: aperio pointed at another freebusy (a restored database, the
// wrong endpoint) rather than links broken one by one. Nothing is repaired
// or flagged; a person looks.
func suspect(names []string, got map[string]engine.Reservation) error {
	missing := 0
	for _, n := range names {
		if _, ok := got[n]; !ok {
			missing++
		}
	}
	if len(names) < 10 || missing*2 <= len(names) {
		return nil
	}
	return fmt.Errorf("freebusysync: freebusy knows none of %d of %d reservations; is aperio pointed at the right freebusy? nothing was repaired", missing, len(names))
}

// HoldLapsed forgets hold on the lead that kept it.
func (s *Syncer) HoldLapsed(ctx context.Context, hold string) error {
	if s.leads == nil {
		return nil
	}
	return s.leads.HoldLapsed(ctx, hold)
}
