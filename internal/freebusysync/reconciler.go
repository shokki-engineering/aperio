// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package freebusysync

// reconciler.go is the Reconciler: the webhook's safety net. Every round it
// walks the live bookings -- stays not cancelled and not yet departed,
// sessions not yet over -- a page at a time, reads each page's freebusy
// reservations in batches (engine.Lookup), and settles what differs, as the
// webhook would have. A round costs a read per page of aperio and a call per
// fifty reservations of freebusy, whatever the bookings.

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/shokki-engineering/aperio/protobuf/generated/go/event/v1/eventpbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/stay/v1/staypbv1"
)

// reconcilePage is how many bookings a page reads.
const reconcilePage = 100

// reconcileInterval is how often Start runs a round when given no interval.
const reconcileInterval = 5 * time.Minute

// Reconciler walks the live bookings against freebusy.
type Reconciler struct {
	s     *Syncer
	venue string // venues/- for every venue
}

// Reconciler is the job walking the bookings s settles, at every venue.
func (s *Syncer) Reconciler() *Reconciler { return &Reconciler{s: s, venue: "venues/-"} }

// At is r walking venue's bookings only.
func (r *Reconciler) At(venue string) *Reconciler { return &Reconciler{s: r.s, venue: venue} }

// Round settles every live booking once. A page that fails to settle does
// not stop the round; their errors come back joined.
func (r *Reconciler) Round(ctx context.Context) error {
	now := r.s.now()
	var errs []error
	stays := &staypbv1.ListReservationsRequest{Parent: r.venue, PageSize: reconcilePage,
		Filter: fmt.Sprintf("(state = PENDING OR state = CONFIRMED OR state = CHECKED_IN) AND departure_date >= %q",
			now.AddDate(0, 0, -1).Format("2006-01-02"))}
	for {
		page, err := r.s.stays.ListReservations(ctx, stays)
		if err != nil {
			return errors.Join(append(errs, err)...)
		}
		errs = append(errs, r.s.settleStays(ctx, page.GetReservations()))
		if stays.PageToken = page.GetNextPageToken(); stays.PageToken == "" {
			break
		}
	}
	sessions := &eventpbv1.ListSessionsRequest{Parent: r.venue + "/events/-", PageSize: reconcilePage,
		Filter: fmt.Sprintf("end_time >= %q", now.Format(time.RFC3339))}
	if r.venue != "venues/-" {
		// The store reads venues/{v}/events/- as every venue's: say which.
		sessions.Filter += fmt.Sprintf(" AND venue = %q", r.venue)
	}
	for {
		page, err := r.s.events.ListSessions(ctx, sessions)
		if err != nil {
			return errors.Join(append(errs, err)...)
		}
		errs = append(errs, r.s.settleSessions(ctx, page.GetSessions()))
		if sessions.PageToken = page.GetNextPageToken(); sessions.PageToken == "" {
			break
		}
	}
	return errors.Join(errs...)
}

// Start runs Round every interval (five minutes when 0) until ctx is done,
// in a goroutine of its own, handing each failure to report: freebusy's
// StartReconciler's shape.
func (r *Reconciler) Start(ctx context.Context, interval time.Duration, report func(error)) {
	if interval <= 0 {
		interval = reconcileInterval
	}
	go func() {
		t := time.NewTicker(interval)
		defer t.Stop()
		for {
			if err := r.Round(ctx); err != nil && report != nil && ctx.Err() == nil {
				report(err)
			}
			select {
			case <-ctx.Done():
				return
			case <-t.C:
			}
		}
	}()
}
