// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package live

// sales.go is the sales repositories, in a file of their own: the repository
// package is named sales, as the service package a test imports is.

import (
	"github.com/shokki-engineering/aperio/internal/database/repository/aperio/sales"
	"github.com/shokki-engineering/aperio/internal/database/repository/repox"
	"github.com/shokki-engineering/aperio/internal/service/generated/aperio/sales/v1/salespbv1svc"
)

// SalesRepositories are the SalesService's repositories over conn.
func SalesRepositories(conn repox.Conn) salespbv1svc.SalesServiceRepositories {
	r := sales.New(conn)
	return salespbv1svc.SalesServiceRepositories{Leads: r.Leads, Activities: r.Activities, Quotes: r.Quotes}
}
