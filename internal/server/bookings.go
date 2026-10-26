// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package server

// bookings.go is finance's view of the bookings a payment confirms: a stay
// through the stay service, an event through the event service.

import (
	"context"

	"github.com/shokki-engineering/aperio/internal/service/event"
	"github.com/shokki-engineering/aperio/internal/service/stay"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/event/v1/eventpbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/stay/v1/staypbv1"
)

// bookings is finance.Confirmer over the stay and event services.
type bookings struct {
	stays  *stay.Server
	events *event.Server
}

func (b bookings) GetReservation(ctx context.Context, r *staypbv1.GetReservationRequest) (*staypbv1.Reservation, error) {
	return b.stays.GetReservation(ctx, r)
}

func (b bookings) ConfirmReservation(ctx context.Context, r *staypbv1.ConfirmReservationRequest) (*staypbv1.Reservation, error) {
	return b.stays.ConfirmReservation(ctx, r)
}

func (b bookings) GetEvent(ctx context.Context, r *eventpbv1.GetEventRequest) (*eventpbv1.Event, error) {
	return b.events.GetEvent(ctx, r)
}

func (b bookings) ConfirmEvent(ctx context.Context, r *eventpbv1.ConfirmEventRequest) (*eventpbv1.Event, error) {
	return b.events.ConfirmEvent(ctx, r)
}
