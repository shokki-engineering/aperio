// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package database owns the database connection plumbing: it opens the
// configured backend and holds the live handles (GORM and/or Hasura), and it
// selects the provider once from the loaded config ([database].provider; GORM
// by default, Hasura opt-in). It is domain-agnostic -- each service is built
// from Repox, the handle the generated repositories take, so swapping
// backends is a configuration change. It follows freebusy's
// internal/database.
package database

import (
	"strings"

	"github.com/shokki-engineering/aperio/internal/config"
	"github.com/shokki-engineering/aperio/internal/database/hasura/aperioql"
	"github.com/shokki-engineering/aperio/internal/database/repository/repox"
	"gorm.io/gorm"
)

// Provider identifies a database backend implementation.
type Provider string

const (
	// ProviderGorm is the default backend: GORM over the relational database.
	ProviderGorm Provider = "gorm"
	// ProviderHasura is the opt-in backend: Hasura DDN's GraphQL.
	ProviderHasura Provider = "hasura"
)

// Connection carries the live backend handles plus the provider that
// selected them. Only the selected provider's handle needs to be set
// (PgSQLConn for GORM, Hasura for Hasura).
type Connection struct {
	PgSQLConn *gorm.DB
	Hasura    *aperioql.Service
	Provider  Provider
}

// providerFromConfig resolves the configured provider, defaulting to GORM
// for an empty or unrecognized value.
func providerFromConfig() Provider {
	switch strings.ToLower(strings.TrimSpace(config.Get().Database.Provider)) {
	case string(ProviderHasura):
		return ProviderHasura
	default:
		return ProviderGorm
	}
}

// Repox returns the handle the generated repositories take, carrying only
// the selected provider's connection: repox picks GraphQL whenever that
// handle is set, so passing both would route a GORM deployment to Hasura.
func (c *Connection) Repox() repox.Conn {
	if c.Provider == ProviderHasura {
		return repox.Conn{GraphQL: c.Hasura}
	}
	return repox.Conn{Gorm: c.PgSQLConn}
}

// Close closes the Postgres pool, if there is one.
func (c *Connection) Close() error {
	if c.PgSQLConn == nil {
		return nil
	}
	db, err := c.PgSQLConn.DB()
	if err != nil {
		return err
	}
	return db.Close()
}
