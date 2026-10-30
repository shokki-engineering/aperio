// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package sales

// lead_spaces.go is what a lead's hold takes (leads.go has when). A lead
// naming spaces (Lead.spaces, the halls or room types it asks for) holds
// the space it names: as many of a room type's rooms as Lead.rooms says,
// at least one, for the nights of its first dates; one hall, for those
// whole days. A lead naming none holds Lead.rooms of any of the venue's
// rooms, as it always has.
//
// Lead.hold names a single freebusy hold, and one hold takes from one set
// of resources, so a lead naming several spaces holds the first of them:
// the rest are recorded as asked for, and taken when a quote for them is
// accepted. Holding them all needs a repeated hold on the Lead.

import (
	"context"
	"time"

	"github.com/shokki-engineering/aperio/internal/booking"
	"github.com/shokki-engineering/aperio/internal/database/repository/repox"
	"github.com/shokki-engineering/aperio/internal/engine"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/sales/v1/salespbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/venue/v1/venuepbv1"
)

// wants is the hold l asks for, in loc, without its key: no resources when
// there is nothing to hold.
func (h *hooks) wants(ctx context.Context, l *salespbv1.Lead, loc *time.Location) (engine.Booking, error) {
	d := l.GetDates()[0]
	depart := d.GetEndDate()
	if days(d.GetStartDate(), depart) < 1 {
		depart = addDays(d.GetStartDate(), 1)
	}
	nights := engine.Stay(d.GetStartDate(), depart, loc)
	if len(l.GetSpaces()) == 0 {
		rooms, err := h.roomsOf(ctx, l.GetVenue())
		return engine.Booking{Resources: rooms, Window: nights, Quantity: l.GetRooms()}, err
	}
	sp, err := h.base.Bookable(ctx, l.GetVenue(), "lead.spaces[0]", l.GetSpaces()[0])
	if err != nil {
		return engine.Booking{}, err
	}
	if rooms(sp) {
		return engine.Booking{Resources: sp.GetResources(), Window: nights, Quantity: max(l.GetRooms(), 1)}, nil
	}
	return engine.Booking{Resources: sp.GetResources(), Window: booking.Days(d, loc), Quantity: 1}, nil
}

// rooms reports whether sp is rooms, held by the night; anything else is a
// hall, held by the day.
func rooms(sp *venuepbv1.Space) bool {
	k := sp.GetKind()
	return k == venuepbv1.SpaceKind_SPACE_KIND_ROOM_TYPE || k == venuepbv1.SpaceKind_SPACE_KIND_UNIT
}

// roomsOf is the freebusy resources of every room of venueName.
func (h *hooks) roomsOf(ctx context.Context, venueName string) ([]string, error) {
	var out []string
	in := repox.ListInput{PageSize: 200}
	for {
		spaces, next, err := h.base.Spaces.List(ctx, venueName, in)
		if err != nil {
			return nil, err
		}
		for _, sp := range spaces {
			if rooms(sp) {
				out = append(out, sp.GetResources()...)
			}
		}
		if next == "" {
			return out, nil
		}
		in.PageToken = next
	}
}
