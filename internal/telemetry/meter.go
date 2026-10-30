// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package telemetry

// Meter lives apart from telemetry.go: the SDK's package and runtime-go's
// metrics contract are both named telemetry, so each file imports one.
//
// With /metrics on (prometheus.go), the meter records through OTel's global
// provider -- the OTLP pipeline's and /metrics' together (fanout.go) -- so
// what the cache, notify and search measure is scraped as well as exported.

import (
	"context"

	"github.com/the-protobuf-project/runtime-go/telemetry"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// Meter is the process pipeline's meter, or one that records nothing when
// there is no pipeline.
func Meter() telemetry.Meter {
	t := Telemetry
	switch {
	case t == nil:
		return telemetry.NoopMeter
	case prometheusOn():
		return otelMeter{m: otel.GetMeterProvider().Meter(serviceName)}
	}
	return t.Meter()
}

// otelMeter is runtime-go's Meter contract over an OTel metric.Meter, as
// telemetry-go's own adapter is over its provider.
type otelMeter struct {
	m metric.Meter
}

// Counter implements telemetry.Meter.
func (a otelMeter) Counter(name string, opts ...telemetry.InstrumentOption) telemetry.Counter {
	cfg := telemetry.NewInstrumentConfig(opts...)
	c, err := a.m.Float64Counter(name, metric.WithDescription(cfg.Description), metric.WithUnit(cfg.Unit))
	if err != nil {
		return telemetry.NoopMeter.Counter(name)
	}
	return otelAdder{add: c.Add}
}

// UpDownCounter implements telemetry.Meter.
func (a otelMeter) UpDownCounter(name string, opts ...telemetry.InstrumentOption) telemetry.UpDownCounter {
	cfg := telemetry.NewInstrumentConfig(opts...)
	c, err := a.m.Float64UpDownCounter(name, metric.WithDescription(cfg.Description), metric.WithUnit(cfg.Unit))
	if err != nil {
		return telemetry.NoopMeter.UpDownCounter(name)
	}
	return otelAdder{add: c.Add}
}

// Gauge implements telemetry.Meter.
func (a otelMeter) Gauge(name string, opts ...telemetry.InstrumentOption) telemetry.Gauge {
	cfg := telemetry.NewInstrumentConfig(opts...)
	g, err := a.m.Float64Gauge(name, metric.WithDescription(cfg.Description), metric.WithUnit(cfg.Unit))
	if err != nil {
		return telemetry.NoopMeter.Gauge(name)
	}
	return otelRecorder{record: g.Record}
}

// Histogram implements telemetry.Meter.
func (a otelMeter) Histogram(name string, opts ...telemetry.InstrumentOption) telemetry.Histogram {
	cfg := telemetry.NewInstrumentConfig(opts...)
	hopts := []metric.Float64HistogramOption{metric.WithDescription(cfg.Description), metric.WithUnit(cfg.Unit)}
	if len(cfg.Buckets) > 0 {
		hopts = append(hopts, metric.WithExplicitBucketBoundaries(cfg.Buckets...))
	}
	h, err := a.m.Float64Histogram(name, hopts...)
	if err != nil {
		return telemetry.NoopMeter.Histogram(name)
	}
	return otelRecorder{record: h.Record}
}

// otelAdder is a counter or up-down counter.
type otelAdder struct {
	add func(context.Context, float64, ...metric.AddOption)
}

// Add implements telemetry.Counter and telemetry.UpDownCounter.
func (a otelAdder) Add(ctx context.Context, delta float64, labels telemetry.Labels) {
	a.add(ctx, delta, metric.WithAttributes(attributes(labels)...))
}

// otelRecorder is a gauge or a histogram.
type otelRecorder struct {
	record func(context.Context, float64, ...metric.RecordOption)
}

// Set implements telemetry.Gauge.
func (r otelRecorder) Set(ctx context.Context, value float64, labels telemetry.Labels) {
	r.record(ctx, value, metric.WithAttributes(attributes(labels)...))
}

// Record implements telemetry.Histogram.
func (r otelRecorder) Record(ctx context.Context, value float64, labels telemetry.Labels) {
	r.record(ctx, value, metric.WithAttributes(attributes(labels)...))
}

// attributes are labels as OTel attributes.
func attributes(labels telemetry.Labels) []attribute.KeyValue {
	out := make([]attribute.KeyValue, 0, len(labels))
	for k, v := range labels {
		out = append(out, attribute.String(k, v))
	}
	return out
}
