// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package sales

// leads.go holds a lead's dates in freebusy while it is negotiated
// (Lead.hold): a lead at a venue that asks for rooms holds that many of the
// venue's rooms for its first dates, so they are not sold under the
// salesperson. The hold is freebusy's, which lapses: it is taken for a day
// (freebusy's longest) and pushed a day on from every edit of the lead, and
// when nobody touches the lead for a day it lapses and Lead.hold is cleared
// (HoldLapsed, told by freebusy's webhook). It is released when the lead is
// won or lost, or its dates, rooms or venue change (and taken again for the
// new ones). Holding is best effort: a lead whose rooms are taken is stored
// without a hold, as the interest is real whether or not they are free.
//
// A lead names how many rooms but not which, nor a hall, so the hold takes
// any of the venue's rooms and an event lead holds no hall; holding a named
// space needs a field the Lead does not have.

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/shokki-engineering/aperio/internal/booking"
	"github.com/shokki-engineering/aperio/internal/database/repository/repox"
	"github.com/shokki-engineering/aperio/internal/engine"
	"github.com/shokki-engineering/aperio/internal/service/generated/servicex"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/sales/v1/salespbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/venue/v1/venuepbv1"
	"google.golang.org/protobuf/proto"
)

// leadHoldTTL is how long a lead's hold lasts from its last edit: freebusy's
// MaxHoldExtension.
const leadHoldTTL = 24 * time.Hour

// AfterCreateLead holds the new lead's dates.
func (h *hooks) AfterCreateLead(ctx context.Context, out *salespbv1.Lead) error {
	h.keepHold(ctx, out)
	return nil
}

// BeforeUpdateLead releases the hold of a lead whose dates, rooms or venue
// change, or that is won or lost: AfterUpdateLead takes one for what it
// asks now, if anything.
func (h *hooks) BeforeUpdateLead(ctx context.Context, m *salespbv1.Lead, paths []string) error {
	asks := servicex.TouchesAny(paths, []string{"venue", "dates", "rooms"})
	closes := servicex.TouchesAny(paths, []string{"stage"}) &&
		(m.GetStage() == salespbv1.Stage_STAGE_WON || m.GetStage() == salespbv1.Stage_STAGE_LOST)
	if h.base == nil || (!asks && !closes) {
		return nil // a step along the pipeline: the hold stays
	}
	if existing, err := h.repos.Leads.Get(ctx, m.GetName()); err == nil && existing.GetHold() != "" {
		h.letGo(ctx, existing.GetName(), existing.GetHold())
	}
	return nil
}

// AfterUpdateLead keeps the edited lead's hold: a day on from now, or taken
// afresh.
func (h *hooks) AfterUpdateLead(ctx context.Context, out *salespbv1.Lead) error {
	h.keepHold(ctx, out)
	return nil
}

// holds reports whether l asks for a hold: open, at a venue, with dates and
// rooms.
func holds(l *salespbv1.Lead) bool {
	open := l.GetStage() != salespbv1.Stage_STAGE_WON && l.GetStage() != salespbv1.Stage_STAGE_LOST
	return open && l.GetVenue() != "" && len(l.GetDates()) > 0 && l.GetRooms() > 0
}

// keepHold makes l's hold what l asks: none, the one it has a day longer, or
// a new one. It writes Lead.hold when that changes, and only logs failures.
func (h *hooks) keepHold(ctx context.Context, l *salespbv1.Lead) {
	if h.base == nil {
		return
	}
	hold := ""
	switch {
	case !holds(l):
		if l.GetHold() != "" {
			h.letGo(ctx, l.GetName(), l.GetHold())
		}
	case l.GetHold() != "":
		_, err := h.base.Engine.ExtendHold(ctx, l.GetHold(), leadHoldTTL)
		if err == nil {
			return
		}
		if !errors.Is(err, engine.ErrLapsed) {
			slog.WarnContext(ctx, "sales: lead hold not extended", "lead", l.GetName(), "err", err)
			return
		}
		fallthrough
	default:
		var err error
		if hold, err = h.takeHold(ctx, l); err != nil {
			slog.WarnContext(ctx, "sales: lead dates not held", "lead", l.GetName(), "err", err)
		}
	}
	if hold == l.GetHold() {
		return
	}
	l.Hold = hold
	updated, err := h.repos.Leads.UpdateOutputOnly(ctx, l, []string{"hold"})
	if err != nil {
		slog.WarnContext(ctx, "sales: lead hold not recorded", "lead", l.GetName(), "hold", hold, "err", err)
		return
	}
	proto.Reset(l)
	proto.Merge(l, updated)
}

// takeHold holds l.rooms of its venue's rooms for the nights of its first
// dates, for leadHoldTTL.
func (h *hooks) takeHold(ctx context.Context, l *salespbv1.Lead) (string, error) {
	loc, err := h.base.Zone(ctx, l.GetVenue())
	if err != nil {
		return "", err
	}
	rooms, err := h.roomsOf(ctx, l.GetVenue())
	if err != nil || len(rooms) == 0 {
		return "", err
	}
	d := l.GetDates()[0]
	depart := d.GetEndDate()
	if days(d.GetStartDate(), depart) < 1 {
		depart = addDays(d.GetStartDate(), 1)
	}
	held, err := h.base.Engine.Hold(ctx, engine.Booking{
		Key: booking.LeadKey(l.GetName(), h.now()), Resources: rooms,
		Window: engine.Stay(d.GetStartDate(), depart, loc), Quantity: l.GetRooms(),
	})
	if err != nil {
		return "", err
	}
	if _, err := h.base.Engine.ExtendHold(ctx, held.Name, leadHoldTTL); err != nil {
		return held.Name, fmt.Errorf("held for minutes only: %w", err)
	}
	return held.Name, nil
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
			if k := sp.GetKind(); k == venuepbv1.SpaceKind_SPACE_KIND_ROOM_TYPE || k == venuepbv1.SpaceKind_SPACE_KIND_UNIT {
				out = append(out, sp.GetResources()...)
			}
		}
		if next == "" {
			return out, nil
		}
		in.PageToken = next
	}
}

// letGo releases lead's hold, logging a failure: freebusy lapses it within a
// day whatever happens.
func (h *hooks) letGo(ctx context.Context, lead, hold string) {
	if err := h.base.Engine.ReleaseHold(ctx, hold); err != nil {
		slog.WarnContext(ctx, "sales: lead hold not released", "lead", lead, "hold", hold, "err", err)
	}
}
