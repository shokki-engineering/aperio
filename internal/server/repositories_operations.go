// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package server

// repositories_operations.go builds the OperationsService's repositories, in a
// file of its own: the repository package is named operations, as the service
// package assembly.go imports is.

import (
	"github.com/shokki-engineering/aperio/internal/database/repository/aperio/operations"
	"github.com/shokki-engineering/aperio/internal/database/repository/repox"
	"github.com/shokki-engineering/aperio/internal/service/generated/aperio/operations/v1/operationspbv1svc"
)

// operationsRepositories are the OperationsService's repositories over conn.
func operationsRepositories(conn repox.Conn) operationspbv1svc.OperationsServiceRepositories {
	return operationspbv1svc.OperationsServiceRepositories{ServiceRequests: operations.New(conn).ServiceRequests}
}
