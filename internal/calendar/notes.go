// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package calendar writes what a booking shows on its rooms' and halls'
// calendars: the engine.Note the booking service puts on the freebusy
// calendar of every resource a booking takes. freebusy serves those
// calendars over CalDAV, and as read-only .ics feeds Google Calendar
// subscribes to ("From URL"); cmd/aperio-calendar prints each space's link.
//
// A stay is an all-day note from arrival to departure, a session a timed one
// from setup to teardown, each with its guest count, the guests' names, the
// contact's emails, phones and address, dietary needs and special requests.
package calendar

import (
	"fmt"
	"strings"
	"time"

	"github.com/shokki-engineering/aperio/internal/engine"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/booking/v1/bookingpbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/crm/v1/crmpbv1"
)

// Stay is the note for rooms of a reservation r, for contact c (nil when it
// could not be read), at where, in w -- the stay's window in the venue's zone.
func Stay(r *bookingpbv1.Reservation, c *crmpbv1.Contact, rooms int32, where string, w engine.Window) engine.Note {
	guests := fmt.Sprintf("%d adults", r.GetAdults())
	if r.GetChildren() > 0 {
		guests += fmt.Sprintf(", %d children", r.GetChildren())
	}
	lines := []string{"Guests: " + guests, fmt.Sprintf("Rooms: %d", rooms)}
	if names := guestNames(r.GetResGuests()); len(names) > 0 {
		lines = append(lines, "Guest names: "+strings.Join(names, ", "))
	}
	lines = append(lines, contactLines(c)...)
	lines = append(lines, dietLines(r.GetDietaryRequirements())...)
	for _, req := range r.GetSpecialRequests() {
		lines = append(lines, "Request: "+req.GetSummary())
	}
	if r.GetNotes() != "" {
		lines = append(lines, "Notes: "+r.GetNotes())
	}
	lines = append(lines, "Booking: "+r.GetName())
	return engine.Note{
		Summary:     fmt.Sprintf("%s · %s · %d rooms", nameOf(c, r.GetContact()), guests, rooms),
		Description: strings.Join(lines, "\n"),
		Location:    where,
		Window:      w,
		AllDay:      true,
	}
}

// Session is the note for session s of event ev, for contact c (nil when it
// could not be read), at where, in w -- setup to teardown, in the venue's
// zone.
func Session(ev *bookingpbv1.Event, s *bookingpbv1.Session, c *crmpbv1.Contact, where string, w engine.Window) engine.Note {
	at := func(t time.Time) string { return t.In(w.Start.Location()).Format("15:04") }
	lines := []string{
		fmt.Sprintf("Guests: %d (guaranteed %d)", s.GetGuests(), s.GetGuaranteedGuests()),
		"Event: " + ev.GetDisplayName() + " (" + trim(ev.GetEventType().String(), "EVENT_TYPE_") + ")",
		"Times: " + at(s.GetStartTime().AsTime()) + " to " + at(s.GetEndTime().AsTime()) +
			", set up from " + at(w.Start) + ", cleared by " + at(w.End),
	}
	lines = append(lines, contactLines(c)...)
	lines = append(lines, dietLines(s.GetDietaryRequirements())...)
	for _, req := range s.GetSpecialRequests() {
		lines = append(lines, "Request: "+req.GetSummary())
	}
	lines = append(lines, "Booking: "+s.GetName())
	return engine.Note{
		Summary:     fmt.Sprintf("%s · %s · %d guests", ev.GetDisplayName(), s.GetDisplayName(), s.GetGuests()),
		Description: strings.Join(lines, "\n"),
		Location:    where,
		Window:      w,
	}
}

// trim is an enum value's name without its prefix, e.g. WEDDING.
func trim(s, prefix string) string {
	if i := strings.Index(s, prefix); i >= 0 {
		return s[i+len(prefix):]
	}
	return s
}

// nameOf is the contact's display name, or its resource name when it could
// not be read.
func nameOf(c *crmpbv1.Contact, name string) string {
	if c.GetDisplayName() != "" {
		return c.GetDisplayName()
	}
	return name
}
