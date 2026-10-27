// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package stay serves aperio.stay.v1 -- stays at a venue -- on the generated
// AIP server, with what the generated layer leaves to hooks and custom
// methods: a stay's rooms checked against freebusy before it is stored,
// booked in freebusy once it is, with a note of who it is for on the rooms'
// calendars, and its states moved by the custom methods. Anything that fails
// after freebusy was asked gives back what was taken and removes the row, so
// aperio and freebusy never disagree about a stay. What it shares with the
// event service is internal/booking's.
package stay

import (
	"sync"

	"github.com/shokki-engineering/aperio/internal/booking"
	"github.com/shokki-engineering/aperio/internal/search"
	"github.com/shokki-engineering/aperio/internal/service/generated/aperio/stay/v1/staypbv1svc"
	"github.com/shokki-engineering/aperio/internal/service/generated/servicex"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/stay/v1/staypbv1"
)

// Server serves aperio.stay.v1.StayService: the generated standard methods,
// with freebusy behind them (hooks), and the state methods (states.go).
type Server struct {
	*staypbv1svc.StayServiceServer
	h *hooks
}

// New returns the Server over repos, taking rooms through base.
func New(repos staypbv1svc.StayServiceRepositories, base *booking.Base, opts ...servicex.Option) *Server {
	h := &hooks{Base: base, repos: repos, nights: PostgresNights(base.Search)}
	return &Server{StayServiceServer: staypbv1svc.NewStayServiceServer(repos, h, opts...), h: h}
}

// ReadNightsFrom reads the stay read model through r -- its OpenSearch copy
// (OpenNights) -- rather than its table.
func (s *Server) ReadNightsFrom(r search.Reader[*staypbv1.StayNight]) { s.h.nights = r }

// hooks is the business logic around the generated standard methods.
type hooks struct {
	staypbv1svc.NopStayServiceHooks
	*booking.Base

	repos staypbv1svc.StayServiceRepositories
	// nights is the stay read model (nights.go): its table, or its copy.
	nights search.Reader[*staypbv1.StayNight]
	// notes are the calendar notes being written in the background
	// (notes.go noteLater).
	notes sync.WaitGroup
}

// reservationType is the resource type errors name.
const reservationType = "aperio.stay.v1/Reservation"
