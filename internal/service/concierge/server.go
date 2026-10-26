// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package concierge serves aperio.concierge.v1: what a guest asks a venue
// for, as a guest asks it -- on the website, or on a call to a voice agent
// (docs/voice.md). Each method is a task over the resource services, called
// through them so their rules hold: prices from the pricing service, guests
// through the guest service, stays booked by the stay service, which takes
// the rooms in freebusy.
//
//   - FindStay (find.go) prices the room types free for the dates.
//   - HoldStay (hold.go) holds rooms in freebusy for the minutes a guest
//     takes to decide, and seals what was held in a token (token.go).
//   - ConfirmStay (confirm.go) books the hold for the guest the token names:
//     the booking takes its rooms from that very hold.
//
// Every reply carries a sentence to say (spoken.go).
package concierge

import (
	"context"
	"time"

	"github.com/shokki-engineering/aperio/internal/database/repository/aperio/venue"
	"github.com/shokki-engineering/aperio/internal/engine"
	"github.com/shokki-engineering/aperio/internal/service/generated/servicex"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/concierge/v1/conciergepbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/guest/v1/guestpbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/pricing/v1/pricingpbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/stay/v1/staypbv1"
)

// Pricer prices an item for a date: the pricing service.
type Pricer interface {
	CalculatePrice(context.Context, *pricingpbv1.CalculatePriceRequest) (*pricingpbv1.CalculatePriceResponse, error)
}

// Guests finds and adds the guests a stay is for: the guest service, whose
// hooks keep a contact's phone numbers searchable.
type Guests interface {
	ListContacts(context.Context, *guestpbv1.ListContactsRequest) (*guestpbv1.ListContactsResponse, error)
	CreateContact(context.Context, *guestpbv1.CreateContactRequest) (*guestpbv1.Contact, error)
}

// Booker books a stay: the stay service.
type Booker interface {
	CreateReservation(context.Context, *staypbv1.CreateReservationRequest) (*staypbv1.Reservation, error)
	GetReservation(context.Context, *staypbv1.GetReservationRequest) (*staypbv1.Reservation, error)
}

// Server serves aperio.concierge.v1.ConciergeService.
type Server struct {
	conciergepbv1.UnimplementedConciergeServiceServer

	venues venue.VenueRepository
	spaces venue.SpaceRepository
	engine engine.Engine
	prices Pricer
	guests Guests
	booker Booker
	seal   []byte // HMAC key of the hold tokens
	now    func() time.Time
	opts   servicex.Options
}

// New returns the Server, reading venues and their spaces from venues,
// checking and holding rooms through eng, and sealing hold tokens with
// holdSecret.
func New(venues venue.Repositories, eng engine.Engine, prices Pricer, guests Guests, booker Booker, holdSecret []byte, opts ...servicex.Option) *Server {
	return &Server{
		venues: venues.Venues, spaces: venues.Spaces, engine: eng,
		prices: prices, guests: guests, booker: booker, seal: holdSecret, now: time.Now,
		opts: servicex.Apply(servicex.Options{Translator: servicex.NewTranslator("aperio.dev")}, opts),
	}
}

// run runs one method as the generated servers do theirs, so its errors
// render as theirs do: a status with the reason, the field and the fix.
func run[T any](ctx context.Context, s *Server, method string, fn func(context.Context) (T, error)) (T, error) {
	var out T
	err := s.opts.Run(ctx, "ConciergeService", method, func(ctx context.Context) (err error) {
		out, err = fn(ctx)
		return err
	})
	return out, err
}

// The resource types errors name.
const (
	venueType = "aperio.venue.v1/Venue"
	spaceType = "aperio.venue.v1/Space"
)
