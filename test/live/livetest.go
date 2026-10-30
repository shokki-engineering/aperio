// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package live gates the live suites -- the tests that need a running
// Postgres or Hasura DDN engine -- on the environment variables naming them,
// and hands them what they share: a connection to the database, and each
// service's repositories over it (one file per domain, as each repository
// package shares its name with the service package a test imports).
//
// Locally an unset variable skips what needs it, so `go test ./...` passes
// with neither running. In CI, APERIO_TEST_REQUIRE_LIVE=1 makes an unset
// variable a failure instead: a job whose database never came up must not
// pass by testing nothing.
//
//	APERIO_TEST_DATABASE_URL="host=127.0.0.1 port=5433 user=postgres password=postgrespassword dbname=aperio sslmode=disable" \
//	APERIO_TEST_GRAPHQL_URL=http://localhost:3280/graphql \
//	  go test ./...
package live

import (
	"os"
	"testing"

	"github.com/shokki-engineering/aperio/internal/database/repository/repox"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// The variables the live suites read.
const (
	// DatabaseURL is the aperio Postgres, as a libpq DSN or a URL.
	DatabaseURL = "APERIO_TEST_DATABASE_URL"
	// GraphQLURL is the Hasura DDN engine's GraphQL endpoint.
	GraphQLURL = "APERIO_TEST_GRAPHQL_URL"
	// RedisAddr is the cache suites' Redis (or Dragonfly), as host:port.
	RedisAddr = "APERIO_TEST_REDIS_ADDR"
)

// Required reports whether live suites must run rather than skip.
func Required() bool { return os.Getenv("APERIO_TEST_REQUIRE_LIVE") == "1" }

// Env returns the value of key. Unset, it skips t -- or fails it when live
// suites are required.
func Env(t testing.TB, key string) string {
	t.Helper()
	v := os.Getenv(key)
	if v == "" {
		if Required() {
			t.Fatalf("%s is not set, and APERIO_TEST_REQUIRE_LIVE=1 requires the live suites", key)
		}
		t.Skipf("%s not set — live suite skipped", key)
	}
	return v
}

// Optional returns the value of key, for a backend a test covers only when it
// is there. Unset, it is "" -- or t fails when live suites are required.
func Optional(t testing.TB, key string) string {
	t.Helper()
	v := os.Getenv(key)
	if v == "" && Required() {
		t.Fatalf("%s is not set, and APERIO_TEST_REQUIRE_LIVE=1 requires every backend", key)
	}
	return v
}

// Conn opens the live database over GORM, or skips t.
func Conn(t testing.TB) repox.Conn {
	t.Helper()
	db, err := gorm.Open(postgres.Open(Env(t, DatabaseURL)), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatalf("open %s: %v", DatabaseURL, err)
	}
	return repox.Conn{Gorm: db}
}
