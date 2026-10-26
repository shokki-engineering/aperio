// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package server

// repositories_booking.go builds the BookingService's repositories, in a file of
// its own: the repository package is named booking, as the service package
// assembly.go imports is.

import (
	"github.com/shokki-engineering/aperio/internal/database/repository/aperio/booking"
	"github.com/shokki-engineering/aperio/internal/database/repository/repox"
	"github.com/shokki-engineering/aperio/internal/service/generated/aperio/booking/v1/bookingpbv1svc"
)

// bookingRepositories are the BookingService's repositories over conn.
func bookingRepositories(conn repox.Conn) bookingpbv1svc.BookingServiceRepositories {
	r := booking.New(conn)
	return bookingpbv1svc.BookingServiceRepositories{Reservations: r.Reservations, Events: r.Events, Sessions: r.Sessions}
}
