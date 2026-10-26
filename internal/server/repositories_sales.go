// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package server

// repositories_sales.go builds the SalesService's repositories, in a file of
// its own: the repository package is named sales, as the service package
// assembly.go imports is.

import (
	"github.com/shokki-engineering/aperio/internal/database/repository/aperio/sales"
	"github.com/shokki-engineering/aperio/internal/database/repository/repox"
	"github.com/shokki-engineering/aperio/internal/service/generated/aperio/sales/v1/salespbv1svc"
)

// salesRepositories are the SalesService's repositories over conn.
func salesRepositories(conn repox.Conn) salespbv1svc.SalesServiceRepositories {
	r := sales.New(conn)
	return salespbv1svc.SalesServiceRepositories{Dishes: r.Dishes, Menus: r.Menus, FixedPackages: r.FixedPackages, Seasons: r.Seasons, PriceRules: r.PriceRules, Quotes: r.Quotes}
}
