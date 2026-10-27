// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package bookingcom

// config.go builds Booking.com's side from the configuration
// ([channels.bookingcom]): its hotels in package channel's terms, and the
// Sync and Handler over the stay and guest services.

import (
	"github.com/shokki-engineering/aperio/internal/channel"
	"github.com/shokki-engineering/aperio/internal/config"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/type/v1/typepbv1"
)

// Path is where the inbound handler is served on the REST port.
const Path = "/channels/bookingcom/reservations"

// Hotels are cfg's hotels.
func Hotels(cfg config.BookingComConfig) []channel.Hotel {
	var out []channel.Hotel
	for _, h := range cfg.Hotels {
		hotel := channel.Hotel{Code: h.HotelCode, Venue: h.Venue}
		for _, r := range h.Rooms {
			hotel.Rooms = append(hotel.Rooms, channel.RoomType{Code: r.Code, Space: r.Space, RatePlan: r.RatePlan})
		}
		out = append(out, hotel)
	}
	return out
}

// NewSync is the Sync that books Booking.com's reservations at hotels.
func NewSync(hotels []channel.Hotel, stays channel.Stays, guests channel.Guests) *channel.Sync {
	return &channel.Sync{Channel: typepbv1.Channel_CHANNEL_BOOKING_COM, Prefix: Prefix, Hotels: hotels, Stays: stays, Guests: guests}
}

// FromConfig is the inbound Handler cfg configures, over stays and guests.
func FromConfig(cfg config.BookingComConfig, stays channel.Stays, guests channel.Guests) *Handler {
	return NewHandler(NewSync(Hotels(cfg), stays, guests), cfg.Username, cfg.Password)
}
