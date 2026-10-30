// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package telemetry

// fanout.go is an OpenTelemetry MeterProvider that records every measurement
// on several providers at once. telemetry-go builds its OTLP MeterProvider
// itself, with no way to add a reader to it, so /metrics gets a provider of
// its own (prometheus.go) and the process's global provider becomes both:
// what instruments through OTel's global -- the gRPC server's metrics, its
// otelgrpc spans' metrics -- reaches the OTLP collector and /metrics alike.
// The instruments themselves are in fanout_sync.go and fanout_async.go.

import (
	"context"
	"errors"

	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/embedded"
)

// fanout is one provider that records on every one of providers.
func fanout(providers ...metric.MeterProvider) metric.MeterProvider {
	return fanoutProvider{providers: providers}
}

// fanoutProvider is a metric.MeterProvider over providers.
type fanoutProvider struct {
	embedded.MeterProvider
	providers []metric.MeterProvider
}

// Meter implements metric.MeterProvider: the named meter of every provider.
func (p fanoutProvider) Meter(name string, opts ...metric.MeterOption) metric.Meter {
	m := fanoutMeter{meters: make([]metric.Meter, len(p.providers))}
	for i, provider := range p.providers {
		m.meters[i] = provider.Meter(name, opts...)
	}
	return m
}

// fanoutMeter is a metric.Meter over meters, one per provider.
type fanoutMeter struct {
	embedded.Meter
	meters []metric.Meter
}

// fan makes one instrument on each meter with create. An SDK returns a usable
// instrument beside an error (an invalid name, say), so every instrument is
// kept and the errors are joined.
func fan[T any](meters []metric.Meter, create func(metric.Meter) (T, error)) ([]T, error) {
	out := make([]T, len(meters))
	errs := make([]error, len(meters))
	for i, m := range meters {
		out[i], errs[i] = create(m)
	}
	return out, errors.Join(errs...)
}

// anyEnabled reports whether any of parts records anything.
func anyEnabled[T interface{ Enabled(context.Context) bool }](ctx context.Context, parts []T) bool {
	for _, p := range parts {
		if p.Enabled(ctx) {
			return true
		}
	}
	return false
}

// fanned is an asynchronous instrument of this package: one per provider.
type fanned interface {
	part(i int) metric.Observable
}

// RegisterCallback implements metric.Meter: f is registered with every
// provider, observing that provider's own instruments.
func (m fanoutMeter) RegisterCallback(f metric.Callback, instruments ...metric.Observable) (metric.Registration, error) {
	reg := fanoutRegistration{}
	for i, meter := range m.meters {
		parts := make([]metric.Observable, len(instruments))
		for j, inst := range instruments {
			parts[j] = inst
			if fi, ok := inst.(fanned); ok {
				parts[j] = fi.part(i)
			}
		}
		r, err := meter.RegisterCallback(func(ctx context.Context, o metric.Observer) error {
			return f(ctx, fanoutObserver{Observer: o, index: i})
		}, parts...)
		if err != nil {
			return nil, errors.Join(err, reg.Unregister())
		}
		reg.regs = append(reg.regs, r)
	}
	return reg, nil
}

// fanoutObserver is one provider's metric.Observer, given this package's
// instruments: each observation goes to that provider's part.
type fanoutObserver struct {
	metric.Observer
	index int
}

// ObserveInt64 implements metric.Observer.
func (o fanoutObserver) ObserveInt64(inst metric.Int64Observable, v int64, opts ...metric.ObserveOption) {
	if fi, ok := inst.(fanned); ok {
		inst = fi.part(o.index).(metric.Int64Observable)
	}
	o.Observer.ObserveInt64(inst, v, opts...)
}

// ObserveFloat64 implements metric.Observer.
func (o fanoutObserver) ObserveFloat64(inst metric.Float64Observable, v float64, opts ...metric.ObserveOption) {
	if fi, ok := inst.(fanned); ok {
		inst = fi.part(o.index).(metric.Float64Observable)
	}
	o.Observer.ObserveFloat64(inst, v, opts...)
}

// fanoutRegistration is a callback's registration with every provider.
type fanoutRegistration struct {
	embedded.Registration
	regs []metric.Registration
}

// Unregister implements metric.Registration.
func (r fanoutRegistration) Unregister() error {
	var errs []error
	for _, reg := range r.regs {
		errs = append(errs, reg.Unregister())
	}
	return errors.Join(errs...)
}
