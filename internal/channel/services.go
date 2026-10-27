// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package channel

// services.go is what the channels ask of aperio's services -- the few gRPC
// methods each needs, so a channel's reservation keeps every rule the stay
// service keeps for anyone's -- and the map from a channel's codes to
// aperio's venues and room types.

import (
	"context"

	"github.com/shokki-engineering/aperio/protobuf/generated/go/guest/v1/guestpbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/pricing/v1/pricingpbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/stay/v1/staypbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/venue/v1/venuepbv1"
)

// Stays is what Sync asks of the StayService.
type Stays interface {
	CreateReservation(context.Context, *staypbv1.CreateReservationRequest) (*staypbv1.Reservation, error)
	UpdateReservation(context.Context, *staypbv1.UpdateReservationRequest) (*staypbv1.Reservation, error)
	ConfirmReservation(context.Context, *staypbv1.ConfirmReservationRequest) (*staypbv1.Reservation, error)
	CancelReservation(context.Context, *staypbv1.CancelReservationRequest) (*staypbv1.Reservation, error)
	ListReservations(context.Context, *staypbv1.ListReservationsRequest) (*staypbv1.ListReservationsResponse, error)
}

// Guests is what Sync asks of the GuestService.
type Guests interface {
	ListContacts(context.Context, *guestpbv1.ListContactsRequest) (*guestpbv1.ListContactsResponse, error)
	CreateContact(context.Context, *guestpbv1.CreateContactRequest) (*guestpbv1.Contact, error)
}

// Prices is what Outbound asks of the PricingService.
type Prices interface {
	CalculatePrice(context.Context, *pricingpbv1.CalculatePriceRequest) (*pricingpbv1.CalculatePriceResponse, error)
}

// Venues is what Outbound asks of the VenueService.
type Venues interface {
	GetVenue(context.Context, *venuepbv1.GetVenueRequest) (*venuepbv1.Venue, error)
	GetSpace(context.Context, *venuepbv1.GetSpaceRequest) (*venuepbv1.Space, error)
}

// Hotel is a venue as a channel knows it: its code there, and its room types
// by the channel's codes.
type Hotel struct {
	// Code is the channel's id for the venue (OTA HotelCode).
	Code string
	// Venue is the venue.
	// Format: venues/{venue}
	Venue string
	// Rooms are its room types the channel sells.
	Rooms []RoomType
}

// RoomType is one of a venue's room types as a channel sells it.
type RoomType struct {
	// Code is the channel's room type code (OTA InvTypeCode, RoomTypeCode).
	Code string
	// Space is the room type.
	// Format: venues/{venue}/spaces/{space}
	Space string
	// RatePlan is the rate plan its rates are sent on (OTA RatePlanCode).
	RatePlan string
}

// space is the room type the channel codes as code at h; "" when unmapped.
func (h Hotel) space(code string) string {
	for _, r := range h.Rooms {
		if r.Code == code {
			return r.Space
		}
	}
	return ""
}

// hotel is the hotel with code; the only one when code is empty and there is
// only one.
func hotel(hotels []Hotel, code string) (Hotel, bool) {
	if code == "" && len(hotels) == 1 {
		return hotels[0], true
	}
	for _, h := range hotels {
		if h.Code == code {
			return h, true
		}
	}
	return Hotel{}, false
}
