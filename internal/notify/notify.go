// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package notify tells people things -- a department that a request is
// waiting, a guest that it is done -- through a Notifier the configuration
// chooses ([notify] provider), so no vendor is built in:
//
//   - Log writes each message to the log; the default, and what a venue
//     without a channel yet runs.
//   - Webhook posts each message as JSON to a URL, signed with HMAC-SHA256
//     over the body (webhook.go), for a pantry screen, a chat bridge or an
//     automation to route.
//   - WhatsApp sends each one through the WhatsApp Business Cloud API, as a
//     template or as free text (whatsapp.go).
//
// A Sender (sender.go) delivers in the background and never fails its
// caller: a message that cannot be delivered is logged and counted, and the
// request it is about stands.
package notify

import (
	"context"
	"fmt"
	"net/http"

	"github.com/shokki-engineering/aperio/internal/config"
)

// Message is one notification.
type Message struct {
	// To is where it goes: a phone in E.164 for WhatsApp, a recipient key
	// the webhook's receiver routes by.
	To string `json:"to"`
	// Event is what happened, e.g. "aperio.operations.service_request.created".
	Event string `json:"event"`
	// Subject is the resource it is about, e.g.
	// "venues/v/serviceRequests/r".
	Subject string `json:"subject"`
	// Text is the message, as a person reads it.
	Text string `json:"text"`
}

// Notifier delivers a message, or says why it could not.
type Notifier interface {
	Notify(ctx context.Context, m Message) error
}

// Log is the Notifier that writes each message to the log.
type Log struct {
	// Print logs a line with key/value pairs, e.g. telemetry.Info.
	Print func(msg string, kv ...any)
}

// Notify logs m.
func (l Log) Notify(_ context.Context, m Message) error {
	if l.Print != nil {
		l.Print("notification", "to", m.To, "event", m.Event, "subject", m.Subject, "text", m.Text)
	}
	return nil
}

// New is the Notifier cfg chooses; print is where Log writes.
func New(cfg config.NotifyConfig, print func(msg string, kv ...any)) (Notifier, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	client := &http.Client{Timeout: cfg.Timeout()}
	switch cfg.Provider {
	case config.NotifyWebhook:
		return &Webhook{URL: cfg.Webhook.URL, Secret: []byte(cfg.Webhook.Secret), Client: client}, nil
	case config.NotifyWhatsApp:
		w := cfg.WhatsApp
		return &WhatsApp{APIURL: w.APIURL, PhoneNumberID: w.PhoneNumberID, Token: w.Token,
			Template: w.Template, Language: w.Language, Client: client}, nil
	case "", config.NotifyLog:
		return Log{Print: print}, nil
	}
	return nil, fmt.Errorf("notify: provider %q", cfg.Provider)
}

// post sends req through client and fails on a status that is not 2xx.
func post(client *http.Client, req *http.Request, who string) error {
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("notify: %s: %w", who, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("notify: %s: %s", who, resp.Status)
	}
	return nil
}
