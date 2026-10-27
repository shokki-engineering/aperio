// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package razorpay is aperio's adapter for Razorpay, the gateway for Indian
// cards and UPI, behind finance's Gateway and PaymentLinker:
//
//   - Link makes a payment link (links.go) -- the amount in paise, the
//     payer, an expiry, our request id as its reference_id, Razorpay's own
//     SMS and email off -- and Refund refunds through the Refunds API
//     (refunds.go);
//   - Webhook is the http.Handler Razorpay's webhooks are delivered to
//     (webhook.go): it checks X-Razorpay-Signature and hands finance's Settle
//     what happened.
//
// Every call authenticates with the key id and secret (HTTP basic auth);
// neither, nor the webhook secret, is ever logged or put in an error.
package razorpay

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// DefaultBaseURL is Razorpay's API root.
const DefaultBaseURL = "https://api.razorpay.com/v1"

// name is the gateway as aperio's payments name it.
const name = "razorpay"

// Options configure a Client.
type Options struct {
	BaseURL   string        // DefaultBaseURL when empty
	KeyID     string        // the API key id
	KeySecret string        // the API key secret
	Timeout   time.Duration // one call's bound; 15s when 0
	HTTP      *http.Client  // nil: a client with Timeout
}

// Client calls Razorpay's API.
type Client struct {
	base, keyID, keySecret string
	http                   *http.Client
}

// New returns a Client over o.
func New(o Options) *Client {
	c := &Client{base: strings.TrimRight(o.BaseURL, "/"), keyID: o.KeyID, keySecret: o.KeySecret, http: o.HTTP}
	if c.base == "" {
		c.base = DefaultBaseURL
	}
	if c.http == nil {
		t := o.Timeout
		if t <= 0 {
			t = 15 * time.Second
		}
		c.http = &http.Client{Timeout: t}
	}
	return c
}

// Name is "razorpay".
func (c *Client) Name() string { return name }

// apiError is a non-2xx answer from Razorpay.
type apiError struct {
	Status      int
	Code        string
	Description string
}

func (e *apiError) Error() string {
	return fmt.Sprintf("razorpay: %d %s: %s", e.Status, e.Code, e.Description)
}

// do sends body (nil: none) to method path and decodes the answer into out.
func (c *Client) do(ctx context.Context, method, path string, body, out any) error {
	var r io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		r = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.base+path, r)
	if err != nil {
		return err
	}
	req.SetBasicAuth(c.keyID, c.keySecret)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("razorpay: %s %s: %w", method, path, err)
	}
	defer func() { _ = res.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return err
	}
	if res.StatusCode/100 != 2 {
		var e struct {
			Error struct {
				Code        string `json:"code"`
				Description string `json:"description"`
			} `json:"error"`
		}
		_ = json.Unmarshal(raw, &e)
		return &apiError{Status: res.StatusCode, Code: e.Error.Code, Description: e.Error.Description}
	}
	return json.Unmarshal(raw, out)
}
