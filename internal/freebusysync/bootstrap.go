// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package freebusysync

// bootstrap.go makes sure freebusy sends aperio its changes: the webhook
// webhooks/{id} exists, POSTs to the configured URL, and asks for
// EventTypes. freebusy mints the signing secret and shows it only when it
// creates or rotates one, so the secret in the configuration is the one an
// operator copied from that response; when there is none, Bootstrap rotates
// to learn one, and the process verifies with it until it ends -- set it in
// the configuration (APERIO_FREEBUSY_SYNC__WEBHOOK_SECRET), as each replica
// rotating would leave the others with a secret a day from lapsing.

import (
	"context"
	"log/slog"

	"github.com/shokki-engineering/aperio/internal/config"
	"github.com/shokki-engineering/aperio/internal/engine"
)

// Subscriber is the part of engine.Client Bootstrap uses.
type Subscriber interface {
	Subscribe(ctx context.Context, id, uri string, types []string) (engine.Subscription, error)
	RotateSecret(ctx context.Context, webhook string) (string, error)
}

// Bootstrap ensures aperio's subscription in freebusy as c says, and adds to
// h any secret it learned. It does nothing without a webhook URL.
func Bootstrap(ctx context.Context, fb Subscriber, c config.FreebusySyncConfig, h *Handler) error {
	if c.WebhookURL == "" {
		return nil
	}
	id := c.WebhookID
	if id == "" {
		id = "aperio"
	}
	sub, err := fb.Subscribe(ctx, id, c.WebhookURL, EventTypes)
	if err != nil {
		return err
	}
	if sub.Paused {
		slog.WarnContext(ctx, "freebusysync: the webhook is paused in freebusy; nothing is delivered until it is resumed", "webhook", sub.Name)
	}
	if sub.Secret == "" && c.WebhookSecret == "" {
		if sub.Secret, err = fb.RotateSecret(ctx, sub.Name); err != nil {
			return err
		}
	}
	if sub.Secret != "" {
		h.AddSecret(sub.Secret)
		slog.WarnContext(ctx, "freebusysync: freebusy minted the webhook's signing secret; read it with RotateWebhookSecret "+
			"and set it as freebusy_sync.webhook_secret, or every restart rotates it", "webhook", sub.Name)
	}
	return nil
}
