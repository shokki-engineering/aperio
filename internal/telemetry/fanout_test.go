// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package telemetry

// fanout_test.go checks that the fan-out provider records synchronous and
// observed measurements on every provider.

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel/metric"
)

// TestFanoutRecordsOnEveryProvider adds to a counter and observes a gauge
// through a callback, and reads both back from each provider.
func TestFanoutRecordsOnEveryProvider(t *testing.T) {
	a, readA := sdkProvider(t)
	b, readB := sdkProvider(t)
	m := fanout(a, b).Meter("test")
	ctx := context.Background()

	c, err := m.Int64Counter("calls")
	if err != nil {
		t.Fatal(err)
	}
	c.Add(ctx, 3)
	g, err := m.Int64ObservableGauge("depth")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.RegisterCallback(func(_ context.Context, o metric.Observer) error {
		o.ObserveInt64(g, 7)
		return nil
	}, g); err != nil {
		t.Fatal(err)
	}
	for name, read := range map[string]func(string) int64{"a": readA, "b": readB} {
		if got := read("calls"); got != 3 {
			t.Errorf("provider %s: calls = %d, want 3", name, got)
		}
		if got := read("depth"); got != 7 {
			t.Errorf("provider %s: depth = %d, want 7", name, got)
		}
	}
}
