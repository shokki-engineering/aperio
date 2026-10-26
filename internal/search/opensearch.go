// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package search

// opensearch.go is a read model's copy in OpenSearch, as freebusy keeps its
// availability read model's: the index is ensured at startup, an indexer
// follows every committed write to the read model, a reindex reloads it from
// Postgres behind an alias, and reads fall back to the table whenever
// OpenSearch errs or takes longer than [search.opensearch] timeout_ms.

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/oh-tarnished/opensearch-go-sdk/opensearch"
	"github.com/shokki-engineering/aperio/internal/config"
	"github.com/shokki-engineering/aperio/internal/database/opensearch/osx"
	"github.com/shokki-engineering/aperio/internal/database/repository/repox"
	"github.com/the-protobuf-project/runtime-go/telemetry"
	"google.golang.org/protobuf/proto"
)

// Cluster is the OpenSearch cluster the read models are copied to.
type Cluster struct {
	client    *opensearch.Client
	timeout   time.Duration
	queue     int
	fallbacks telemetry.Counter
	warn      func(msg string, kv ...any)
}

// Connect builds the client for the cluster cfg names without touching it;
// warn logs a fallback, m counts them (nil counts nothing).
func Connect(cfg config.OpenSearchConfig, m telemetry.Meter, warn func(msg string, kv ...any)) (*Cluster, error) {
	if cfg.URL == "" {
		return nil, errors.New("search: [search.opensearch] url is required with provider = \"opensearch\"")
	}
	c, err := opensearch.New(opensearch.Config{URL: cfg.URL, Username: cfg.Username, Password: cfg.Password, Timeout: 30 * time.Second, MaxRetries: -1})
	if err != nil {
		return nil, fmt.Errorf("search: opensearch client: %w", err)
	}
	if m == nil {
		m = telemetry.NoopMeter
	}
	return &Cluster{client: c, timeout: cfg.Timeout(), queue: cmp.Or(cfg.IndexQueue, 1024), warn: warn,
		fallbacks: m.Counter("aperio.search.fallbacks", telemetry.WithUnit("1"),
			telemetry.WithDescription("Read-model reads served from Postgres because OpenSearch failed, by read model and reason (error, timeout)."))}, nil
}

// Copy is one read model's copy in the cluster, read with the read model's
// table (pg) behind it.
type Copy[T proto.Message] struct {
	cluster *Cluster
	index   *osx.Index[T]
	pg      Reader[T]
	indexer *osx.Indexer[T]

	mu         sync.Mutex
	nextLog    time.Time // when the next fallback may be logged
	suppressed int       // fallbacks since the last line that were not logged
}

// Open ensures index in the cluster -- its template, a first versioned index
// and the alias, when they are missing -- and returns the copy, read with pg
// behind it.
func Open[T proto.Message](ctx context.Context, c *Cluster, index *osx.Index[T], pg Reader[T]) (*Copy[T], error) {
	if _, err := index.Ensure(ctx, c.client); err != nil {
		return nil, fmt.Errorf("search: ensure index %s: %w", index.Spec.Alias, err)
	}
	return &Copy[T]{cluster: c, index: index, pg: pg}, nil
}

// Feed is the indexer following every committed write to the read model, for
// repox.Projections.Feed. A change it fails to index goes to onError; Reindex
// repairs what it missed.
func (o *Copy[T]) Feed(onError func(ctx context.Context, err error)) repox.ProjectionFeed {
	return o.FeedWith(osx.IndexerOptions{Queue: o.cluster.queue, OnError: onError})
}

// FeedWith is Feed with the indexer's options given: Queue 0 and Refresh
// "wait_for" index each commit before the write returns, for a test.
func (o *Copy[T]) FeedWith(opts osx.IndexerOptions) repox.ProjectionFeed {
	if o.indexer == nil {
		o.indexer = o.index.Indexer(o.cluster.client, opts)
	}
	return o.indexer
}

// Close flushes the indexer.
func (o *Copy[T]) Close(ctx context.Context) error {
	if o.indexer == nil {
		return nil
	}
	return o.indexer.Close(ctx)
}

// Reindex loads a new index from the read model's table read through pg,
// swaps the alias to it, re-applies rows changed during the load, and drops
// the old index.
func (o *Copy[T]) Reindex(ctx context.Context, pg repox.Conn) (osx.ReindexResult, error) {
	return o.index.Reindex(ctx, o.cluster.client, o.index.Source(ctx, pg), osx.ReindexOptions{})
}

// Empty reports whether the index holds no documents.
func (o *Copy[T]) Empty(ctx context.Context) (bool, error) {
	n, err := o.index.Count(ctx, o.cluster.client, repox.SearchInput{})
	return n == 0, err
}

// Search reads a page from OpenSearch, or from Postgres when that fails. A
// page token is the backend's own: a token OpenSearch refuses (one Postgres
// issued after an earlier fallback) is retried on Postgres.
func (o *Copy[T]) Search(ctx context.Context, in repox.SearchInput) ([]T, string, error) {
	tctx, cancel := context.WithTimeout(ctx, o.cluster.timeout)
	rows, next, err := o.index.Search(tctx, o.cluster.client, in)
	cancel()
	if err == nil {
		return rows, next, nil
	}
	if errors.Is(err, repox.ErrInvalidArgument) && in.PageToken == "" {
		return nil, "", err // the filter itself is wrong: Postgres would say the same
	}
	o.record(ctx, tctx, "search", err)
	return o.pg.Search(ctx, in)
}

// Count counts on OpenSearch, or on Postgres when that fails.
func (o *Copy[T]) Count(ctx context.Context, in repox.SearchInput) (int64, error) {
	tctx, cancel := context.WithTimeout(ctx, o.cluster.timeout)
	n, err := o.index.Count(tctx, o.cluster.client, in)
	cancel()
	if err == nil || errors.Is(err, repox.ErrInvalidArgument) {
		return n, err
	}
	o.record(ctx, tctx, "count", err)
	return o.pg.Count(ctx, in)
}

// Facet counts a field's values on OpenSearch, or on Postgres when that
// fails.
func (o *Copy[T]) Facet(ctx context.Context, in repox.SearchInput, field string, limit int) ([]repox.Bucket, error) {
	tctx, cancel := context.WithTimeout(ctx, o.cluster.timeout)
	out, err := o.index.Facet(tctx, o.cluster.client, in, field, limit)
	cancel()
	if err == nil || errors.Is(err, repox.ErrInvalidArgument) {
		return out, err
	}
	o.record(ctx, tctx, "facet", err)
	return o.pg.Facet(ctx, in, field, limit)
}

// fallbackLogEvery is how often a fallback is logged at most. The others are
// only counted, and the next line says how many there were, so an outage
// under load is one warning every few seconds rather than one per read.
const fallbackLogEvery = 10 * time.Second

// record counts a fallback from op and logs it, at most once every
// fallbackLogEvery.
func (o *Copy[T]) record(ctx, tctx context.Context, op string, err error) {
	reason := "error"
	if errors.Is(tctx.Err(), context.DeadlineExceeded) || errors.Is(err, context.DeadlineExceeded) {
		reason = "timeout"
	}
	o.cluster.fallbacks.Add(ctx, 1, telemetry.Labels{"read_model": o.index.Spec.Alias, "reason": reason})
	o.mu.Lock()
	now := time.Now()
	if now.Before(o.nextLog) {
		o.suppressed++
		o.mu.Unlock()
		return
	}
	suppressed := o.suppressed
	o.nextLog, o.suppressed = now.Add(fallbackLogEvery), 0
	o.mu.Unlock()
	if o.cluster.warn != nil {
		o.cluster.warn("read model fell back to Postgres: OpenSearch failed",
			"read_model", o.index.Spec.Alias, "op", op, "reason", reason, "error", err, "fallbacks_not_logged", suppressed)
	}
}
