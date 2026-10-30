// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package telemetry

// fanout_sync.go is fanoutMeter's synchronous instruments (fanout.go): each
// one records on an instrument of every provider.

import (
	"context"

	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/embedded"
)

// int64Counter is one metric.Int64Counter per provider.
type int64Counter struct {
	embedded.Int64Counter
	p []metric.Int64Counter
}

func (c int64Counter) Add(ctx context.Context, v int64, o ...metric.AddOption) {
	for _, x := range c.p {
		x.Add(ctx, v, o...)
	}
}
func (c int64Counter) Enabled(ctx context.Context) bool { return anyEnabled(ctx, c.p) }

// int64UpDownCounter is one metric.Int64UpDownCounter per provider.
type int64UpDownCounter struct {
	embedded.Int64UpDownCounter
	p []metric.Int64UpDownCounter
}

func (c int64UpDownCounter) Add(ctx context.Context, v int64, o ...metric.AddOption) {
	for _, x := range c.p {
		x.Add(ctx, v, o...)
	}
}
func (c int64UpDownCounter) Enabled(ctx context.Context) bool { return anyEnabled(ctx, c.p) }

// int64Histogram is one metric.Int64Histogram per provider.
type int64Histogram struct {
	embedded.Int64Histogram
	p []metric.Int64Histogram
}

func (c int64Histogram) Record(ctx context.Context, v int64, o ...metric.RecordOption) {
	for _, x := range c.p {
		x.Record(ctx, v, o...)
	}
}
func (c int64Histogram) Enabled(ctx context.Context) bool { return anyEnabled(ctx, c.p) }

// int64Gauge is one metric.Int64Gauge per provider.
type int64Gauge struct {
	embedded.Int64Gauge
	p []metric.Int64Gauge
}

func (c int64Gauge) Record(ctx context.Context, v int64, o ...metric.RecordOption) {
	for _, x := range c.p {
		x.Record(ctx, v, o...)
	}
}
func (c int64Gauge) Enabled(ctx context.Context) bool { return anyEnabled(ctx, c.p) }

// float64Counter is one metric.Float64Counter per provider.
type float64Counter struct {
	embedded.Float64Counter
	p []metric.Float64Counter
}

func (c float64Counter) Add(ctx context.Context, v float64, o ...metric.AddOption) {
	for _, x := range c.p {
		x.Add(ctx, v, o...)
	}
}
func (c float64Counter) Enabled(ctx context.Context) bool { return anyEnabled(ctx, c.p) }

// float64UpDownCounter is one metric.Float64UpDownCounter per provider.
type float64UpDownCounter struct {
	embedded.Float64UpDownCounter
	p []metric.Float64UpDownCounter
}

func (c float64UpDownCounter) Add(ctx context.Context, v float64, o ...metric.AddOption) {
	for _, x := range c.p {
		x.Add(ctx, v, o...)
	}
}
func (c float64UpDownCounter) Enabled(ctx context.Context) bool { return anyEnabled(ctx, c.p) }

// float64Histogram is one metric.Float64Histogram per provider.
type float64Histogram struct {
	embedded.Float64Histogram
	p []metric.Float64Histogram
}

func (c float64Histogram) Record(ctx context.Context, v float64, o ...metric.RecordOption) {
	for _, x := range c.p {
		x.Record(ctx, v, o...)
	}
}
func (c float64Histogram) Enabled(ctx context.Context) bool { return anyEnabled(ctx, c.p) }

// float64Gauge is one metric.Float64Gauge per provider.
type float64Gauge struct {
	embedded.Float64Gauge
	p []metric.Float64Gauge
}

func (c float64Gauge) Record(ctx context.Context, v float64, o ...metric.RecordOption) {
	for _, x := range c.p {
		x.Record(ctx, v, o...)
	}
}
func (c float64Gauge) Enabled(ctx context.Context) bool { return anyEnabled(ctx, c.p) }

// The constructors: one instrument per provider, behind one that records on all.

// Int64Counter implements metric.Meter.
func (m fanoutMeter) Int64Counter(name string, o ...metric.Int64CounterOption) (metric.Int64Counter, error) {
	p, err := fan(m.meters, func(x metric.Meter) (metric.Int64Counter, error) { return x.Int64Counter(name, o...) })
	return int64Counter{p: p}, err
}

// Int64UpDownCounter implements metric.Meter.
func (m fanoutMeter) Int64UpDownCounter(name string, o ...metric.Int64UpDownCounterOption) (metric.Int64UpDownCounter, error) {
	p, err := fan(m.meters, func(x metric.Meter) (metric.Int64UpDownCounter, error) { return x.Int64UpDownCounter(name, o...) })
	return int64UpDownCounter{p: p}, err
}

// Int64Histogram implements metric.Meter.
func (m fanoutMeter) Int64Histogram(name string, o ...metric.Int64HistogramOption) (metric.Int64Histogram, error) {
	p, err := fan(m.meters, func(x metric.Meter) (metric.Int64Histogram, error) { return x.Int64Histogram(name, o...) })
	return int64Histogram{p: p}, err
}

// Int64Gauge implements metric.Meter.
func (m fanoutMeter) Int64Gauge(name string, o ...metric.Int64GaugeOption) (metric.Int64Gauge, error) {
	p, err := fan(m.meters, func(x metric.Meter) (metric.Int64Gauge, error) { return x.Int64Gauge(name, o...) })
	return int64Gauge{p: p}, err
}

// Float64Counter implements metric.Meter.
func (m fanoutMeter) Float64Counter(name string, o ...metric.Float64CounterOption) (metric.Float64Counter, error) {
	p, err := fan(m.meters, func(x metric.Meter) (metric.Float64Counter, error) { return x.Float64Counter(name, o...) })
	return float64Counter{p: p}, err
}

// Float64UpDownCounter implements metric.Meter.
func (m fanoutMeter) Float64UpDownCounter(name string, o ...metric.Float64UpDownCounterOption) (metric.Float64UpDownCounter, error) {
	p, err := fan(m.meters, func(x metric.Meter) (metric.Float64UpDownCounter, error) { return x.Float64UpDownCounter(name, o...) })
	return float64UpDownCounter{p: p}, err
}

// Float64Histogram implements metric.Meter.
func (m fanoutMeter) Float64Histogram(name string, o ...metric.Float64HistogramOption) (metric.Float64Histogram, error) {
	p, err := fan(m.meters, func(x metric.Meter) (metric.Float64Histogram, error) { return x.Float64Histogram(name, o...) })
	return float64Histogram{p: p}, err
}

// Float64Gauge implements metric.Meter.
func (m fanoutMeter) Float64Gauge(name string, o ...metric.Float64GaugeOption) (metric.Float64Gauge, error) {
	p, err := fan(m.meters, func(x metric.Meter) (metric.Float64Gauge, error) { return x.Float64Gauge(name, o...) })
	return float64Gauge{p: p}, err
}
