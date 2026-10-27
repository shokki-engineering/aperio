// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package config

// notify.go configures notifications (internal/notify): who a venue's
// departments are told through when a service request is made, and the guest
// when it is done. One provider at a time, chosen here, so no vendor is built
// in: "log" (the default) writes each message to the log; "webhook" posts it,
// signed, to a URL of the venue's choosing (a pantry screen, a chat bridge,
// an automation); "whatsapp" sends it through the WhatsApp Business Cloud API.

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// Notification providers ([notify] provider).
const (
	NotifyLog      = "log"
	NotifyWebhook  = "webhook"
	NotifyWhatsApp = "whatsapp"
)

// NotifyConfig is how notifications are sent, and to whom.
type NotifyConfig struct {
	// Provider is "log" (the default), "webhook" or "whatsapp".
	// APERIO_NOTIFY__PROVIDER.
	Provider string `koanf:"provider"`
	// Departments maps a department -- pantry, kitchen, housekeeping,
	// maintenance, it_av, front_desk, banquet, security -- to where it is
	// told: a phone in E.164 for WhatsApp, a recipient key the webhook's
	// receiver routes by. "default" catches a department not listed; a
	// department with neither is not told. APERIO_NOTIFY__DEPARTMENTS__PANTRY.
	Departments map[string]string `koanf:"departments"`
	// TimeoutSeconds bounds one delivery; 10 when 0.
	TimeoutSeconds int `koanf:"timeout_seconds"`
	// Webhook is provider "webhook"'s endpoint.
	Webhook WebhookConfig `koanf:"webhook"`
	// WhatsApp is provider "whatsapp"'s account.
	WhatsApp WhatsAppConfig `koanf:"whatsapp"`
}

// WebhookConfig is where a signed webhook posts.
type WebhookConfig struct {
	// URL receives each message as a JSON POST.
	URL string `koanf:"url"`
	// Secret keys the HMAC-SHA256 signature of the body in the
	// X-Aperio-Signature header. A secret: APERIO_NOTIFY__WEBHOOK__SECRET.
	Secret string `koanf:"secret"`
}

// WhatsAppConfig is a WhatsApp Business Cloud API sender.
type WhatsAppConfig struct {
	// APIURL is the Graph API's versioned root;
	// https://graph.facebook.com/v21.0 when empty.
	APIURL string `koanf:"api_url"`
	// PhoneNumberID is the business number's id, as Meta's console shows it.
	PhoneNumberID string `koanf:"phone_number_id"`
	// Token is the system user's access token. A secret:
	// APERIO_NOTIFY__WHATSAPP__TOKEN.
	Token string `koanf:"token"`
	// Template is an approved template with one body parameter, the
	// message's text; empty sends free text, which WhatsApp delivers only
	// within 24 hours of the recipient's last message.
	Template string `koanf:"template"`
	// Language is the template's language code; "en" when empty.
	Language string `koanf:"language"`
}

// Timeout is one delivery's bound.
func (c NotifyConfig) Timeout() time.Duration {
	if c.TimeoutSeconds <= 0 {
		return 10 * time.Second
	}
	return time.Duration(c.TimeoutSeconds) * time.Second
}

// Validate reports the first setting the provider cannot run with.
func (c NotifyConfig) Validate() error {
	switch c.Provider {
	case "", NotifyLog:
	case NotifyWebhook:
		if u, err := url.Parse(c.Webhook.URL); err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
			return fmt.Errorf("config invalid: notify.webhook.url=%q (want an http or https URL)", c.Webhook.URL)
		}
		if len(c.Webhook.Secret) < 32 {
			return errors.New("config invalid: notify.webhook.secret needs 32 or more bytes (APERIO_NOTIFY__WEBHOOK__SECRET)")
		}
	case NotifyWhatsApp:
		if c.WhatsApp.PhoneNumberID == "" || c.WhatsApp.Token == "" {
			return errors.New("config invalid: notify.whatsapp needs phone_number_id and token (APERIO_NOTIFY__WHATSAPP__TOKEN)")
		}
	default:
		return fmt.Errorf("config invalid: notify.provider=%q (want log|webhook|whatsapp)", c.Provider)
	}
	if c.TimeoutSeconds < 0 {
		return fmt.Errorf("config invalid: notify.timeout_seconds=%d (want 0 for the default, or more)", c.TimeoutSeconds)
	}
	for k := range c.Departments {
		if k != strings.ToLower(k) {
			return fmt.Errorf("config invalid: notify.departments.%s (want lower-case department names)", k)
		}
	}
	return nil
}
