// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package sales

// expiry.go is the job that lapses offers: a SENT quote past its
// expire_time becomes EXPIRED, read through the (state, expire_time) index
// a page at a time. AcceptQuote refuses a lapsed offer whether or not the
// job has reached it; the job makes the state say so, for the pipeline and
// for ReviseQuote.

import (
	"context"
	"fmt"
	"time"

	"github.com/shokki-engineering/aperio/internal/database/repository/repox"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/sales/v1/salespbv1"
)

// expiryPage is how many quotes one read of a round takes.
const expiryPage = 100

// expiryInterval is how often Start runs a round when given no interval.
const expiryInterval = time.Minute

// Expiry expires lapsed offers: a background job.
type Expiry struct{ s *Server }

// QuoteExpiry is the job expiring s's lapsed offers.
func (s *Server) QuoteExpiry() *Expiry { return &Expiry{s: s} }

// Round expires every SENT quote past its expire_time, returning how many.
// An expired quote leaves the filter, so each page is read from the start;
// a page that expires none -- each one failing -- ends the round.
func (e *Expiry) Round(ctx context.Context) (int, error) {
	in := repox.ListInput{PageSize: expiryPage, OrderBy: "expire_time", ReadMask: []string{"name"},
		Filter: fmt.Sprintf("state = SENT AND expire_time < %q", e.s.h.now().Format(time.RFC3339Nano))}
	n := 0
	for {
		page, _, err := e.s.Repositories().Quotes.List(ctx, "venues/-", in)
		if err != nil {
			return n, err
		}
		done := 0
		var last error
		for _, q := range page {
			if err := e.expire(ctx, q.GetName()); err != nil {
				last = err
				continue
			}
			done++
		}
		n += done
		if len(page) < expiryPage || done == 0 {
			return n, last
		}
	}
}

// expire moves one quote from SENT to EXPIRED, if it is still both.
func (e *Expiry) expire(ctx context.Context, name string) error {
	_, err := e.s.move(ctx, "ExpireQuote", name, "", []salespbv1.Quote_State{salespbv1.Quote_STATE_SENT},
		func(ctx context.Context, q *salespbv1.Quote) (*salespbv1.Quote, error) {
			if q.GetExpireTime() == nil || e.s.h.now().Before(q.GetExpireTime().AsTime()) {
				return q, nil // sent again with a new clock since it was read
			}
			q.State = salespbv1.Quote_STATE_EXPIRED
			return e.s.Repositories().Quotes.UpdateOutputOnly(ctx, q, []string{"state"})
		})
	return err
}

// Start runs Round every interval (a minute when 0) until ctx is done, in
// a goroutine of its own, handing each failure to report.
func (e *Expiry) Start(ctx context.Context, interval time.Duration, report func(error)) {
	if interval <= 0 {
		interval = expiryInterval
	}
	go func() {
		t := time.NewTicker(interval)
		defer t.Stop()
		for {
			if _, err := e.Round(ctx); err != nil && report != nil && ctx.Err() == nil {
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
