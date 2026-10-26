// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package config

// search.go configures the read models (stay.v1.StayNight and those to come)
// and their optional OpenSearch copy (internal/search), as freebusy's
// [search] does for its own.

import (
	"cmp"
	"time"
)

// SearchConfig is where the read models are searched, and when they are
// rebuilt.
type SearchConfig struct {
	// Provider is where the read models are searched: "postgres" (the
	// default) or "opensearch", which falls back to Postgres on an error or
	// a timeout. APERIO_SEARCH__PROVIDER.
	Provider string `koanf:"provider"`
	// RebuildOnStart is when the server recomputes a read model as it
	// starts, in the background while it serves: "empty" (the default) when
	// its table has no rows -- a fresh deployment, or an upgrade from a
	// version without it; "always" on every start -- the repair after writes
	// that bypassed the API; "never". APERIO_SEARCH__REBUILD_ON_START.
	RebuildOnStart string `koanf:"rebuild_on_start"`
	// OpenSearch is the cluster provider "opensearch" indexes into.
	OpenSearch OpenSearchConfig `koanf:"opensearch"`
}

// When the server rebuilds a read model, or reindexes its OpenSearch copy, at
// startup ([search] rebuild_on_start, [search.opensearch] reindex_on_start).
const (
	OnStartEmpty  = "empty"
	OnStartAlways = "always"
	OnStartNever  = "never"
)

// OnStartModes are the values rebuild_on_start and reindex_on_start take.
var OnStartModes = []string{OnStartEmpty, OnStartAlways, OnStartNever}

// RebuildMode is RebuildOnStart, "empty" when unset.
func (s SearchConfig) RebuildMode() string { return cmp.Or(s.RebuildOnStart, OnStartEmpty) }

// UsesOpenSearch reports whether the read models are searched on OpenSearch.
func (s SearchConfig) UsesOpenSearch() bool { return s.Provider == "opensearch" }

// OpenSearchConfig is the OpenSearch cluster the read models are copied to.
type OpenSearchConfig struct {
	// URL is the cluster endpoint, e.g. "http://localhost:9200".
	// APERIO_SEARCH__OPENSEARCH__URL.
	URL string `koanf:"url"`
	// Username and Password are HTTP basic credentials; empty sends none.
	// Secrets: APERIO_SEARCH__OPENSEARCH__USERNAME / __PASSWORD.
	Username string `koanf:"username"`
	Password string `koanf:"password"`
	// IndexQueue is how many committed changes an indexer buffers before a
	// write waits for it; 0 is 1024. APERIO_SEARCH__OPENSEARCH__INDEX_QUEUE.
	IndexQueue int `koanf:"index_queue"`
	// TimeoutMS bounds one search before it falls back to Postgres; 0 is
	// 500. APERIO_SEARCH__OPENSEARCH__TIMEOUT_MS.
	TimeoutMS int `koanf:"timeout_ms"`
	// ReindexOnStart is when the server reloads an index from its read
	// model's table as it starts, in the background: "empty" (the default)
	// when the index has no documents but the table has rows; "always" on
	// every start -- the repair after an indexer missed changes; "never".
	// APERIO_SEARCH__OPENSEARCH__REINDEX_ON_START.
	ReindexOnStart string `koanf:"reindex_on_start"`
}

// ReindexMode is ReindexOnStart, "empty" when unset.
func (o OpenSearchConfig) ReindexMode() string { return cmp.Or(o.ReindexOnStart, OnStartEmpty) }

// Timeout is TimeoutMS as a duration: 500ms when unset.
func (o OpenSearchConfig) Timeout() time.Duration {
	return time.Duration(cmp.Or(o.TimeoutMS, 500)) * time.Millisecond
}
