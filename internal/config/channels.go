// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package config

// channels.go configures the sales channels (internal/channel): which
// venues each sells, by the channel's codes, and the credentials it logs in
// with. Booking.com's hotels are an array of tables, which no single
// environment variable can set, so they can also be given as JSON
// (hotels_json, APERIO_CHANNELS__BOOKINGCOM__HOTELS_JSON).

import (
	"encoding/json"
	"fmt"
	"strings"
)

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
	// HotelsJSON is Hotels as a JSON array, with the same keys
	// ([{"hotel_code": ..., "venue": ..., "rooms": [{"code": ..., "space":
	// ..., "rate_plan": ...}]}]): how a deployment configured only through
	// the environment sets them. Set, it replaces Hotels.
	HotelsJSON string `koanf:"hotels_json"`
}

// resolve parses HotelsJSON into Hotels, when set, and checks every hotel
// names its code and venue, and every room its code and space.
func (b *BookingComConfig) resolve() error {
	if raw := strings.TrimSpace(b.HotelsJSON); raw != "" {
		var hotels []ChannelHotel
		if err := json.Unmarshal([]byte(raw), &hotels); err != nil {
			return fmt.Errorf("config invalid: channels.bookingcom.hotels_json: %w", err)
		}
		b.Hotels = hotels
	}
	for i, h := range b.Hotels {
		if h.HotelCode == "" || !strings.HasPrefix(h.Venue, "venues/") {
			return fmt.Errorf("config invalid: channels.bookingcom.hotels[%d] needs a hotel_code and a venue (venues/{venue})", i)
		}
		for j, r := range h.Rooms {
			if r.Code == "" || !strings.HasPrefix(r.Space, h.Venue+"/spaces/") {
				return fmt.Errorf("config invalid: channels.bookingcom.hotels[%d].rooms[%d] needs a code and a space of %s (%s/spaces/{space})", i, j, h.Venue, h.Venue)
			}
		}
	}
	return nil
}

// ChannelHotel is a venue as a channel knows it.
type ChannelHotel struct {
	// HotelCode is the channel's id for the venue.
	HotelCode string `koanf:"hotel_code" json:"hotel_code"`
	// Venue is the venue, venues/{venue}.
	Venue string `koanf:"venue" json:"venue"`
	// Rooms are its room types the channel sells.
	Rooms []ChannelRoom `koanf:"rooms" json:"rooms"`
}

// ChannelRoom is a room type as a channel sells it.
type ChannelRoom struct {
	// Code is the channel's room type code.
	Code string `koanf:"code" json:"code"`
	// Space is the room type, venues/{venue}/spaces/{space}.
	Space string `koanf:"space" json:"space"`
	// RatePlan is the channel's rate plan code its rates are sent on.
	RatePlan string `koanf:"rate_plan" json:"rate_plan"`
}
