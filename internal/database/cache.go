// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package database

// cache.go opens the cache backend ([cache], Redis or Dragonfly) and the
// generated list caches on it, and makes them the connection's change
// observer, as freebusy's internal/database/cache.go does.

import (
	"context"
	"fmt"

	"github.com/shokki-engineering/aperio/internal/config"
	"github.com/shokki-engineering/aperio/internal/database/repository/repox"
	"github.com/shokki-engineering/aperio/internal/telemetry"
	"github.com/the-protobuf-project/runtime-go/cache"
	"github.com/the-protobuf-project/runtime-go/cache/redis"
)

// attachCaches opens the list caches when [cache] is enabled and makes them
// the connection's change observer.
//
// A cache that cannot be opened is logged and left off rather than failing
// startup: every read it would have served is still correct from the
// database, only slower. When several instances share a cache, one running
// without it moves no versions, so its writes reach the others' cached pages
// only when those expire -- a minute for every list aperio caches.
func (c *Connection) attachCaches(ctx context.Context) {
	p, err := OpenCacheProvider(ctx)
	if err == nil && p != nil {
		err = c.UseCaches(ctx, p)
	}
	if err != nil {
		telemetry.Warn("List caches are off: the cache could not be opened", "error", err)
	}
}

// UseCaches opens every list cache on p (caches.go) and observes the
// connection's writes with them.
func (c *Connection) UseCaches(ctx context.Context, p cache.Provider) error {
	caches, err := openCaches(ctx, p)
	if err != nil {
		return err
	}
	c.Caches = caches
	c.cacheProvider = p
	c.Changes = caches.observers()
	return nil
}

// measured wraps db's Volatile store, which a list cache reads its pages
// through, in the process's meter: its gets are the hit rate
// (cache_gets_total, strategy volatile).
func measured(db *cache.DB) *cache.DB {
	db.Volatile = cache.WithVolatileTelemetry(db.Volatile, telemetry.Meter())
	return db
}

// reportCacheError logs a cache failure a list cache absorbed. None fails a
// request, but a version that did not move leaves a parent's pages stale for
// their ttl, so each is worth a line.
func reportCacheError(_ context.Context, resource, op string, err error) {
	telemetry.Warn("List cache failure", "resource", resource, "op", op, "error", err)
}

// OpenCacheProvider builds the cache.Provider the generated caches are
// constructed against, or returns (nil, nil) when [cache] enabled is false.
//
// The backend lives here rather than in the protos on purpose: cache.v1 names
// strategies, never a driver, because which server serves them is deployment
// policy. Prefix is config for the same reason, and separates one
// deployment's keys from another's sharing a server. Redis and Dragonfly are
// the same driver behind RESP, so `address` may name either.
//
// A nil Provider is a supported result, not an error: callers treat it as "no
// cache configured" and read the database, which is always correct.
func OpenCacheProvider(ctx context.Context) (cache.Provider, error) {
	cfg := config.Get().Cache
	if !cfg.Enabled {
		return nil, nil
	}
	client, err := redis.NewClient(ctx, redis.Config{
		Address:  cfg.Address,
		Username: cfg.Username,
		Password: cfg.Password,
		Database: cfg.Database,
		PoolSize: cfg.PoolSize,
	})
	if err != nil {
		return nil, fmt.Errorf("open cache redis %q: %w", cfg.Address, err)
	}
	// DefaultTTL is deliberately left zero: every cached resource carries its
	// own ttl in cache.v1, so a default would silently apply to one whose
	// annotation was forgotten. RequireTTL makes that an error instead.
	return redis.New(client, cache.Config{Prefix: cfg.Prefix, RequireTTL: true}), nil
}

// observers is every list cache in c, as one change observer.
func (c Caches) observers() repox.Changes {
	return repox.Changes{c.Venues, c.Spaces, c.Dishes, c.Menus, c.FixedPackages, c.Seasons, c.PriceRules}
}
