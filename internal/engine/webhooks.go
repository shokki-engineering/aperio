// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package engine

// webhooks.go keeps aperio's subscription to freebusy's WebhookService: the
// endpoint freebusy POSTs reservation and hold changes to, signed with a
// secret freebusy mints -- returned by CreateWebhook and RotateWebhookSecret
// alone -- so aperio can name the subscription but not choose its secret.

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/shokki-engineering/freebusy-go"
	"github.com/shokki-engineering/freebusy-go/pb/webhookpb"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
)

// Subscription is aperio's webhook in freebusy.
type Subscription struct {
	// Name is webhooks/{id}.
	Name string
	// Secret is the signing secret, when this call minted one (a create or
	// a rotation); empty otherwise.
	Secret string
	// Paused is a webhook an operator paused: nothing is delivered.
	Paused bool
}

// Subscribe makes sure webhooks/{id} sends uri the events of types: it
// creates it, or brings its endpoint and types up to date. Run it at every
// start; it changes nothing when nothing changed.
func (c *Client) Subscribe(ctx context.Context, id, uri string, types []string) (Subscription, error) {
	name := "webhooks/" + id
	w, err := c.fb.Webhooks.Get(ctx, &webhookpb.GetWebhookRequest{Name: name})
	if errors.Is(err, freebusy.ErrNotFound) {
		w, err = c.fb.Webhooks.New(ctx, &webhookpb.CreateWebhookRequest{WebhookId: id, Webhook: &webhookpb.Webhook{
			DisplayName: "aperio", Uri: uri, EventTypes: types,
		}})
		if err != nil {
			return Subscription{}, fmt.Errorf("engine: create webhook: %w", err)
		}
		return Subscription{Name: w.GetName(), Secret: w.GetSecret()}, nil
	}
	if err != nil {
		return Subscription{}, fmt.Errorf("engine: get webhook: %w", err)
	}
	if w.GetUri() != uri || !sameTypes(w.GetEventTypes(), types) {
		w, err = c.fb.Webhooks.Update(ctx, &webhookpb.UpdateWebhookRequest{
			Webhook:    &webhookpb.Webhook{Name: name, Uri: uri, EventTypes: types},
			UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"uri", "event_types"}},
		})
		if err != nil {
			return Subscription{}, fmt.Errorf("engine: update webhook: %w", err)
		}
	}
	return Subscription{Name: name, Paused: w.GetState() == webhookpb.WebhookState_WEBHOOK_STATE_PAUSED}, nil
}

// RotateSecret mints the webhook a new signing secret and returns it; for a
// day freebusy signs with the old one too.
func (c *Client) RotateSecret(ctx context.Context, webhook string) (string, error) {
	w, err := c.fb.Webhooks.RotateSecret(ctx, &webhookpb.RotateWebhookSecretRequest{Name: webhook})
	if err != nil {
		return "", fmt.Errorf("engine: rotate %s: %w", webhook, err)
	}
	return w.GetSecret(), nil
}

// sameTypes reports whether a and b name the same event types.
func sameTypes(a, b []string) bool {
	a, b = slices.Clone(a), slices.Clone(b)
	slices.Sort(a)
	slices.Sort(b)
	return slices.Equal(a, b)
}
