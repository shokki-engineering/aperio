// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package config

// database.go is aperio's own database -- Postgres, read and written over
// GORM or through Hasura DDN -- and the cache in front of it.

import (
	"cmp"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"time"
)

// DatabaseConfig selects the persistence backend and carries the connection
// settings for each supported provider. Only the selected provider's block
// needs to be populated.
type DatabaseConfig struct {
	// Provider selects the backend: "gorm" (Postgres) or "hasura".
	Provider string `koanf:"provider"`
	// Postgres holds the GORM/Postgres connection settings.
	Postgres PostgresConfig `koanf:"postgres"`
	// Hasura holds the Hasura GraphQL endpoint and admin authentication.
	Hasura HasuraConfig `koanf:"hasura"`
}

// PostgresConfig is the GORM/Postgres backend connection.
type PostgresConfig struct {
	// Host is the database server hostname or IP (e.g. "127.0.0.1").
	Host string `koanf:"host"`
	// Port is the database server port (Postgres default is 5432).
	Port int `koanf:"port"`
	// User is the role to connect as.
	User string `koanf:"user"`
	// Password is the role's password (blank in release; injected at deploy).
	Password string `koanf:"password"`
	// DBName is the target database name.
	DBName string `koanf:"dbname"`
	// SSLMode is the libpq sslmode, e.g. "disable" (local) or "require" (prod).
	SSLMode string `koanf:"sslmode"`
	// TimeZone is the session time zone passed to the driver (e.g. "UTC").
	TimeZone string `koanf:"timezone"`
	// MaxOpenConns caps the pool's open connections; excess requests queue on
	// the pool, making it the process's backpressure point (0 or negative = 25).
	MaxOpenConns int `koanf:"max_open_conns"`
	// MaxIdleConns is how many idle connections the pool retains (0 or negative
	// = MaxOpenConns, so steady traffic reuses instead of churning connections).
	MaxIdleConns int `koanf:"max_idle_conns"`
	// ConnMaxLifetimeMinutes recycles connections after this long, so load
	// re-spreads after database failovers (0 or negative = 30).
	ConnMaxLifetimeMinutes int `koanf:"conn_max_lifetime_minutes"`
	// ConnMaxIdleMinutes closes connections idle this long (0 or negative = 5).
	ConnMaxIdleMinutes int `koanf:"conn_max_idle_minutes"`
	// PlanCacheMode is the plan_cache_mode every connection the server opens
	// sets at startup: "force_custom_plan" (the default when empty), "auto"
	// (Postgres's own default) or "force_generic_plan"; "database" sends
	// nothing and leaves it to ALTER DATABASE/ROLE, for a pooler that refuses
	// startup parameters. See PlanCacheModes.
	PlanCacheMode string `koanf:"plan_cache_mode"`
}

// PlanCacheModeDatabase is the PlanCacheMode that leaves plan_cache_mode to
// the database or role settings instead of setting it per connection.
const PlanCacheModeDatabase = "database"

// PlanCacheModes are the accepted PlanCacheMode values ("" means the first).
var PlanCacheModes = []string{"force_custom_plan", "auto", "force_generic_plan", PlanCacheModeDatabase}

// PlanCache is the plan_cache_mode the connections set, "" for none.
//
// The default forces a custom plan, as freebusy's does: aperio's statements
// are short and filter on JSONB documents and dates, whose best plan depends
// on the values bound, and with Postgres's default a pooled connection
// switches a prepared statement to a generic plan after five runs (freebusy
// measured 150-420 ms against 7-25 ms). Planning each run costs well under a
// millisecond.
func (p PostgresConfig) PlanCache() string {
	switch p.PlanCacheMode {
	case "":
		return PlanCacheModes[0]
	case PlanCacheModeDatabase:
		return ""
	}
	return p.PlanCacheMode
}

// PoolSettings are the resolved connection-pool bounds: PostgresConfig's pool
// fields with the defaults applied for unset values.
type PoolSettings struct {
	MaxOpen     int
	MaxIdle     int
	MaxLifetime time.Duration
	MaxIdleTime time.Duration
}

// Pool resolves the connection-pool settings, applying defaults for unset fields.
func (p PostgresConfig) Pool() PoolSettings {
	s := PoolSettings{
		MaxOpen:     p.MaxOpenConns,
		MaxIdle:     p.MaxIdleConns,
		MaxLifetime: time.Duration(p.ConnMaxLifetimeMinutes) * time.Minute,
		MaxIdleTime: time.Duration(p.ConnMaxIdleMinutes) * time.Minute,
	}
	if s.MaxOpen <= 0 {
		s.MaxOpen = 25
	}
	if s.MaxIdle <= 0 {
		s.MaxIdle = s.MaxOpen
	}
	if s.MaxLifetime <= 0 {
		s.MaxLifetime = 30 * time.Minute
	}
	if s.MaxIdleTime <= 0 {
		s.MaxIdleTime = 5 * time.Minute
	}
	return s
}

// DSN renders the libpq keyword/value connection string that
// gorm.Open(postgres.Open(...)) expects.
//
// Keys pgx does not know itself, like plan_cache_mode, are sent as runtime
// parameters in the startup message, so every pooled connection has them.
func (p PostgresConfig) DSN() string {
	dsn := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s TimeZone=%s",
		p.Host, p.Port, p.User, p.Password, p.DBName, p.SSLMode, p.TimeZone,
	)
	if mode := p.PlanCache(); mode != "" {
		dsn += " plan_cache_mode=" + mode
	}
	return dsn
}

// URL is the connection as a postgres:// URL, for tools that take one (atlas),
// optionally naming another database on the same server.
func (p PostgresConfig) URL(dbname string) string {
	if dbname == "" {
		dbname = p.DBName
	}
	u := url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(p.User, p.Password),
		Host:     net.JoinHostPort(p.Host, strconv.Itoa(p.Port)),
		Path:     "/" + dbname,
		RawQuery: url.Values{"sslmode": {cmp.Or(p.SSLMode, "disable")}}.Encode(),
	}
	return u.String()
}

// HasuraConfig is the Hasura GraphQL backend connection and admin auth.
type HasuraConfig struct {
	// URL is the GraphQL endpoint, e.g. "http://localhost:8080/v1/graphql".
	URL string `koanf:"url"`
	// AdminSecret is sent as the x-hasura-admin-secret header (empty to omit).
	AdminSecret string `koanf:"admin_secret"`
}

// CacheConfig is the backend behind protoc-gen-cache's generated decorators.
//
// The backend is deliberately absent from the protos: cache.v1 names strategies,
// never drivers, so which server serves them is deployment policy and lives here.
// It matters which one, though — resources annotated STRATEGY_INDEXED need
// server-side sets, so Redis or Dragonfly. memcached has no sets and the
// generated constructor refuses at startup rather than failing every lookup
// later.
type CacheConfig struct {
	// Enabled turns the cache on. Off means repositories read straight through
	// to the database, which is always a valid configuration.
	Enabled bool `koanf:"enabled"`
	// Address is the Redis/Dragonfly server, host:port.
	Address string `koanf:"address"`
	// Username and Password authenticate, when the server asks.
	Username string `koanf:"username"`
	Password string `koanf:"password"`
	// Database is the Redis database index this service binds to.
	Database int `koanf:"database"`
	// PoolSize caps concurrent connections. Zero takes the driver's default.
	PoolSize int `koanf:"pool_size"`
	// Prefix namespaces every key, separating this deployment's entries from
	// another's sharing one server. It is deliberately not a proto annotation:
	// a prefix separates deployments, and an annotation travels with the schema.
	Prefix string `koanf:"prefix"`
}
