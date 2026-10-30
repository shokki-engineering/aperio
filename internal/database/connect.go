// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package database

// connect.go waits for the Hasura DDN engine at startup. OpenHasura sends a
// connectivity query, so an engine still starting beside aperio -- in
// Compose, or a pod that came up first -- refused the connection and the
// server exited, recovering only on a restart. connectHasura retries with
// backoff instead, for as long as [database.hasura] connect_timeout_seconds
// allows, logging each wait.

import (
	"context"
	"time"

	"github.com/shokki-engineering/aperio/internal/config"
	"github.com/shokki-engineering/aperio/internal/telemetry"
)

// The backoff between attempts: doubling from firstRetry, capped at maxRetry.
const (
	firstRetry = 500 * time.Millisecond
	maxRetry   = 5 * time.Second
)

// connectHasura opens the engine h names (OpenHasura), retrying until it
// answers or h.ConnectTimeout() has passed; the last error is returned then.
func connectHasura(ctx context.Context, h config.HasuraConfig) (*Connection, error) {
	return retryConnect(ctx, h.URL, h.ConnectTimeout(), func() (*Connection, error) {
		return OpenHasura(h.URL, h.AdminSecret)
	})
}

// retryConnect calls open until it succeeds, ctx ends or timeout has passed
// since the first attempt; a timeout of 0 tries once.
func retryConnect(ctx context.Context, url string, timeout time.Duration, open func() (*Connection, error)) (*Connection, error) {
	deadline := time.Now().Add(timeout)
	wait := firstRetry
	for attempt := 1; ; attempt++ {
		conn, err := open()
		if err == nil {
			if attempt > 1 {
				telemetry.Info("hasura engine is up", "url", url, "attempts", attempt)
			}
			return conn, nil
		}
		left := time.Until(deadline)
		if left <= 0 {
			return nil, err
		}
		wait = min(wait, left)
		telemetry.Warn("waiting for the hasura engine", "url", url, "attempt", attempt, "retry_in", wait, "err", err)
		select {
		case <-ctx.Done():
			return nil, err
		case <-time.After(wait):
		}
		wait = min(wait*2, maxRetry)
	}
}
