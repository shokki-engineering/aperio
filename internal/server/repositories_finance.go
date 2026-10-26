// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package server

// repositories_finance.go builds the FinanceService's repositories, in a file of
// its own: the repository package is named finance, as the service package
// assembly.go imports is.

import (
	"github.com/shokki-engineering/aperio/internal/database/repository/aperio/finance"
	"github.com/shokki-engineering/aperio/internal/database/repository/repox"
	"github.com/shokki-engineering/aperio/internal/service/generated/aperio/finance/v1/financepbv1svc"
)

// financeRepositories are the FinanceService's repositories over conn.
func financeRepositories(conn repox.Conn) financepbv1svc.FinanceServiceRepositories {
	r := finance.New(conn)
	return financepbv1svc.FinanceServiceRepositories{Invoices: r.Invoices, Payments: r.Payments}
}
