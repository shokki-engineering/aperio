// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package live

// event.go is the event repositories, in a file of their own: the repository
// package is named event, as the service package a test imports is.

import (
	"github.com/shokki-engineering/aperio/internal/database/repository/aperio/event"
	"github.com/shokki-engineering/aperio/internal/database/repository/repox"
	"github.com/shokki-engineering/aperio/internal/service/generated/aperio/event/v1/eventpbv1svc"
)

// EventRepositories are the EventService's repositories over conn.
func EventRepositories(conn repox.Conn) eventpbv1svc.EventServiceRepositories {
	r := event.New(conn)
	return eventpbv1svc.EventServiceRepositories{Events: r.Events, Sessions: r.Sessions}
}
