// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package server

// repositories_stay.go builds the StayService's repositories, in a file of
// its own: the repository package is named stay, as the service package
// assembly.go imports is.

import (
	"github.com/shokki-engineering/aperio/internal/database/repository/aperio/stay"
	"github.com/shokki-engineering/aperio/internal/database/repository/repox"
	"github.com/shokki-engineering/aperio/internal/service/generated/aperio/stay/v1/staypbv1svc"
)

// stayRepositories are the StayService's repositories over conn.
func stayRepositories(conn repox.Conn) staypbv1svc.StayServiceRepositories {
	r := stay.New(conn)
	return staypbv1svc.StayServiceRepositories{Reservations: r.Reservations}
}
