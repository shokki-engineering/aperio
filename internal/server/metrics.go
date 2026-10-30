// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package server

// metrics.go serves the process's metrics for Prometheus at GET /metrics,
// beside the REST gateway (routes.go), when [server] metrics is on: the
// handler internal/telemetry builds (telemetry.MetricsHandler), which the
// Helm chart's ServiceMonitor scrapes on the http port.

import (
	"net/http"

	"github.com/shokki-engineering/aperio/internal/telemetry"
)

// metricsPath is where the metrics are served.
const metricsPath = "/metrics"

// useMetrics mounts /metrics when on and the telemetry pipeline serves one.
func (s *Services) useMetrics(on bool) {
	if h := telemetry.MetricsHandler(); on && h != nil {
		s.Route(http.MethodGet, metricsPath, h)
	}
}
