// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package database_test

// tracing_handle_test.go builds a telemetry pipeline whose spans land in
// memory, for instrument_live_test.go. It is a file of its own because the
// generated gorm telemetry package and aperio's internal/telemetry are both
// named telemetry.

import (
	"testing"

	"github.com/shokki-engineering/aperio/internal/database/gorm/telemetry"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// inMemoryTelemetry returns a tracing pipeline whose spans also reach the
// returned in-memory exporter. Without OTLP the SDK traces through the global
// tracer provider, so that is pointed at the exporter first; with it -- and
// the SDK's default OTLP host, localhost, turns it on -- the SDK installs a
// provider of its own as the global one, and the exporter is added to that.
// The handle is not closed: closing flushes to a collector that may not be
// there, which takes the exporter's whole timeout.
func inMemoryTelemetry(t *testing.T) (*telemetry.Handle, *tracetest.InMemoryExporter) {
	t.Helper()
	exporter := tracetest.NewInMemoryExporter()
	provider := trace.NewTracerProvider(trace.WithSyncer(exporter))
	previous := otel.GetTracerProvider()
	otel.SetTracerProvider(provider)
	t.Cleanup(func() {
		otel.SetTracerProvider(previous)
		_ = provider.Shutdown(t.Context())
	})
	h, err := telemetry.Build().WithService("aperio-test", "0.0.0").WithTracing().Build()
	if err != nil {
		t.Fatal(err)
	}
	if built, ok := otel.GetTracerProvider().(*trace.TracerProvider); ok && built != provider {
		built.RegisterSpanProcessor(trace.NewSimpleSpanProcessor(exporter))
	}
	return h, exporter
}
