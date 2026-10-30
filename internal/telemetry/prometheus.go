// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package telemetry

// prometheus.go serves the process's metrics for scraping: a MeterProvider
// whose reader is OTel's Prometheus exporter, in the registry registry.go
// makes, which the server mounts at /metrics beside the REST gateway. The
// OTLP pipeline stays as it was: telemetry-go's provider takes no second
// reader, so OTel's global provider becomes both (fanout.go).

import (
	"context"
	"fmt"
	"net/http"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
)

var (
	// scrape is /metrics' handler and provider its MeterProvider; both nil
	// until startPrometheus.
	scrape   http.Handler
	provider *metric.MeterProvider
)

// startPrometheus builds the scrape provider, labelled with the service's
// name and version, and makes OTel's global provider record on it -- beside
// telemetry-go's OTLP provider when that is the global one.
func startPrometheus(name, version string) error {
	reg := newRegistry()
	exporter, err := prometheus.New(prometheus.WithRegisterer(reg))
	if err != nil {
		return fmt.Errorf("prometheus exporter: %w", err)
	}
	res, err := resource.Merge(resource.Default(), resource.NewSchemaless(
		attribute.String("service.name", name), attribute.String("service.version", version)))
	if err != nil {
		return fmt.Errorf("prometheus resource: %w", err)
	}
	provider = metric.NewMeterProvider(metric.WithReader(exporter), metric.WithResource(res))
	// Only an SDK provider joins the fan-out: OTel's default global is a
	// delegate that would forward to the fan-out itself, a loop.
	if otlp, ok := otel.GetMeterProvider().(*metric.MeterProvider); ok {
		otel.SetMeterProvider(fanout(otlp, provider))
	} else {
		otel.SetMeterProvider(provider)
	}
	scrape = handlerFor(reg)
	return nil
}

// prometheusOn reports whether Init started the scrape provider.
func prometheusOn() bool {
	return provider != nil
}

// MetricsHandler is what the server serves at /metrics: nil unless Init was
// asked for it (Settings.Prometheus).
func MetricsHandler() http.Handler {
	return scrape
}

// stopPrometheus shuts the scrape provider down, if there is one.
func stopPrometheus() error {
	if provider == nil {
		return nil
	}
	return provider.Shutdown(context.Background())
}
