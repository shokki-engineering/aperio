// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package live

// pricing.go is the pricing repositories, in a file of their own: the repository
// package is named pricing, as the service package a test imports is.

import (
	"github.com/shokki-engineering/aperio/internal/database/repository/aperio/pricing"
	"github.com/shokki-engineering/aperio/internal/database/repository/repox"
	"github.com/shokki-engineering/aperio/internal/service/generated/aperio/pricing/v1/pricingpbv1svc"
)

// PricingRepositories are the PricingService's repositories over conn.
func PricingRepositories(conn repox.Conn) pricingpbv1svc.PricingServiceRepositories {
	r := pricing.New(conn)
	return pricingpbv1svc.PricingServiceRepositories{Seasons: r.Seasons, PriceRules: r.PriceRules}
}
