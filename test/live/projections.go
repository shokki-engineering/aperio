// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package live

// projections.go puts aperio's read models on a test's connection, as the
// server's assembly does, so the repositories built over it keep them
// current.

import (
	"github.com/shokki-engineering/aperio/internal/database/repository/repox"
	"github.com/shokki-engineering/aperio/internal/service/stay"
)

// Projected is conn keeping the read models current with its writes.
func Projected(conn repox.Conn) repox.Conn {
	conn.Projections = repox.NewProjections(stay.NightsProjector())
	return conn
}
