// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package config

// freebusy_sync.go configures how aperio stays in step with freebusy
// (internal/freebusysync): the webhook freebusy sends its changes to, and
// how often the reconciler and the quote expiry job run.

// FreebusySyncConfig is [freebusy_sync].
type FreebusySyncConfig struct {
	// WebhookURL is the public URL freebusy POSTs changes to: aperio's REST
	// port and /webhooks/freebusy. Empty, no subscription is made (the
	// reconciler still runs).
	WebhookURL string `koanf:"webhook_url"`
	// WebhookID names the subscription in freebusy, webhooks/{id}; "aperio"
	// when empty.
	WebhookID string `koanf:"webhook_id"`
	// WebhookSecret is the subscription's whsec_ signing secret, as
	// freebusy's CreateWebhook or RotateWebhookSecret returned it. A secret:
	// set it through APERIO_FREEBUSY_SYNC__WEBHOOK_SECRET, not a file.
	WebhookSecret string `koanf:"webhook_secret"`
	// ReconcileIntervalSeconds is how often the reconciler walks the live
	// bookings; 0 is five minutes, below 0 turns it off.
	ReconcileIntervalSeconds int `koanf:"reconcile_interval_seconds"`
	// QuoteExpiryIntervalSeconds is how often sent quotes past their
	// expire_time are expired; 0 is a minute, below 0 turns it off.
	QuoteExpiryIntervalSeconds int `koanf:"quote_expiry_interval_seconds"`
}
