// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package stay_test

// The front desk's days read from the stay read model's OpenSearch copy: the
// indexer following each write, and every read served by OpenSearch -- the
// Postgres reader behind the copy fails the test if it is ever asked.
//
//	APERIO_TEST_OPENSEARCH_URL=http://localhost:9200 go test ./internal/service/stay/ -run OpenSearch

import (
	"context"
	"os"
	"testing"

	"github.com/shokki-engineering/aperio/internal/config"
	"github.com/shokki-engineering/aperio/internal/database/opensearch/osx"
	"github.com/shokki-engineering/aperio/internal/database/repository/repox"
	"github.com/shokki-engineering/aperio/internal/search"
	"github.com/shokki-engineering/aperio/internal/service/stay"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/stay/v1/staypbv1"
)

func TestLiveSummarizeDayOpenSearch(t *testing.T) {
	url := os.Getenv("APERIO_TEST_OPENSEARCH_URL")
	if url == "" {
		t.Skip("APERIO_TEST_OPENSEARCH_URL not set: the OpenSearch copy is optional")
	}
	f, ctx := setup(t), context.Background()
	cluster, err := search.Connect(config.OpenSearchConfig{URL: url}, nil, func(msg string, kv ...any) { t.Errorf("%s %v", msg, kv) })
	if err != nil {
		t.Fatal(err)
	}
	nights, err := stay.OpenNights(ctx, cluster, unasked{t})
	if err != nil {
		t.Fatalf("open the copy: %v", err)
	}
	f.conn.Projections.Feed = nights.FeedWith(osx.IndexerOptions{Refresh: "wait_for", OnError: func(_ context.Context, err error) { t.Errorf("index: %v", err) }})
	f.svc.ReadNightsFrom(nights)
	checkDays(t, f)
}

// unasked is a Postgres reader that fails t when read: the copy must answer.
type unasked struct{ t *testing.T }

func (u unasked) Search(context.Context, repox.SearchInput) ([]*staypbv1.StayNight, string, error) {
	u.t.Error("the copy fell back to Postgres")
	return nil, "", nil
}

func (u unasked) Count(context.Context, repox.SearchInput) (int64, error) {
	u.t.Error("the copy fell back to Postgres")
	return 0, nil
}

func (u unasked) Facet(context.Context, repox.SearchInput, string, int) ([]repox.Bucket, error) {
	u.t.Error("the copy fell back to Postgres")
	return nil, nil
}
