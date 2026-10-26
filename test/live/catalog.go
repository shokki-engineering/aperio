// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package live

// catalog.go is the catalog repositories, in a file of their own: the repository
// package is named catalog, as the service package a test imports is.

import (
	"github.com/shokki-engineering/aperio/internal/database/repository/aperio/catalog"
	"github.com/shokki-engineering/aperio/internal/database/repository/repox"
	"github.com/shokki-engineering/aperio/internal/service/generated/aperio/catalog/v1/catalogpbv1svc"
)

// CatalogRepositories are the CatalogService's repositories over conn.
func CatalogRepositories(conn repox.Conn) catalogpbv1svc.CatalogServiceRepositories {
	r := catalog.New(conn)
	return catalogpbv1svc.CatalogServiceRepositories{Dishes: r.Dishes, Menus: r.Menus, FixedPackages: r.FixedPackages}
}
