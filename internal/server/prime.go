// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package server

// prime.go brings the read models, then their OpenSearch copies, up to date
// as the process starts, as freebusy's does for its own -- in the
// background, while the server serves -- and flushes the copies' indexers as
// it stops. Several replicas starting together may each do it: a rebuild only
// replaces rows, and the last reindex's alias swap wins.

import (
	"context"
	"errors"

	"github.com/shokki-engineering/aperio/internal/config"
	"github.com/shokki-engineering/aperio/internal/database/opensearch/osx"
	"github.com/shokki-engineering/aperio/internal/database/repository/repox"
	"github.com/shokki-engineering/aperio/internal/service/stay"
	"github.com/shokki-engineering/aperio/internal/telemetry"
)

// readModel is one read model as priming sees it.
type readModel struct {
	name    string
	empty   func(ctx context.Context, pg repox.Conn) (bool, error)
	rebuild func(ctx context.Context, conn, pg repox.Conn) error
	// index is its OpenSearch copy; nil without one.
	index searchIndex
}

// searchIndex is what priming and shutdown need of an OpenSearch copy
// (search.Copy).
type searchIndex interface {
	Empty(ctx context.Context) (bool, error)
	Reindex(ctx context.Context, pg repox.Conn) (osx.ReindexResult, error)
	Close(ctx context.Context) error
}

// readModels are aperio's read models, with the copies useOpenSearch opened.
func (s *Services) readModels() []readModel {
	return []readModel{{name: "stay nights", empty: stay.NightsEmpty, rebuild: stay.RebuildNights, index: s.copies["stay nights"]}}
}

// Prime rebuilds each read model as [search] rebuild_on_start says, then
// reindexes its copy as [search.opensearch] reindex_on_start says: "empty"
// reindexes a copy with no documents, unless its read model was just rebuilt
// (the indexer follows that).
func (s *Services) Prime(ctx context.Context) {
	cfg := config.Get().Search
	conn, pg := s.Conn.Repox(), s.Conn.SearchConn()
	for _, m := range s.readModels() {
		rebuilt, err := prime(ctx, cfg.RebuildMode(), func() (bool, error) { return m.empty(ctx, pg) },
			func() error { return m.rebuild(ctx, conn, pg) })
		if err != nil {
			telemetry.Warn("read model startup: not rebuilt", "read_model", m.name, "error", err)
		} else if rebuilt {
			telemetry.Info("read model startup: rebuilt", "read_model", m.name)
		}
		if m.index == nil || (rebuilt && cfg.OpenSearch.ReindexMode() == config.OnStartEmpty) {
			continue
		}
		reindexed, err := prime(ctx, cfg.OpenSearch.ReindexMode(), func() (bool, error) { return m.index.Empty(ctx) },
			func() error { _, err := m.index.Reindex(ctx, pg); return err })
		if err != nil {
			telemetry.Warn("read model startup: copy not reindexed", "read_model", m.name, "error", err)
		} else if reindexed {
			telemetry.Info("read model startup: copy reindexed", "read_model", m.name)
		}
	}
}

// prime runs do as mode says: always, never, or when empty reports so.
func prime(ctx context.Context, mode string, empty func() (bool, error), do func() error) (bool, error) {
	switch mode {
	case config.OnStartNever:
		return false, nil
	case config.OnStartEmpty:
		if ok, err := empty(); err != nil || !ok {
			return false, err
		}
	}
	return true, do()
}

// CloseSearch flushes the OpenSearch copies' indexers.
func (s *Services) CloseSearch(ctx context.Context) error {
	var errs []error
	for _, c := range s.copies {
		errs = append(errs, c.Close(ctx))
	}
	return errors.Join(errs...)
}
