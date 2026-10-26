// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package live

// finance.go is the finance repositories, in a file of their own: the repository
// package is named finance, as the service package a test imports is.

import (
	"github.com/shokki-engineering/aperio/internal/database/repository/aperio/finance"
	"github.com/shokki-engineering/aperio/internal/database/repository/repox"
	"github.com/shokki-engineering/aperio/internal/service/generated/aperio/finance/v1/financepbv1svc"
)

// FinanceRepositories are the FinanceService's repositories over conn.
func FinanceRepositories(conn repox.Conn) financepbv1svc.FinanceServiceRepositories {
	r := finance.New(conn)
	return financepbv1svc.FinanceServiceRepositories{Invoices: r.Invoices, Payments: r.Payments}
}
