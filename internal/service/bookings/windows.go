// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package bookings

// windows.go turns a booking into what freebusy is asked about: the window of
// time it takes, and the freebusy resources of the space it takes them in.

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/shokki-engineering/aperio/internal/engine"
	"github.com/shokki-engineering/aperio/internal/service/generated/servicex"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/booking/v1/bookingpbv1"
	"google.golang.org/genproto/googleapis/type/date"
)

// stayWindow is the window a stay takes in its venue's time zone: from the
// arrival day's midnight to the departure day's, so the nights between are
// the guest's and a stay ending on a day never overlaps one starting on it.
func (h *hooks) stayWindow(ctx context.Context, venueName string, r *bookingpbv1.Reservation) (engine.Window, error) {
	loc, err := h.zone(ctx, venueName)
	if err != nil {
		return engine.Window{}, err
	}
	w := engine.Window{Start: midnight(r.GetArrivalDate(), loc), End: midnight(r.GetDepartureDate(), loc)}
	if !w.End.After(w.Start) {
		return engine.Window{}, servicex.BadField("INVALID_DATES", "reservation.departure_date",
			"the departure date is not after the arrival date", "Set a departure date after the arrival date")
	}
	return w, nil
}

// sessionWindow is the window a session takes its space for: from setup to
// teardown, or from its start to its end where those are not set.
func sessionWindow(s *bookingpbv1.Session) (engine.Window, error) {
	w := engine.Window{Start: s.GetStartTime().AsTime(), End: s.GetEndTime().AsTime()}
	if s.GetSetupTime() != nil {
		w.Start = s.GetSetupTime().AsTime()
	}
	if s.GetTeardownTime() != nil {
		w.End = s.GetTeardownTime().AsTime()
	}
	if !s.GetEndTime().AsTime().After(s.GetStartTime().AsTime()) || !w.End.After(w.Start) {
		return engine.Window{}, servicex.BadField("INVALID_TIMES", "session.end_time",
			"the session ends before it starts, or is torn down before it is set up",
			"Set start_time before end_time, and setup_time and teardown_time around them")
	}
	return w, nil
}

// zone is venue's time zone.
func (h *hooks) zone(ctx context.Context, venueName string) (*time.Location, error) {
	v, err := h.venues.Get(ctx, venueName)
	if err != nil {
		return nil, servicex.NotFoundAs(err, venueType, venueName)
	}
	loc, err := time.LoadLocation(v.GetTimeZone())
	if err != nil {
		return nil, servicex.Blocked("INVALID_TIME_ZONE", venueType, venueName,
			"the venue's time zone "+v.GetTimeZone()+" is not an IANA zone", "Set the venue's time_zone, e.g. Asia/Kolkata")
	}
	return loc, nil
}

// resources are the freebusy resources of space, which must be one of
// venueName's and must have some: a space with none cannot be booked.
func (h *hooks) resources(ctx context.Context, venueName, field, space string) ([]string, error) {
	if servicex.ParentOf(space) != venueName {
		return nil, servicex.BadField("SPACE_OF_ANOTHER_VENUE", field,
			space+" is not a space of "+venueName, "Name a space of the venue the booking is made at")
	}
	sp, err := h.spaces.Get(ctx, space)
	if err != nil {
		return nil, servicex.NotFoundAs(err, spaceType, space)
	}
	if len(sp.GetResources()) == 0 {
		return nil, servicex.Blocked("SPACE_NOT_BOOKABLE", spaceType, space,
			"the space names no freebusy resources", "Add the space's rooms or halls to its resources")
	}
	return sp.GetResources(), nil
}

// midnight is the start of day d in loc.
func midnight(d *date.Date, loc *time.Location) time.Time {
	return time.Date(int(d.GetYear()), time.Month(d.GetMonth()), int(d.GetDay()), 0, 0, 0, 0, loc)
}

// release cancels freebusy reservations.
func (h *hooks) release(ctx context.Context, reservations []string, reason string) error {
	var errs []error
	for _, name := range reservations {
		if err := h.engine.Cancel(ctx, name, reason); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// unavailable is the error for rooms of space that are not free.
func unavailable(space string, rooms int32) error {
	return servicex.Blocked("NOT_AVAILABLE", spaceType, space,
		fmt.Sprintf("%d of the space are not free for the dates", rooms),
		"Choose other dates or another space, or book fewer")
}
