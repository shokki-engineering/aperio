// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package telemetry

// registry.go is the Prometheus registry /metrics serves. It lives apart from
// prometheus.go: client_golang's package and OTel's exporter are both named
// prometheus, so each file imports one.

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// newRegistry is a registry with the Go runtime's and the process's
// collectors; prometheus.go registers the OTel exporter in it.
func newRegistry() *prometheus.Registry {
	reg := prometheus.NewRegistry()
	reg.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)
	return reg
}

// handlerFor serves reg in the Prometheus text (or OpenMetrics) format.
func handlerFor(reg *prometheus.Registry) http.Handler {
	return promhttp.HandlerFor(reg, promhttp.HandlerOpts{Registry: reg, EnableOpenMetrics: true})
}
