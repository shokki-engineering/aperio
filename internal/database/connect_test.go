// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package database

// connect_test.go checks the startup retry: it waits out a refusing engine,
// and gives up once the timeout has passed.

import (
	"context"
	"errors"
	"testing"
	"time"
)

// TestRetryConnectWaitsForTheEngine succeeds on the third attempt.
func TestRetryConnectWaitsForTheEngine(t *testing.T) {
	calls := 0
	conn, err := retryConnect(context.Background(), "http://engine", 10*time.Second, func() (*Connection, error) {
		if calls++; calls < 3 {
			return nil, errors.New("connection refused")
		}
		return &Connection{Provider: ProviderHasura}, nil
	})
	if err != nil || conn == nil || calls != 3 {
		t.Fatalf("retryConnect = %v, %v after %d calls; want a connection after 3", conn, err, calls)
	}
}

// TestRetryConnectGivesUp returns the last error once the timeout passes,
// and tries just once with none.
func TestRetryConnectGivesUp(t *testing.T) {
	refused := errors.New("connection refused")
	for _, tc := range []struct {
		timeout  time.Duration
		maxCalls int
	}{{0, 1}, {700 * time.Millisecond, 3}} {
		calls := 0
		_, err := retryConnect(context.Background(), "http://engine", tc.timeout, func() (*Connection, error) {
			calls++
			return nil, refused
		})
		if !errors.Is(err, refused) || calls > tc.maxCalls {
			t.Errorf("timeout %v: err %v after %d calls; want the refusal within %d", tc.timeout, err, calls, tc.maxCalls)
		}
	}
}
