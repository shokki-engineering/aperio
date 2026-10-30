// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package telemetry

// fanout_async.go is fanoutMeter's asynchronous instruments (fanout.go). Each
// embeds its first provider's instrument, which carries the interface's
// unexported methods, and hands RegisterCallback and the observer the
// instrument of the provider being collected (part).

import "go.opentelemetry.io/otel/metric"

// int64ObservableCounter is one metric.Int64ObservableCounter per provider.
type int64ObservableCounter struct {
	metric.Int64ObservableCounter
	p []metric.Int64ObservableCounter
}

func (o int64ObservableCounter) part(i int) metric.Observable { return o.p[i] }

// Int64ObservableCounter implements metric.Meter. Callbacks among o run on every provider.
func (m fanoutMeter) Int64ObservableCounter(name string, o ...metric.Int64ObservableCounterOption) (metric.Int64ObservableCounter, error) {
	p, err := fan(m.meters, func(x metric.Meter) (metric.Int64ObservableCounter, error) {
		return x.Int64ObservableCounter(name, o...)
	})
	return int64ObservableCounter{Int64ObservableCounter: p[0], p: p}, err
}

// int64ObservableUpDownCounter is one metric.Int64ObservableUpDownCounter per provider.
type int64ObservableUpDownCounter struct {
	metric.Int64ObservableUpDownCounter
	p []metric.Int64ObservableUpDownCounter
}

func (o int64ObservableUpDownCounter) part(i int) metric.Observable { return o.p[i] }

// Int64ObservableUpDownCounter implements metric.Meter. Callbacks among o run on every provider.
func (m fanoutMeter) Int64ObservableUpDownCounter(name string, o ...metric.Int64ObservableUpDownCounterOption) (metric.Int64ObservableUpDownCounter, error) {
	p, err := fan(m.meters, func(x metric.Meter) (metric.Int64ObservableUpDownCounter, error) {
		return x.Int64ObservableUpDownCounter(name, o...)
	})
	return int64ObservableUpDownCounter{Int64ObservableUpDownCounter: p[0], p: p}, err
}

// int64ObservableGauge is one metric.Int64ObservableGauge per provider.
type int64ObservableGauge struct {
	metric.Int64ObservableGauge
	p []metric.Int64ObservableGauge
}

func (o int64ObservableGauge) part(i int) metric.Observable { return o.p[i] }

// Int64ObservableGauge implements metric.Meter. Callbacks among o run on every provider.
func (m fanoutMeter) Int64ObservableGauge(name string, o ...metric.Int64ObservableGaugeOption) (metric.Int64ObservableGauge, error) {
	p, err := fan(m.meters, func(x metric.Meter) (metric.Int64ObservableGauge, error) { return x.Int64ObservableGauge(name, o...) })
	return int64ObservableGauge{Int64ObservableGauge: p[0], p: p}, err
}

// float64ObservableCounter is one metric.Float64ObservableCounter per provider.
type float64ObservableCounter struct {
	metric.Float64ObservableCounter
	p []metric.Float64ObservableCounter
}

func (o float64ObservableCounter) part(i int) metric.Observable { return o.p[i] }

// Float64ObservableCounter implements metric.Meter. Callbacks among o run on every provider.
func (m fanoutMeter) Float64ObservableCounter(name string, o ...metric.Float64ObservableCounterOption) (metric.Float64ObservableCounter, error) {
	p, err := fan(m.meters, func(x metric.Meter) (metric.Float64ObservableCounter, error) {
		return x.Float64ObservableCounter(name, o...)
	})
	return float64ObservableCounter{Float64ObservableCounter: p[0], p: p}, err
}

// float64ObservableUpDownCounter is one metric.Float64ObservableUpDownCounter per provider.
type float64ObservableUpDownCounter struct {
	metric.Float64ObservableUpDownCounter
	p []metric.Float64ObservableUpDownCounter
}

func (o float64ObservableUpDownCounter) part(i int) metric.Observable { return o.p[i] }

// Float64ObservableUpDownCounter implements metric.Meter. Callbacks among o run on every provider.
func (m fanoutMeter) Float64ObservableUpDownCounter(name string, o ...metric.Float64ObservableUpDownCounterOption) (metric.Float64ObservableUpDownCounter, error) {
	p, err := fan(m.meters, func(x metric.Meter) (metric.Float64ObservableUpDownCounter, error) {
		return x.Float64ObservableUpDownCounter(name, o...)
	})
	return float64ObservableUpDownCounter{Float64ObservableUpDownCounter: p[0], p: p}, err
}

// float64ObservableGauge is one metric.Float64ObservableGauge per provider.
type float64ObservableGauge struct {
	metric.Float64ObservableGauge
	p []metric.Float64ObservableGauge
}

func (o float64ObservableGauge) part(i int) metric.Observable { return o.p[i] }

// Float64ObservableGauge implements metric.Meter. Callbacks among o run on every provider.
func (m fanoutMeter) Float64ObservableGauge(name string, o ...metric.Float64ObservableGaugeOption) (metric.Float64ObservableGauge, error) {
	p, err := fan(m.meters, func(x metric.Meter) (metric.Float64ObservableGauge, error) {
		return x.Float64ObservableGauge(name, o...)
	})
	return float64ObservableGauge{Float64ObservableGauge: p[0], p: p}, err
}
