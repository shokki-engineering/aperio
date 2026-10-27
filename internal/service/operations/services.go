// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package operations

// services.go is the other services operations calls, through their gRPC
// methods so each one's rules hold: finance, to post a done request's charge
// to its booking's bill; the stay and event services, for whom a booking is
// for; the catalog, for a dish's GST; the guest service, for a guest's phone.

import (
	"context"
	"strings"

	"github.com/shokki-engineering/aperio/internal/service/generated/servicex"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/catalog/v1/catalogpbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/event/v1/eventpbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/finance/v1/financepbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/guest/v1/guestpbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/stay/v1/staypbv1"
)

// Services are the services a request is charged and its guest found
// through. Without Finance nothing is charged; without Guests no guest is
// told.
type Services struct {
	Finance  Finance
	Bookings Bookings
	Dishes   Dishes
	Guests   Guests
}

// Finance reads and writes a booking's invoices (the FinanceService).
type Finance interface {
	ListInvoices(ctx context.Context, req *financepbv1.ListInvoicesRequest) (*financepbv1.ListInvoicesResponse, error)
	CreateInvoice(ctx context.Context, req *financepbv1.CreateInvoiceRequest) (*financepbv1.Invoice, error)
	UpdateInvoice(ctx context.Context, req *financepbv1.UpdateInvoiceRequest) (*financepbv1.Invoice, error)
}

// Bookings reads stays and events (the StayService and the EventService).
type Bookings interface {
	GetReservation(ctx context.Context, req *staypbv1.GetReservationRequest) (*staypbv1.Reservation, error)
	GetEvent(ctx context.Context, req *eventpbv1.GetEventRequest) (*eventpbv1.Event, error)
}

// Dishes reads a dish (the CatalogService).
type Dishes interface {
	GetDish(ctx context.Context, req *catalogpbv1.GetDishRequest) (*catalogpbv1.Dish, error)
}

// Guests reads a contact (the GuestService).
type Guests interface {
	GetContact(ctx context.Context, req *guestpbv1.GetContactRequest) (*guestpbv1.Contact, error)
}

// CallServices has s charge done requests and tell their guests through
// svcs: the services built alongside this one.
func (s *Server) CallServices(svcs Services) { s.h.svcs = svcs }

// billed is the booking a request naming booking is billed to: the
// reservation, or the event (of a session); "" for anything else.
func billed(booking string) string {
	parts := strings.Split(booking, "/")
	if len(parts) < 4 || parts[0] != "venues" || (parts[2] != "reservations" && parts[2] != "events") {
		return ""
	}
	return strings.Join(parts[:4], "/")
}

// contactOf is the contact a booking is for.
func (h *hooks) contactOf(ctx context.Context, booking string) (string, error) {
	if h.svcs.Bookings == nil {
		return "", servicex.Blocked("NO_BOOKINGS", requestType, booking, "the server cannot read bookings", "Configure the stay and event services")
	}
	if strings.Contains(booking, "/reservations/") {
		r, err := h.svcs.Bookings.GetReservation(ctx, &staypbv1.GetReservationRequest{Name: booking})
		return r.GetContact(), err
	}
	e, err := h.svcs.Bookings.GetEvent(ctx, &eventpbv1.GetEventRequest{Name: booking})
	return e.GetContact(), err
}

// phoneOf is the first phone of contact; "" when it has none or cannot be
// read.
func (h *hooks) phoneOf(ctx context.Context, contact string) string {
	if h.svcs.Guests == nil || !strings.HasPrefix(contact, "contacts/") {
		return ""
	}
	c, err := h.svcs.Guests.GetContact(ctx, &guestpbv1.GetContactRequest{Name: contact})
	if err != nil || len(c.GetPhones()) == 0 {
		return ""
	}
	return c.GetPhones()[0].GetNumber()
}
