// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package database

// caches_wrap.go puts the list caches in front of the repositories they
// cache, one schema's set at a time, as the server assembles them:
//
//	venues := conn.Caches.Venue(venue.New(conn.Repox()))
//
// Only List is cached; Get, writes and everything else are the repository's.
// A set wrapped while the cache is off is returned as it was.

import (
	"github.com/shokki-engineering/aperio/internal/database/repository/aperio/catalog"
	"github.com/shokki-engineering/aperio/internal/database/repository/aperio/pricing"
	"github.com/shokki-engineering/aperio/internal/database/repository/aperio/venue"
)

// Venue returns r with its venues' and spaces' List served from the cache.
func (c Caches) Venue(r venue.Repositories) venue.Repositories {
	if c.Venues != nil {
		r.Venues = c.Venues.Wrap(r.Venues)
	}
	if c.Spaces != nil {
		r.Spaces = c.Spaces.Wrap(r.Spaces)
	}
	return r
}

// Catalog returns r with its dishes', menus' and packages' List served from
// the cache.
func (c Caches) Catalog(r catalog.Repositories) catalog.Repositories {
	if c.Dishes != nil {
		r.Dishes = c.Dishes.Wrap(r.Dishes)
	}
	if c.Menus != nil {
		r.Menus = c.Menus.Wrap(r.Menus)
	}
	if c.FixedPackages != nil {
		r.FixedPackages = c.FixedPackages.Wrap(r.FixedPackages)
	}
	return r
}

// Pricing returns r with its seasons' and price rules' List served from the
// cache.
func (c Caches) Pricing(r pricing.Repositories) pricing.Repositories {
	if c.Seasons != nil {
		r.Seasons = c.Seasons.Wrap(r.Seasons)
	}
	if c.PriceRules != nil {
		r.PriceRules = c.PriceRules.Wrap(r.PriceRules)
	}
	return r
}
