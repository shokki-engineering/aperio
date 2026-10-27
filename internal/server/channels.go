// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package server

// channels.go serves the sales channels' inbound messages beside the REST
// gateway (routes.go): Booking.com's OTA_HotelResNotif, which books, changes
// and cancels stays through the stay and guest services (internal/channel).

import (
	"net/http"

	"github.com/shokki-engineering/aperio/internal/channel/bookingcom"
	"github.com/shokki-engineering/aperio/internal/config"
)

// useChannels mounts each channel cfg enables.
func (s *Services) useChannels(cfg config.ChannelsConfig) {
	if bc := cfg.BookingCom; bc.Enabled {
		s.Route(http.MethodPost, bookingcom.Path, bookingcom.FromConfig(bc, s.Stays, s.Guests))
	}
}
