// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package booking is what the stay and event services share in taking a
// venue's rooms and halls from freebusy: the venue's clock, the freebusy
// resources of a space, giving reservations back, and the calendar notes'
// location and guest. It is a library, not a service: each service's hooks
// embed a Base.
package booking

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/shokki-engineering/aperio/internal/database/repository/aperio/guest"
	"github.com/shokki-engineering/aperio/internal/database/repository/aperio/venue"
	"github.com/shokki-engineering/aperio/internal/engine"
	"github.com/shokki-engineering/aperio/internal/service/generated/servicex"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/guest/v1/guestpbv1"
)

// The resource types errors name.
const (
	VenueType = "aperio.venue.v1/Venue"
	SpaceType = "aperio.venue.v1/Space"
)

// Base is what a booking service's hooks share.
type Base struct {
	Engine   engine.Engine
	Venues   venue.VenueRepository
	Spaces   venue.SpaceRepository
	Contacts guest.ContactRepository
	Now      func() time.Time
}

// NewBase is a Base taking rooms through eng, reading venues and spaces from
// venues and the guests notes name from contacts.
func NewBase(eng engine.Engine, venues venue.Repositories, contacts guest.ContactRepository) *Base {
	return &Base{Engine: eng, Venues: venues.Venues, Spaces: venues.Spaces, Contacts: contacts, Now: time.Now}
}

// Zone is venueName's time zone.
func (b *Base) Zone(ctx context.Context, venueName string) (*time.Location, error) {
	v, err := b.Venues.Get(ctx, venueName)
	if err != nil {
		return nil, servicex.NotFoundAs(err, VenueType, venueName)
	}
	loc, err := time.LoadLocation(v.GetTimeZone())
	if err != nil {
		return nil, servicex.Blocked("INVALID_TIME_ZONE", VenueType, venueName,
			"the venue's time zone "+v.GetTimeZone()+" is not an IANA zone", "Set the venue's time_zone, e.g. Asia/Kolkata")
	}
	return loc, nil
}

// Resources are the freebusy resources of space, which must be one of
// venueName's and must have some: a space with none cannot be booked. field
// names the request field space came in, for errors.
func (b *Base) Resources(ctx context.Context, venueName, field, space string) ([]string, error) {
	if servicex.ParentOf(space) != venueName {
		return nil, servicex.BadField("SPACE_OF_ANOTHER_VENUE", field,
			space+" is not a space of "+venueName, "Name a space of the venue the booking is made at")
	}
	sp, err := b.Spaces.Get(ctx, space)
	if err != nil {
		return nil, servicex.NotFoundAs(err, SpaceType, space)
	}
	if len(sp.GetResources()) == 0 {
		return nil, servicex.Blocked("SPACE_NOT_BOOKABLE", SpaceType, space,
			"the space names no freebusy resources", "Add the space's rooms or halls to its resources")
	}
	return sp.GetResources(), nil
}

// Release cancels freebusy reservations.
func (b *Base) Release(ctx context.Context, reservations []string, reason string) error {
	var errs []error
	for _, name := range reservations {
		if err := b.Engine.Cancel(ctx, name, reason); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// Where is a calendar note's location: the space, the venue, and its address.
func (b *Base) Where(ctx context.Context, venueName, space string) string {
	var parts []string
	if sp, err := b.Spaces.Get(ctx, space); err == nil {
		parts = append(parts, sp.GetDisplayName())
	}
	if v, err := b.Venues.Get(ctx, venueName); err == nil {
		parts = append(parts, v.GetDisplayName())
		parts = append(parts, v.GetAddress().GetAddressLines()...)
		if v.GetAddress().GetLocality() != "" {
			parts = append(parts, v.GetAddress().GetLocality())
		}
	}
	return strings.Join(parts, ", ")
}

// Contact reads name, or nil when it cannot be read: a note then names the
// guest by their resource name.
func (b *Base) Contact(ctx context.Context, name string) *guestpbv1.Contact {
	c, err := b.Contacts.Get(ctx, name)
	if err != nil {
		return nil
	}
	return c
}

// NoteFailed logs a calendar note that could not be written: the booking
// stands without it, and its next edit writes it again.
func (b *Base) NoteFailed(ctx context.Context, booking string, err error) {
	slog.WarnContext(ctx, "booking: calendar note not written", "booking", booking, "err", err)
}
