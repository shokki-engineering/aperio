// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package server

// runtime.go opens what the services run on, from the configuration: the
// database connection (opened here when the caller hands none, as the e2e
// suites do) and the freebusy client.

import (
	"github.com/shokki-engineering/aperio/internal/config"
	"github.com/shokki-engineering/aperio/internal/database"
	"github.com/shokki-engineering/aperio/internal/engine"
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
	return New(conn, fb), fb, nil
}
