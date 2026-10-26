// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package stay

// nights_search.go is the stay read model's copy in OpenSearch, in a file of
// its own: it imports the generated OpenSearch stay package, which shares
// this package's name (and the repository package's, which nights.go
// imports).

import (
	"context"

	"github.com/shokki-engineering/aperio/internal/database/opensearch/aperio/stay"
	"github.com/shokki-engineering/aperio/internal/search"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/stay/v1/staypbv1"
)

// OpenNights ensures the read model's index in c and returns its copy, read
// with pg (PostgresNights) behind it.
func OpenNights(ctx context.Context, c *search.Cluster, pg search.Reader[*staypbv1.StayNight]) (*search.Copy[*staypbv1.StayNight], error) {
	return search.Open(ctx, c, stay.StayNightIndex, pg)
}
