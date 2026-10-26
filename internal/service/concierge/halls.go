// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package concierge

// halls.go is which of a venue's halls an event could have: those that seat
// its guests in some layout and are free in freebusy for the whole day --
// the sessions' times are not known at an enquiry, so the day is asked.

import (
	"context"
	"slices"
	"time"

	"github.com/shokki-engineering/aperio/internal/database/repository/repox"
	"github.com/shokki-engineering/aperio/internal/engine"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/concierge/v1/conciergepbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/venue/v1/venuepbv1"
)

// freeHalls are the display names of v's halls that seat req's guests and
// are free all of req's day.
func (s *Server) freeHalls(ctx context.Context, v *venuepbv1.Venue, req *conciergepbv1.EnquireEventRequest) ([]string, error) {
	loc, err := time.LoadLocation(v.GetTimeZone())
	if err != nil {
		loc = time.UTC
	}
	w := engine.Stay(req.GetEventDate(), engine.AddDays(req.GetEventDate(), 1), loc)
	spaces, _, err := s.spaces.List(ctx, v.GetName(), repox.ListInput{PageSize: 200})
	if err != nil {
		return nil, err
	}
	var out []string
	for _, sp := range spaces {
		if sp.GetKind() != venuepbv1.SpaceKind_SPACE_KIND_HALL || len(sp.GetResources()) == 0 || !seats(sp, req.GetGuests()) {
			continue
		}
		if free, err := s.engine.Free(ctx, sp.GetResources(), w, 1); err == nil && free {
			out = append(out, sp.GetDisplayName())
		}
	}
	return out, nil
}

// seats reports whether hall sp seats guests in some layout; a hall that
// states no layouts is taken to.
func seats(sp *venuepbv1.Space, guests int32) bool {
	layouts := sp.GetCapacity().GetLayouts()
	return len(layouts) == 0 || slices.ContainsFunc(layouts, func(l *venuepbv1.LayoutCapacity) bool { return l.GetGuests() >= guests })
}
