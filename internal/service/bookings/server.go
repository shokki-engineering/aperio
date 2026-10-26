// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package bookings serves aperio.booking.v1: stays, events and their sessions,
// on the generated AIP server (protoc-gen-service), with the business logic
// the generated layer leaves to hooks and custom methods -- taking the rooms
// and spaces from freebusy, and moving a booking through its states.
//
// A stay's rooms and a session's space are freebusy's: a reservation or a
// session is checked against freebusy before it is stored (no side effects,
// so validate_only and a full date fail the same way), booked in freebusy once
// it is stored and has its name, and its freebusy reservations recorded on it,
// with a note of who it is for on the booked rooms' calendars (notes.go).
// Anything that fails after freebusy was asked gives back what was taken and
// removes the row, so aperio and freebusy never disagree about a booking.
package bookings

import (
	"time"

	"github.com/shokki-engineering/aperio/internal/database/repository/aperio/crm"
	"github.com/shokki-engineering/aperio/internal/database/repository/aperio/venue"
	"github.com/shokki-engineering/aperio/internal/engine"
	"github.com/shokki-engineering/aperio/internal/service/generated/aperio/booking/v1/bookingpbv1svc"
	"github.com/shokki-engineering/aperio/internal/service/generated/servicex"
)

// Server serves aperio.booking.v1.BookingService: the generated standard
// methods, with freebusy behind them (hooks), and the state methods
// (reservations.go, events.go).
type Server struct {
	*bookingpbv1svc.BookingServiceServer
	h *hooks
}

// New returns the Server over the booking repositories, reading venues and
// spaces from venues and the contacts its calendar notes name from
// contacts, and taking rooms and spaces from eng.
func New(repos bookingpbv1svc.BookingServiceRepositories, venues venue.Repositories, contacts crm.ContactRepository,
	eng engine.Engine, opts ...servicex.Option) *Server {
	h := &hooks{
		engine:   eng,
		repos:    repos,
		venues:   venues.Venues,
		spaces:   venues.Spaces,
		contacts: contacts,
		now:      time.Now,
	}
	return &Server{
		BookingServiceServer: bookingpbv1svc.NewBookingServiceServer(repos, h, opts...),
		h:                    h,
	}
}

// hooks is the business logic around the generated standard methods.
type hooks struct {
	bookingpbv1svc.NopBookingServiceHooks

	engine   engine.Engine
	repos    bookingpbv1svc.BookingServiceRepositories
	venues   venue.VenueRepository
	spaces   venue.SpaceRepository
	contacts crm.ContactRepository
	now      func() time.Time
}

// The resource types errors name.
const (
	reservationType = "aperio.booking.v1/Reservation"
	eventType       = "aperio.booking.v1/Event"
	sessionType     = "aperio.booking.v1/Session"
	venueType       = "aperio.venue.v1/Venue"
	spaceType       = "aperio.venue.v1/Space"
)
