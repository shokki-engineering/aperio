// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package notify

// whatsapp.go is the WhatsApp Business Cloud API: each message is sent from
// the venue's business number to To, a phone, by POST
// {api}/{phone_number_id}/messages with the system user's bearer token.
// With a Template, it is that approved template with the text as its one
// body parameter -- what WhatsApp delivers at any time; without, it is free
// text, which WhatsApp delivers only within 24 hours of the recipient's last
// message to the business: right for staff who keep the chat open, not for a
// guest who never wrote.

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
)

// DefaultWhatsAppAPI is the Graph API root a WhatsApp sends through when its
// APIURL is empty.
const DefaultWhatsAppAPI = "https://graph.facebook.com/v21.0"

// WhatsApp is the Notifier that sends each message on WhatsApp.
type WhatsApp struct {
	// APIURL is the Graph API's versioned root; DefaultWhatsAppAPI when empty.
	APIURL string
	// PhoneNumberID is the business number's id.
	PhoneNumberID string
	// Token is the access token.
	Token string
	// Template is the approved template to send, with the text as its one
	// body parameter; free text when empty.
	Template string
	// Language is the template's language code; "en" when empty.
	Language string
	// Client sends them; http.DefaultClient when nil.
	Client *http.Client
}

// whatsAppMessage is the Cloud API's message body, of type "text" or
// "template".
type whatsAppMessage struct {
	Product  string            `json:"messaging_product"`
	To       string            `json:"to"`
	Type     string            `json:"type"`
	Text     *whatsAppText     `json:"text,omitempty"`
	Template *whatsAppTemplate `json:"template,omitempty"`
}

type whatsAppText struct {
	Body string `json:"body"`
}

type whatsAppTemplate struct {
	Name       string              `json:"name"`
	Language   whatsAppLanguage    `json:"language"`
	Components []whatsAppComponent `json:"components"`
}

type whatsAppLanguage struct {
	Code string `json:"code"`
}

type whatsAppComponent struct {
	Type       string              `json:"type"`
	Parameters []whatsAppParameter `json:"parameters"`
}

// whatsAppParameter is a template parameter: {"type": "text", "text": ...}.
type whatsAppParameter struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// Notify sends m to m.To.
func (w *WhatsApp) Notify(ctx context.Context, m Message) error {
	to := strings.TrimPrefix(strings.TrimSpace(m.To), "+")
	if to == "" {
		return errors.New("notify: whatsapp: no phone to send to")
	}
	msg := whatsAppMessage{Product: "whatsapp", To: to, Type: "text", Text: &whatsAppText{Body: m.Text}}
	if w.Template != "" {
		msg.Type, msg.Text = "template", nil
		msg.Template = &whatsAppTemplate{Name: w.Template, Language: whatsAppLanguage{Code: cmp.Or(w.Language, "en")},
			Components: []whatsAppComponent{{Type: "body", Parameters: []whatsAppParameter{{Type: "text", Text: m.Text}}}}}
	}
	body, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	url := strings.TrimSuffix(cmp.Or(w.APIURL, DefaultWhatsAppAPI), "/") + "/" + w.PhoneNumberID + "/messages"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+w.Token)
	return post(w.Client, req, "whatsapp")
}
