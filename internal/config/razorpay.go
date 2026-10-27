// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package config

// razorpay.go configures Razorpay, the payment gateway for Indian cards and
// UPI (internal/gateway/razorpay): its API key, for payment links and
// refunds, and the secret its webhooks are signed with. With no key id it
// is off: gateway payments are recorded by hand, and RequestPayment is
// unimplemented.

import (
	"errors"
	"fmt"
	"net/url"
	"time"
)

// RazorpayConfig is the Razorpay account aperio takes payments through.
type RazorpayConfig struct {
	// BaseURL is the API root; https://api.razorpay.com/v1 when empty.
	BaseURL string `koanf:"base_url"`
	// KeyID is the API key id, rzp_live_... or rzp_test_...
	// APERIO_RAZORPAY__KEY_ID.
	KeyID string `koanf:"key_id"`
	// KeySecret is the API key's secret. A secret:
	// APERIO_RAZORPAY__KEY_SECRET.
	KeySecret string `koanf:"key_secret"`
	// WebhookSecret is the secret set on the webhook in Razorpay's
	// dashboard, which signs each delivery. A secret:
	// APERIO_RAZORPAY__WEBHOOK_SECRET.
	WebhookSecret string `koanf:"webhook_secret"`
	// TimeoutSeconds bounds one API call; 15 when 0.
	TimeoutSeconds int `koanf:"timeout_seconds"`
}

// Enabled reports whether Razorpay is configured.
func (c RazorpayConfig) Enabled() bool { return c.KeyID != "" }

// Timeout is one API call's bound.
func (c RazorpayConfig) Timeout() time.Duration {
	if c.TimeoutSeconds <= 0 {
		return 15 * time.Second
	}
	return time.Duration(c.TimeoutSeconds) * time.Second
}

// Validate reports the first setting an enabled Razorpay cannot run with.
func (c RazorpayConfig) Validate() error {
	if !c.Enabled() {
		return nil
	}
	if c.KeySecret == "" {
		return errors.New("config invalid: razorpay.key_secret is empty (APERIO_RAZORPAY__KEY_SECRET)")
	}
	if c.WebhookSecret == "" {
		return errors.New("config invalid: razorpay.webhook_secret is empty (APERIO_RAZORPAY__WEBHOOK_SECRET)")
	}
	if c.BaseURL != "" {
		if u, err := url.Parse(c.BaseURL); err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
			return fmt.Errorf("config invalid: razorpay.base_url=%q (want an http or https URL)", c.BaseURL)
		}
	}
	if c.TimeoutSeconds < 0 {
		return fmt.Errorf("config invalid: razorpay.timeout_seconds=%d (want 0 for the default, or more)", c.TimeoutSeconds)
	}
	return nil
}
