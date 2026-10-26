// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package event serves aperio.event.v1 -- events at a venue and their
// sessions -- on the generated AIP server, with what the generated layer
// leaves to hooks and custom methods: each session's space checked against
// freebusy for its window and booked there once it is stored, with a note of
// who it is for on the hall's calendar; an event confirmed, or cancelled with
// every session's space given back. What it shares with the stay service is
// internal/booking's.
package event

import (
	"github.com/shokki-engineering/aperio/internal/booking"
	"github.com/shokki-engineering/aperio/internal/service/generated/aperio/event/v1/eventpbv1svc"
	"github.com/shokki-engineering/aperio/internal/service/generated/servicex"
)

// Server serves aperio.event.v1.EventService: the generated standard
// methods, with freebusy behind the sessions' (hooks), and the event state
// methods (events.go).
type Server struct {
	*eventpbv1svc.EventServiceServer
	h *hooks
}

// New returns the Server over repos, taking halls through base.
func New(repos eventpbv1svc.EventServiceRepositories, base *booking.Base, opts ...servicex.Option) *Server {
	h := &hooks{Base: base, repos: repos}
	return &Server{EventServiceServer: eventpbv1svc.NewEventServiceServer(repos, h, opts...), h: h}
}

// hooks is the business logic around the generated standard methods.
type hooks struct {
	eventpbv1svc.NopEventServiceHooks
	*booking.Base

	repos eventpbv1svc.EventServiceRepositories
}

// The resource types errors name.
const (
	eventType   = "aperio.event.v1/Event"
	sessionType = "aperio.event.v1/Session"
)
