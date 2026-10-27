// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package config

// channels.go configures the sales channels (internal/channel): which
// venues each sells, by the channel's codes, and the credentials it logs in
// with.

// ChannelsConfig is the channels' settings.
type ChannelsConfig struct {
	// BookingCom is Booking.com, over OTA XML.
	BookingCom BookingComConfig `koanf:"bookingcom"`
}

// BookingComConfig is Booking.com's settings.
type BookingComConfig struct {
	// Enabled serves the inbound reservation endpoint.
	Enabled bool `koanf:"enabled"`
	// Username and Password are the HTTP basic auth Booking.com posts
	// reservations with. Secrets: set them through
	// APERIO_CHANNELS__BOOKINGCOM__USERNAME and
	// APERIO_CHANNELS__BOOKINGCOM__PASSWORD, not a file. With either empty,
	// every inbound message is refused.
	Username string `koanf:"username"`
	Password string `koanf:"password"`
	// Hotels are the venues sold on Booking.com.
	Hotels []ChannelHotel `koanf:"hotels"`
}

// ChannelHotel is a venue as a channel knows it.
type ChannelHotel struct {
	// HotelCode is the channel's id for the venue.
	HotelCode string `koanf:"hotel_code"`
	// Venue is the venue, venues/{venue}.
	Venue string `koanf:"venue"`
	// Rooms are its room types the channel sells.
	Rooms []ChannelRoom `koanf:"rooms"`
}

// ChannelRoom is a room type as a channel sells it.
type ChannelRoom struct {
	// Code is the channel's room type code.
	Code string `koanf:"code"`
	// Space is the room type, venues/{venue}/spaces/{space}.
	Space string `koanf:"space"`
	// RatePlan is the channel's rate plan code its rates are sent on.
	RatePlan string `koanf:"rate_plan"`
}
