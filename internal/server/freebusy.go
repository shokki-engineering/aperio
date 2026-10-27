// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package server

// freebusy.go keeps aperio in step with what changes in freebusy outside it
// (internal/freebusysync): freebusy's webhooks, served beside the REST
// gateway at /webhooks/freebusy, and the subscription that sends them,
// ensured as the server starts. A failed subscription is logged, never
// fatal: the reconciler (background.go) still finds the drift.

import (
	"context"
	"net/http"

	"github.com/shokki-engineering/aperio/internal/config"
	"github.com/shokki-engineering/aperio/internal/engine"
	"github.com/shokki-engineering/aperio/internal/freebusysync"
	"github.com/shokki-engineering/aperio/internal/telemetry"
)

// freebusyPath is where freebusy delivers its webhooks.
const freebusyPath = "/webhooks/freebusy"

// useFreebusySync serves freebusy's webhooks and ensures their subscription.
func (s *Services) useFreebusySync(ctx context.Context, fb *engine.Client, cfg config.FreebusySyncConfig) {
	hook := freebusysync.NewHandler(s.FreebusySync, cfg.WebhookSecret)
	s.Route(http.MethodPost, freebusyPath, hook)
	if err := freebusysync.Bootstrap(ctx, fb, cfg, hook); err != nil {
		telemetry.Error("freebusy webhook subscription not ensured", "error", err)
	}
}
