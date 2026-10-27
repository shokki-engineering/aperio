// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package server

// repositories_pricing.go builds the PricingService's repositories, in a file of
// its own: the repository package is named pricing, as the service package
// assembly.go imports is.

import (
	"github.com/shokki-engineering/aperio/internal/database"
	"github.com/shokki-engineering/aperio/internal/database/repository/aperio/pricing"
	"github.com/shokki-engineering/aperio/internal/database/repository/repox"
	"github.com/shokki-engineering/aperio/internal/service/generated/aperio/pricing/v1/pricingpbv1svc"
)

// pricingRepositories are the PricingService's repositories over conn, listing
// through caches when they are on.
func pricingRepositories(conn repox.Conn, caches database.Caches) pricingpbv1svc.PricingServiceRepositories {
	r := caches.Pricing(pricing.New(conn))
	return pricingpbv1svc.PricingServiceRepositories{Seasons: r.Seasons, PriceRules: r.PriceRules}
}
