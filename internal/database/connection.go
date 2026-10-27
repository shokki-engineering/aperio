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
	"context"
	"strings"
	"sync"

	"github.com/shokki-engineering/aperio/internal/config"
	"github.com/shokki-engineering/aperio/internal/database/hasura/aperioql"
	"github.com/shokki-engineering/aperio/internal/database/repository/repox"
	"github.com/the-protobuf-project/runtime-go/cache"
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
// (PgSQLConn for GORM, Hasura for Hasura). Build it with Open, or as a
// literal and then UseCaches; it is not copied once in use.
type Connection struct {
	PgSQLConn *gorm.DB
	Hasura    *aperioql.Service
	Provider  Provider

	// Caches are the list caches over this connection's repositories; their
	// fields are nil when the cache is off. Changes reports every write a
	// repository built from Repox commits to them, which is what keeps a
	// cached page from outliving a write.
	Caches  Caches
	Changes repox.ChangeObserver
	// Projections keeps the projection tables (the read models, such as
	// stay.v1.StayNight) current with every write a repository or Batch
	// built from Repox makes: inside the write's transaction on GORM, right
	// after it on Hasura.
	Projections *repox.Projections

	// observers hear every write as Changes does, added through Observe.
	observers observerList
	// cacheProvider is the cache UseCaches opened, nil when it is off.
	cacheProvider cache.Provider
}

// observerList is the observers Observe added, safe to read while one is
// being added.
type observerList struct {
	mu   sync.RWMutex
	list []repox.ChangeObserver
}

// Observe makes o hear every write a repository or Batch built from this
// connection's Repox commits -- built before this call or after -- alongside
// Changes. For a service that keeps something derived from records and must
// drop it when one is written.
func (c *Connection) Observe(o repox.ChangeObserver) {
	c.observers.mu.Lock()
	defer c.observers.mu.Unlock()
	c.observers.list = append(c.observers.list, o)
}

// CacheProvider is the cache backend UseCaches opened, or nil when the cache
// is off.
func (c *Connection) CacheProvider() cache.Provider { return c.cacheProvider }

// changes is the observer a Repox handle carries: Changes, then every
// observer Observe added, read at the time of each write.
type changes struct{ c *Connection }

func (ch changes) Changed(ctx context.Context, name string) {
	if ch.c.Changes != nil {
		ch.c.Changes.Changed(ctx, name)
	}
	ch.c.observers.mu.RLock()
	list := ch.c.observers.list
	ch.c.observers.mu.RUnlock()
	for _, o := range list {
		o.Changed(ctx, name)
	}
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
// Its Changes is the connection's, so the list caches hear every write.
func (c *Connection) Repox() repox.Conn {
	if c.Provider == ProviderHasura {
		return repox.Conn{GraphQL: c.Hasura, Changes: changes{c}, Projections: c.Projections}
	}
	return repox.Conn{Gorm: c.PgSQLConn, Changes: changes{c}, Projections: c.Projections}
}

// SearchConn is the handle projection searches read through: GORM whenever
// the connection has a Postgres handle, Hasura deployments included -- the
// projection tables live in the same Postgres, and DDN has none of the
// upserts or GROUP BY a search wants, so over GraphQL a search scans at most
// repox.GraphQLScanLimit rows. Writes keep going through Repox. Projections
// rides along so a projection's Apply and Rebuild publish to its feed.
func (c *Connection) SearchConn() repox.Conn {
	if c.PgSQLConn != nil {
		return repox.Conn{Gorm: c.PgSQLConn, Projections: c.Projections}
	}
	return repox.Conn{GraphQL: c.Hasura, Projections: c.Projections}
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
