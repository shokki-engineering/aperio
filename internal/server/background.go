// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package server

// background.go runs aperio's background tasks beside the server, as
// freebusy's Services.Start does: each one a goroutine tied to the server's
// context, reporting its failures by name. The server starts them once it
// serves:
//
//	s.services.Start(s.ctx, s.services.Background())
//
// A job joins by filling its slot in Background and its line in Start; one
// that is off, or not built, leaves its slot nil.

import (
	"context"
	"time"

	"github.com/shokki-engineering/aperio/internal/config"
	"github.com/shokki-engineering/aperio/internal/database"
	"github.com/shokki-engineering/aperio/internal/telemetry"
)

// Job is a background task: Start runs it every interval until ctx is done,
// in goroutines of its own, and returns at once; it hands each failure to
// report and carries on. An interval of 0 is the job's own default. It is
// the shape of freebusy's reconciler, hold sweeper and purger.
type Job interface {
	Start(ctx context.Context, interval time.Duration, report func(error))
}

// Background selects the background tasks Start runs.
type Background struct {
	// QuoteExpiry moves sent quotes past their expire_time to expired, and
	// releases the holds they kept (the sales service's job); nil is off.
	QuoteExpiry Job
	// Reconciler finishes bookings left between aperio and freebusy -- a
	// hold or reservation one side has and the other does not -- (the
	// booking services' job); nil is off.
	Reconciler Job
	// QuoteExpiryInterval and ReconcileInterval are how often each runs; 0
	// is the job's own default.
	QuoteExpiryInterval, ReconcileInterval time.Duration
	// PoolMonitor publishes the Postgres pool's health to the process's
	// telemetry pipeline (database.StartPoolMonitor).
	PoolMonitor bool
	// Report receives each failure of a task, named by task; nil drops them.
	Report func(task string, err error)
}

// Background is the server's selection: every job the services built, the
// pool monitor, and failures logged through telemetry.
func (s *Services) Background() Background {
	cfg := config.Get().FreebusySync
	b := Background{
		PoolMonitor: true,
		Report: func(task string, err error) {
			telemetry.Error(task+" failed", "error", err)
		},
	}
	// [freebusy_sync]: below 0 turns a job off, 0 is its default.
	if cfg.QuoteExpiryIntervalSeconds >= 0 {
		b.QuoteExpiry, b.QuoteExpiryInterval = s.Sales.QuoteExpiry(), time.Duration(cfg.QuoteExpiryIntervalSeconds)*time.Second
	}
	if cfg.ReconcileIntervalSeconds >= 0 && s.FreebusySync != nil {
		b.Reconciler, b.ReconcileInterval = s.FreebusySync.Reconciler(), time.Duration(cfg.ReconcileIntervalSeconds)*time.Second
	}
	return b
}

// Start launches the background tasks b selects, tied to ctx: the quote
// expiry job, the freebusy reconciler and the pool monitor. The goroutines
// exit when ctx is cancelled (on the server's Stop). Non-blocking.
func (s *Services) Start(ctx context.Context, b Background) {
	report := func(task string) func(error) {
		return func(err error) {
			if b.Report != nil {
				b.Report(task, err)
			}
		}
	}
	if b.QuoteExpiry != nil {
		b.QuoteExpiry.Start(ctx, b.QuoteExpiryInterval, report("quote expiry"))
	}
	if b.Reconciler != nil {
		b.Reconciler.Start(ctx, b.ReconcileInterval, report("freebusy reconciler"))
	}
	if b.PoolMonitor {
		database.StartPoolMonitor(ctx, s.Conn, 0)
	}
}
