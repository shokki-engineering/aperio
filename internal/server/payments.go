// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package server

// payments.go connects the payment gateway [razorpay] names
// (internal/gateway/razorpay): finance refunds through it and asks guests to
// pay through its payment links, and its webhook -- served beside the REST
// gateway at /webhooks/razorpay -- settles what they paid. Without key
// credentials there is no gateway: payments are recorded by hand and
// RequestPayment answers Unimplemented.

import (
	"net/http"

	"github.com/shokki-engineering/aperio/internal/config"
	"github.com/shokki-engineering/aperio/internal/gateway/razorpay"
	"github.com/shokki-engineering/aperio/internal/service/finance"
)

// razorpayPath is where Razorpay delivers its webhooks.
const razorpayPath = "/webhooks/razorpay"

// gateways are the payment gateways cfg configures, and the Razorpay client
// among them (nil when it is off).
func gateways(cfg config.RazorpayConfig) ([]finance.Gateway, *razorpay.Client) {
	if !cfg.Enabled() {
		return nil, nil
	}
	c := razorpay.New(razorpay.Options{BaseURL: cfg.BaseURL, KeyID: cfg.KeyID, KeySecret: cfg.KeySecret, Timeout: cfg.Timeout()})
	return []finance.Gateway{c}, c
}

// useRazorpay has finance make payment links through c and serves its
// webhook, signed with secret; nothing when c is nil.
func (s *Services) useRazorpay(c *razorpay.Client, secret string) {
	if c == nil {
		return
	}
	s.Finance.LinkPaymentsWith(c)
	s.Route(http.MethodPost, razorpayPath, razorpay.NewWebhook(secret, s.Finance))
}
