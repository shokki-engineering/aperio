// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package database

// caches.go is the list caches protoc-gen-cache generates for the resources
// aperio's protos annotate ((cache.v1.cache).list): venues and spaces, the
// catalog's dishes, menus and packages, pricing's seasons and rules -- the
// reads every quote and price makes, and that change rarely.

import (
	"context"

	"github.com/shokki-engineering/aperio/internal/database/cache/aperio/catalogcache"
	"github.com/shokki-engineering/aperio/internal/database/cache/aperio/pricingcache"
	"github.com/shokki-engineering/aperio/internal/database/cache/aperio/venuecache"
	"github.com/the-protobuf-project/runtime-go/cache"
)

// Caches are the generated list caches a Connection's repositories are
// served through (Venue, Catalog and Pricing, caches_wrap.go). A nil field is
// a resource read straight from the database; all of them are nil when the
// cache is off.
type Caches struct {
	Venues        *venuecache.VenueListCache
	Spaces        *venuecache.SpaceListCache
	Dishes        *catalogcache.DishListCache
	Menus         *catalogcache.MenuListCache
	FixedPackages *catalogcache.FixedPackageListCache
	Seasons       *pricingcache.SeasonListCache
	PriceRules    *pricingcache.PriceRuleListCache
}

// opener opens one cache database after another on p, measured, keeping the
// first failure: once one fails, the rest are not opened.
type opener struct {
	ctx context.Context
	p   cache.Provider
	err error
}

// db opens the database open selects, or returns nil after a failure.
func (o *opener) db(open func(context.Context, cache.Provider) (*cache.DB, error)) *cache.DB {
	if o.err != nil {
		return nil
	}
	db, err := open(o.ctx, o.p)
	if err != nil {
		o.err = err
		return nil
	}
	return measured(db)
}

// keep records err, if it is the first failure.
func (o *opener) keep(err error) {
	if o.err == nil {
		o.err = err
	}
}

// openCaches opens every list cache on p, each reporting the failures it
// absorbs to reportCacheError. Any one failing fails them all, so a
// connection has every cache or none.
func openCaches(ctx context.Context, p cache.Provider) (Caches, error) {
	o := &opener{ctx: ctx, p: p}
	var (
		c   Caches
		err error
	)
	c.Venues, err = venuecache.NewVenueListCache(o.db(venuecache.OpenVenueListCache), venuecache.WithListErrors(reportCacheError))
	o.keep(err)
	c.Spaces, err = venuecache.NewSpaceListCache(o.db(venuecache.OpenSpaceListCache), venuecache.WithListErrors(reportCacheError))
	o.keep(err)
	c.Dishes, err = catalogcache.NewDishListCache(o.db(catalogcache.OpenDishListCache), catalogcache.WithListErrors(reportCacheError))
	o.keep(err)
	c.Menus, err = catalogcache.NewMenuListCache(o.db(catalogcache.OpenMenuListCache), catalogcache.WithListErrors(reportCacheError))
	o.keep(err)
	c.FixedPackages, err = catalogcache.NewFixedPackageListCache(o.db(catalogcache.OpenFixedPackageListCache), catalogcache.WithListErrors(reportCacheError))
	o.keep(err)
	c.Seasons, err = pricingcache.NewSeasonListCache(o.db(pricingcache.OpenSeasonListCache), pricingcache.WithListErrors(reportCacheError))
	o.keep(err)
	c.PriceRules, err = pricingcache.NewPriceRuleListCache(o.db(pricingcache.OpenPriceRuleListCache), pricingcache.WithListErrors(reportCacheError))
	o.keep(err)
	if o.err != nil {
		return Caches{}, o.err
	}
	return c, nil
}
