// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package operations serves aperio.operations.v1 -- the service requests a
// venue's departments answer -- on the generated AIP server, with what the
// generated layer leaves to custom methods: the state methods that move a
// request from asked to done (states.go); the department told the moment a
// request is made, and the guest when it is done (notify.go); what a done
// request costs posted to its booking's bill (charge.go); and
// SummarizeServiceRequests, each department's open queue and the day's
// completed requests (summary.go).
package operations

import (
	"context"
	"time"

	"github.com/shokki-engineering/aperio/internal/database/repository/aperio/venue"
	"github.com/shokki-engineering/aperio/internal/notify"
	"github.com/shokki-engineering/aperio/internal/service/generated/aperio/operations/v1/operationspbv1svc"
	"github.com/shokki-engineering/aperio/internal/service/generated/servicex"
)

// Server serves aperio.operations.v1.OperationsService: the generated
// standard methods, the state methods and SummarizeServiceRequests.
type Server struct {
	*operationspbv1svc.OperationsServiceServer
	h *hooks
}

// New returns the Server over repos, reading each venue's time zone,
// currency and GST state from venues. It tells no one and charges nothing
// until NotifyThrough and CallServices give it the means.
func New(repos operationspbv1svc.OperationsServiceRepositories, venues venue.VenueRepository, opts ...servicex.Option) *Server {
	h := &hooks{repos: repos, venues: venues, now: time.Now}
	return &Server{OperationsServiceServer: operationspbv1svc.NewOperationsServiceServer(repos, h, opts...), h: h}
}

// hooks is the business logic around the generated standard methods.
type hooks struct {
	operationspbv1svc.NopOperationsServiceHooks

	repos  operationspbv1svc.OperationsServiceRepositories
	venues venue.VenueRepository
	now    func() time.Time

	sender *notify.Sender // nil tells no one
	routes routes         // where each department is told
	svcs   Services       // the services a request is charged and its guest found through
}

// venueType is the resource type errors name.
const venueType = "aperio.venue.v1/Venue"

// zone is venueName's time zone.
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
