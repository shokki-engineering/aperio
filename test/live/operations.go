// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package live

// operations.go is the operations repositories, in a file of their own: the
// repository package is named operations, as a service package would be.

import (
	"github.com/shokki-engineering/aperio/internal/database/repository/aperio/operations"
	"github.com/shokki-engineering/aperio/internal/database/repository/repox"
	"github.com/shokki-engineering/aperio/internal/service/generated/aperio/operations/v1/operationspbv1svc"
)

// OperationsRepositories are the OperationsService's repositories over conn.
func OperationsRepositories(conn repox.Conn) operationspbv1svc.OperationsServiceRepositories {
	return operationspbv1svc.OperationsServiceRepositories{ServiceRequests: operations.New(conn).ServiceRequests}
}
