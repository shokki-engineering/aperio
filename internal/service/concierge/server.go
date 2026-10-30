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
//     the booking takes its rooms from that very hold; then it raises the
//     stay's proforma and asks for the advance by a payment link
//     (advance.go).
//   - FindBooking and CancelBooking (bookings.go) find and cancel a guest's
//     stay by their phone and its reference -- never by a phone alone.
//   - EnquireEvent (enquire.go) takes an event enquiry as a lead and a draft
//     quote, with the menus that suit and the halls free that day.
//   - RequestService (service.go) sends a guest's ask to the department
//     that does it.
//
// Every reply carries a sentence to say (spoken.go).
package concierge

import (
	"context"
	"time"

	"github.com/shokki-engineering/aperio/internal/database/repository/aperio/venue"
	"github.com/shokki-engineering/aperio/internal/engine"
	"github.com/shokki-engineering/aperio/internal/notify"
	"github.com/shokki-engineering/aperio/internal/service/generated/servicex"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/catalog/v1/catalogpbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/concierge/v1/conciergepbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/guest/v1/guestpbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/operations/v1/operationspbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/pricing/v1/pricingpbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/sales/v1/salespbv1"
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

// Booker books, finds and cancels stays: the stay service.
type Booker interface {
	CreateReservation(context.Context, *staypbv1.CreateReservationRequest) (*staypbv1.Reservation, error)
	GetReservation(context.Context, *staypbv1.GetReservationRequest) (*staypbv1.Reservation, error)
	ListReservations(context.Context, *staypbv1.ListReservationsRequest) (*staypbv1.ListReservationsResponse, error)
	CancelReservation(context.Context, *staypbv1.CancelReservationRequest) (*staypbv1.Reservation, error)
}

// Sales takes an enquiry as a lead and a draft quote: the sales service.
type Sales interface {
	CreateLead(context.Context, *salespbv1.CreateLeadRequest) (*salespbv1.Lead, error)
	GetLead(context.Context, *salespbv1.GetLeadRequest) (*salespbv1.Lead, error)
	CreateQuote(context.Context, *salespbv1.CreateQuoteRequest) (*salespbv1.Quote, error)
	GetQuote(context.Context, *salespbv1.GetQuoteRequest) (*salespbv1.Quote, error)
}

// Catalog lists what a venue sells: the catalog service.
type Catalog interface {
	ListMenus(context.Context, *catalogpbv1.ListMenusRequest) (*catalogpbv1.ListMenusResponse, error)
}

// Operations sends a guest's request to a department: the operations service.
type Operations interface {
	CreateServiceRequest(context.Context, *operationspbv1.CreateServiceRequestRequest) (*operationspbv1.ServiceRequest, error)
	GetServiceRequest(context.Context, *operationspbv1.GetServiceRequestRequest) (*operationspbv1.ServiceRequest, error)
}

// Services are the services the concierge works through.
type Services struct {
	Pricing    Pricer
	Guests     Guests
	Stays      Booker
	Sales      Sales
	Catalog    Catalog
	Operations Operations
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
	sales  Sales
	menus  Catalog
	ops    Operations
	seal   []byte // HMAC key of the hold tokens
	// biller raises a booked stay's proforma and asks for advancePercent of
	// it (advance.go); nil until BillWith, and then no advance is asked.
	biller         Biller
	advancePercent int
	sender         *notify.Sender // sends the payment link; nil sends nothing
	now            func() time.Time
	opts           servicex.Options
}

// New returns the Server, reading venues and their spaces from venues,
// checking and holding rooms through eng, working through svcs, and sealing
// hold tokens with holdSecret.
func New(venues venue.Repositories, eng engine.Engine, svcs Services, holdSecret []byte, opts ...servicex.Option) *Server {
	return &Server{
		venues: venues.Venues, spaces: venues.Spaces, engine: eng,
		prices: svcs.Pricing, guests: svcs.Guests, booker: svcs.Stays, sales: svcs.Sales, menus: svcs.Catalog, ops: svcs.Operations,
		seal: holdSecret, now: time.Now,
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
