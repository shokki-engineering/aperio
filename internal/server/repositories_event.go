// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package server

// repositories_event.go builds the EventService's repositories, in a file of
// its own: the repository package is named event, as the service package
// assembly.go imports is.

import (
	"github.com/shokki-engineering/aperio/internal/database/repository/aperio/event"
	"github.com/shokki-engineering/aperio/internal/database/repository/repox"
	"github.com/shokki-engineering/aperio/internal/service/generated/aperio/event/v1/eventpbv1svc"
)

// eventRepositories are the EventService's repositories over conn.
func eventRepositories(conn repox.Conn) eventpbv1svc.EventServiceRepositories {
	r := event.New(conn)
	return eventpbv1svc.EventServiceRepositories{Events: r.Events, Sessions: r.Sessions}
}
