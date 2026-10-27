// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package database_test

// Query tracing: with the process's pipeline set, Instrument makes every
// query a repository runs a span, a child of the caller's -- the request's,
// in the server.
//
//	APERIO_TEST_DATABASE_URL=... go test ./internal/database/ -run LiveInstrument -v

import (
	"context"
	"strings"
	"testing"

	"github.com/shokki-engineering/aperio/internal/database"
	"github.com/shokki-engineering/aperio/internal/database/repository/aperio/venue"
	"github.com/shokki-engineering/aperio/internal/telemetry"
	"github.com/shokki-engineering/aperio/test/live"
	"go.opentelemetry.io/otel"
)

func TestLiveInstrument(t *testing.T) {
	db := live.Conn(t).Gorm
	h, exporter := inMemoryTelemetry(t)
	telemetry.Use(h)
	t.Cleanup(func() { telemetry.Use(nil) })
	if err := database.Instrument(db); err != nil {
		t.Fatal(err)
	}
	conn := &database.Connection{PgSQLConn: db, Provider: database.ProviderGorm}

	ctx, parent := otel.Tracer("aperio-test").Start(context.Background(), "request")
	_, err := venue.New(conn.Repox()).Venues.Get(ctx, "venues/instrument-test-missing")
	parent.End()
	if err == nil {
		t.Fatal("Get found a venue that does not exist")
	}

	var queries int
	for _, s := range exporter.GetSpans() {
		if !strings.HasPrefix(s.Name, "gorm.") {
			continue
		}
		queries++
		if s.Parent.TraceID() != parent.SpanContext().TraceID() {
			t.Errorf("span %s is in trace %s, want the request's %s", s.Name, s.Parent.TraceID(), parent.SpanContext().TraceID())
		}
	}
	if queries == 0 {
		var names []string
		for _, s := range exporter.GetSpans() {
			names = append(names, s.Name)
		}
		t.Fatalf("no query span; spans: %v", names)
	}
	t.Logf("%d query span(s) under the request's trace", queries)
}
