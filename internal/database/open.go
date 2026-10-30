// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package database

// open.go opens the configured backend: Postgres over GORM (OpenGorm, with
// constraint errors wrapped and the expected ones kept out of the log),
// traced and with a bounded pool, or the Hasura DDN engine with the caller's
// trace propagated, waited for while it starts (connect.go); and attaches the
// list caches when [cache] is enabled.

import (
	"context"
	"fmt"
	"net/url"

	"github.com/shokki-engineering/aperio/internal/config"
	"github.com/shokki-engineering/aperio/internal/database/gorm/aperio"
	"github.com/shokki-engineering/aperio/internal/database/hasura/aperioql"
	"github.com/shokki-engineering/aperio/internal/telemetry"
	"github.com/the-protobuf-project/runtime-go/network/runtime"
	"go.opentelemetry.io/otel/propagation"
	"gorm.io/gorm"
)

// Open connects to the backend the configuration selects, with the list
// caches attached when [cache] is enabled.
func Open() (*Connection, error) {
	var (
		conn *Connection
		err  error
	)
	if providerFromConfig() == ProviderHasura {
		conn, err = openHasura()
	} else {
		conn, err = openGorm()
	}
	if err != nil {
		return nil, err
	}
	conn.attachCaches(context.Background())
	return conn, nil
}

// openHasura connects to the configured Hasura endpoint (see OpenHasura). The
// read models are searched over Postgres when it is configured, as
// freebusy's are (see SearchConn); writes stay on Hasura. Without it they
// are searched over GraphQL.
func openHasura() (*Connection, error) {
	conn, err := connectHasura(context.Background(), config.Get().Database.Hasura)
	if err != nil {
		return nil, err
	}
	if pg := config.Get().Database.Postgres; pg.Host != "" {
		db, err := OpenGorm(pg.DSN(), &gorm.Config{})
		if err == nil {
			err = Instrument(db)
		}
		if err == nil {
			err = BoundPool(db, pg.Pool())
		}
		if err != nil {
			telemetry.Warn("read models are searched over Hasura: Postgres could not be opened", "err", err)
			return conn, nil
		}
		conn.PgSQLConn = db
	}
	return conn, nil
}

// openGorm dials Postgres with the libpq DSN rendered from config, instruments
// it, and bounds the connection pool. The pool cap is the process's backpressure point:
// excess concurrent queries queue on the pool instead of piling onto
// Postgres.
func openGorm() (*Connection, error) {
	pg := config.Get().Database.Postgres
	db, err := OpenGorm(pg.DSN(), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}
	if err := Instrument(db); err != nil {
		return nil, err
	}
	if err := BoundPool(db, pg.Pool()); err != nil {
		return nil, err
	}
	return &Connection{PgSQLConn: db, Provider: ProviderGorm}, nil
}

// Instrument traces and measures every query on db through the process's
// telemetry pipeline: a span per statement, nested under the calling RPC's,
// and the ORM's metrics (the generated gorm plugin, aperio.Default's
// Instrument). Without a pipeline it does nothing, so call it after
// telemetry.Init.
func Instrument(db *gorm.DB) error {
	if telemetry.Telemetry == nil {
		return nil
	}
	if err := aperio.Default.Instrument(db, telemetry.Telemetry); err != nil {
		return fmt.Errorf("instrument postgres: %w", err)
	}
	return nil
}

// BoundPool applies pool to db's connection pool.
func BoundPool(db *gorm.DB, pool config.PoolSettings) error {
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("postgres pool handle: %w", err)
	}
	sqlDB.SetMaxOpenConns(pool.MaxOpen)
	sqlDB.SetMaxIdleConns(pool.MaxIdle)
	sqlDB.SetConnMaxLifetime(pool.MaxLifetime)
	sqlDB.SetConnMaxIdleTime(pool.MaxIdleTime)
	return nil
}

// OpenHasura connects the typed GraphQL client to the endpoint at rawURL,
// sending adminSecret as the x-hasura-admin-secret header when set. The W3C
// traceparent propagator injects the calling context's span into every
// request, so the DDN engine's spans nest under aperio's in one trace.
func OpenHasura(rawURL, adminSecret string) (*Connection, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("parse hasura url %q: %w", rawURL, err)
	}
	var headers map[string]string
	if adminSecret != "" {
		headers = map[string]string{"x-hasura-admin-secret": adminSecret}
	}
	svc, err := aperioql.New(runtime.ConnectionOptions{
		URL:             runtime.URLFromStd(u),
		Headers:         headers,
		TracePropagator: propagation.TraceContext{},
	})
	if err != nil {
		return nil, fmt.Errorf("connect hasura: %w", err)
	}
	return &Connection{Hasura: svc, Provider: ProviderHasura}, nil
}
