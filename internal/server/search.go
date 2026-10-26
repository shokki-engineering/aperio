// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package server

// search.go reads the read models from their OpenSearch copies when [search]
// provider = "opensearch", as freebusy's assembly does: each copy's index is
// ensured, its indexer follows the commits to its read model, and the
// service reading it reads the copy, falling back to the table.

import (
	"context"
	"fmt"

	"github.com/shokki-engineering/aperio/internal/config"
	"github.com/shokki-engineering/aperio/internal/database/repository/repox"
	"github.com/shokki-engineering/aperio/internal/search"
	"github.com/shokki-engineering/aperio/internal/service/stay"
	"github.com/shokki-engineering/aperio/internal/telemetry"
)

// useOpenSearch opens the copies cfg asks for; none when it names Postgres.
func (s *Services) useOpenSearch(ctx context.Context, cfg config.SearchConfig) error {
	if !cfg.UsesOpenSearch() {
		return nil
	}
	cluster, err := search.Connect(cfg.OpenSearch, telemetry.Meter(), telemetry.Warn)
	if err != nil {
		return err
	}
	nights, err := stay.OpenNights(ctx, cluster, stay.PostgresNights(s.Conn.SearchConn()))
	if err != nil {
		return fmt.Errorf("open search: %w", err)
	}
	s.Conn.Projections.Feed = repox.Feeds(nights.Feed(func(ctx context.Context, err error) {
		// The write landed; the copy missed it until the next write to
		// those rows, or a reindex ([search.opensearch] reindex_on_start).
		telemetry.Warn("OpenSearch did not take a read-model change", "read_model", "stay nights", "error", err)
	}))
	s.Stays.ReadNightsFrom(nights)
	s.copies = map[string]searchIndex{"stay nights": nights}
	return nil
}
