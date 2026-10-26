// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package server

// runtime.go opens what the services run on, from the configuration: the
// database connection (opened here when the caller hands none, as the e2e
// suites do) and the freebusy client.

import (
	"context"
	"crypto/rand"

	"github.com/shokki-engineering/aperio/internal/config"
	"github.com/shokki-engineering/aperio/internal/database"
	"github.com/shokki-engineering/aperio/internal/engine"
	"github.com/shokki-engineering/aperio/internal/telemetry"
)

// newServices assembles the services on conn, or on a connection opened
// from config when conn is nil, with the freebusy client the config names.
// It returns the client too, which the server closes on Stop.
func newServices(conn *database.Connection) (*Services, *engine.Client, error) {
	if conn == nil {
		opened, err := database.Open()
		if err != nil {
			return nil, nil, err
		}
		conn = opened
	}
	fb, err := engine.DialConfig(config.Get().Freebusy)
	if err != nil {
		return nil, nil, err
	}
	secret, err := holdSecret(config.Get().Concierge.HoldSecret)
	if err != nil {
		return nil, nil, err
	}
	services := New(conn, fb, secret)
	if err := services.useOpenSearch(context.Background(), config.Get().Search); err != nil {
		return nil, nil, err
	}
	return services, fb, nil
}

// holdSecret is the concierge's configured hold secret, or one made for this
// process when none is: its hold tokens then outlive neither a restart nor a
// move to another replica, which is said.
func holdSecret(configured string) ([]byte, error) {
	if configured != "" {
		return []byte(configured), nil
	}
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return nil, err
	}
	telemetry.Warn("concierge.hold_secret is not set: hold tokens are this process's own; set APERIO_CONCIERGE__HOLD_SECRET")
	return secret, nil
}
