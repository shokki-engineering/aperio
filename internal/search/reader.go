// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package search is how services read aperio's read models -- the
// projections the generated layer keeps current with every write, such as
// stay.v1.StayNight -- and their optional OpenSearch copy, all of it
// generated (internal/database/repository, internal/database/opensearch), as
// freebusy's search package is for its own: a Reader is a read model's
// table in Postgres, or its copy in OpenSearch with the table behind it
// whenever OpenSearch errs or is slow ([search] provider).
package search

import (
	"context"

	"github.com/shokki-engineering/aperio/internal/database/repository/repox"
	"google.golang.org/protobuf/proto"
)

// Reader reads one read model: a page of rows, how many match, and a facet
// (how many rows hold each value of a field).
type Reader[T proto.Message] interface {
	Search(ctx context.Context, in repox.SearchInput) ([]T, string, error)
	Count(ctx context.Context, in repox.SearchInput) (int64, error)
	Facet(ctx context.Context, in repox.SearchInput, field string, limit int) ([]repox.Bucket, error)
}

// Postgres reads the read model p from its table, through conn
// (database.Connection.SearchConn).
func Postgres[T proto.Message](p *repox.Projection[T], conn repox.Conn) Reader[T] {
	return table[T]{p: p, conn: conn}
}

// table is a read model's table.
type table[T proto.Message] struct {
	p    *repox.Projection[T]
	conn repox.Conn
}

func (t table[T]) Search(ctx context.Context, in repox.SearchInput) ([]T, string, error) {
	return t.p.Search(ctx, t.conn, in)
}

func (t table[T]) Count(ctx context.Context, in repox.SearchInput) (int64, error) {
	return t.p.Count(ctx, t.conn, in)
}

func (t table[T]) Facet(ctx context.Context, in repox.SearchInput, field string, limit int) ([]repox.Bucket, error) {
	return t.p.Facet(ctx, t.conn, in, field, limit)
}

// All reads every page of in from r.
func All[T proto.Message](ctx context.Context, r Reader[T], in repox.SearchInput) ([]T, error) {
	var out []T
	for {
		page, next, err := r.Search(ctx, in)
		if err != nil {
			return nil, err
		}
		out = append(out, page...)
		if next == "" {
			return out, nil
		}
		in.PageToken = next
	}
}
