// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package telemetry

// readers_test.go makes the SDK providers fanout_test.go records on. It
// imports the SDK's metric package, whose name the API's shares, so the
// tests themselves live in a file of their own.

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

// sdkProvider is an SDK provider read by hand, and a func returning the
// sum of every data point named name it has collected.
func sdkProvider(t *testing.T) (*metric.MeterProvider, func(name string) int64) {
	t.Helper()
	reader := metric.NewManualReader()
	p := metric.NewMeterProvider(metric.WithReader(reader))
	return p, func(name string) int64 {
		var rm metricdata.ResourceMetrics
		if err := reader.Collect(context.Background(), &rm); err != nil {
			t.Fatal(err)
		}
		var total int64
		for _, sm := range rm.ScopeMetrics {
			for _, m := range sm.Metrics {
				if m.Name != name {
					continue
				}
				switch d := m.Data.(type) {
				case metricdata.Sum[int64]:
					for _, dp := range d.DataPoints {
						total += dp.Value
					}
				case metricdata.Gauge[int64]:
					for _, dp := range d.DataPoints {
						total += dp.Value
					}
				}
			}
		}
		return total
	}
}
