// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package notify_test

// Tests of the notifiers against httptest servers: the webhook's body and
// signature as a receiver checks them, the WhatsApp Cloud API request for
// free text and for a template, a failing delivery logged and counted
// without failing the caller, and the provider the configuration picks.

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/shokki-engineering/aperio/internal/config"
	"github.com/shokki-engineering/aperio/internal/notify"
)

var msg = notify.Message{To: "+919800000000", Event: "aperio.operations.service_request.created",
	Subject: "venues/v/serviceRequests/r", Text: "2 × black coffee to 204"}

func TestWebhookSignsTheBody(t *testing.T) {
	secret := []byte("0123456789abcdef0123456789abcdef")
	var body []byte
	var sig, event string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ = io.ReadAll(r.Body)
		sig, event = r.Header.Get(notify.SignatureHeader), r.Header.Get(notify.EventHeader)
	}))
	defer srv.Close()
	at := time.Date(2026, 10, 27, 9, 30, 0, 0, time.UTC)
	wh := &notify.Webhook{URL: srv.URL, Secret: secret, Now: func() time.Time { return at }}
	if err := wh.Notify(context.Background(), msg); err != nil {
		t.Fatal(err)
	}
	if !notify.Verify(secret, body, sig) || event != msg.Event {
		t.Fatalf("signature %q over %s does not verify (event %q)", sig, body, event)
	}
	if notify.Verify([]byte("another secret, of 32 bytes or so"), body, sig) || notify.Verify(secret, append(body, ' '), sig) {
		t.Fatal("signature verifies under another secret or another body")
	}
	var got map[string]string
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}
	if got["to"] != msg.To || got["text"] != msg.Text || got["subject"] != msg.Subject || got["send_time"] != "2026-10-27T09:30:00Z" {
		t.Errorf("body = %s", body)
	}
}

func TestWebhookFailsOnAnErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusBadGateway) }))
	defer srv.Close()
	if err := (&notify.Webhook{URL: srv.URL, Secret: []byte("s")}).Notify(context.Background(), msg); err == nil {
		t.Fatal("a 502 delivered")
	}
}

func TestWhatsAppRequest(t *testing.T) {
	var path, auth string
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path, auth = r.URL.Path, r.Header.Get("Authorization")
		got = nil
		_ = json.NewDecoder(r.Body).Decode(&got)
		_, _ = w.Write([]byte(`{"messages":[{"id":"wamid.x"}]}`))
	}))
	defer srv.Close()
	wa := &notify.WhatsApp{APIURL: srv.URL + "/v21.0", PhoneNumberID: "1234567890", Token: "tok"}

	if err := wa.Notify(context.Background(), msg); err != nil {
		t.Fatal(err)
	}
	if path != "/v21.0/1234567890/messages" || auth != "Bearer tok" {
		t.Errorf("POST %s with %q", path, auth)
	}
	text, _ := got["text"].(map[string]any)
	if got["messaging_product"] != "whatsapp" || got["to"] != "919800000000" || got["type"] != "text" || text["body"] != msg.Text {
		t.Errorf("text message = %v", got)
	}

	wa.Template, wa.Language = "service_request", "en_GB"
	if err := wa.Notify(context.Background(), msg); err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(got["template"])
	want := `{"components":[{"parameters":[{"text":"2 × black coffee to 204","type":"text"}],"type":"body"}],` +
		`"language":{"code":"en_GB"},"name":"service_request"}`
	if got["type"] != "template" || got["text"] != nil || string(b) != want {
		t.Errorf("template message = %v\n template %s\n want     %s", got, b, want)
	}

	if err := wa.Notify(context.Background(), notify.Message{Text: "x"}); err == nil {
		t.Error("sent to no phone")
	}
}

// failing is a Notifier that never delivers.
type failing struct{}

func (failing) Notify(context.Context, notify.Message) error { return errors.New("down") }

func TestSenderLogsAndCountsFailures(t *testing.T) {
	var warned int
	s := notify.NewSender(failing{}, time.Second, nil, func(string, ...any) { warned++ })
	s.Send(context.Background(), msg)
	s.Send(context.Background(), notify.Message{Text: "no one to tell"})
	s.Wait()
	if s.Failed() != 1 || warned != 1 {
		t.Errorf("failed = %d, warned = %d; want 1, 1", s.Failed(), warned)
	}
	var none *notify.Sender
	none.Send(context.Background(), msg)
	none.Wait()
}

func TestNewPicksTheProvider(t *testing.T) {
	secret := "0123456789abcdef0123456789abcdef"
	for _, c := range []struct {
		cfg  config.NotifyConfig
		want string
	}{
		{config.NotifyConfig{}, "notify.Log"},
		{config.NotifyConfig{Provider: "webhook", Webhook: config.WebhookConfig{URL: "https://example.com/hook", Secret: secret}}, "*notify.Webhook"},
		{config.NotifyConfig{Provider: "whatsapp", WhatsApp: config.WhatsAppConfig{PhoneNumberID: "1", Token: "t"}}, "*notify.WhatsApp"},
		{config.NotifyConfig{Provider: "webhook", Webhook: config.WebhookConfig{URL: "https://example.com/hook", Secret: "short"}}, ""},
		{config.NotifyConfig{Provider: "whatsapp"}, ""},
		{config.NotifyConfig{Provider: "pigeon"}, ""},
	} {
		n, err := notify.New(c.cfg, nil)
		got := ""
		if err == nil {
			got = typeName(n)
		}
		if got != c.want {
			t.Errorf("New(%q) = %s, %v; want %s", c.cfg.Provider, got, err, c.want)
		}
	}
}

func typeName(n notify.Notifier) string {
	switch n.(type) {
	case notify.Log:
		return "notify.Log"
	case *notify.Webhook:
		return "*notify.Webhook"
	case *notify.WhatsApp:
		return "*notify.WhatsApp"
	}
	return "?"
}
