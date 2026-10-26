// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package config loads the aperio service configuration and exposes it via
// Get(). Every configuration file is compiled into the binary, so it runs with
// nothing beside it: aperio.release.toml holds production-safe defaults
// (secrets blank, injected at deploy time), and aperio.dev.toml the
// development profile, chosen with APERIO_PROFILE=dev. See load.go for the
// layers. It follows freebusy's internal/config, trimmed to what aperio uses.
package config

import (
	"embed"
	"time"

	"github.com/the-protobuf-project/runtime-go/grpc/options"
)

//go:embed aperio.release.toml aperio.dev.toml
var configFS embed.FS

// devOverlayPath is the development profile's file on disk, relative to the
// working directory. When present it is read instead of the embedded copy, so
// a developer's edits apply without a rebuild.
const devOverlayPath = "internal/config/aperio.dev.toml"

// ProfileVar picks the profile layered over the release defaults: "dev" for
// the development profile, "release" for none. Unset, the development profile
// applies only when its file is in the working directory -- a checkout.
const ProfileVar = "APERIO_PROFILE"

// AppConfig is the top-level configuration tree.
type AppConfig struct {
	// Meta is the service identity.
	Meta AppMeta `koanf:"app"`
	// Server is the gRPC, HTTP and MCP listeners.
	Server ServerConfig `koanf:"server"`
	// Database is aperio's own Postgres, over GORM or Hasura DDN.
	Database DatabaseConfig `koanf:"database"`
	// Cache is the Redis or Dragonfly behind the generated caches.
	Cache CacheConfig `koanf:"cache"`
	// Freebusy is the availability engine aperio books through.
	Freebusy FreebusyConfig `koanf:"freebusy"`
	// Concierge is the guest-facing find, hold and book.
	Concierge ConciergeConfig `koanf:"concierge"`
}

// AppMeta contains service identity fields.
type AppMeta struct {
	// Name is the service display name.
	Name string `koanf:"name"`
	// Version is the service version (e.g. "v1.0.0").
	Version string `koanf:"version"`
	// Description is the long-form description shown to clients and MCP.
	Description string `koanf:"description"`
}

// ServerConfig holds the hybrid gRPC/HTTP/MCP server settings. GetGRPCOptions
// (options.go) maps these onto the runtime's options.Options.
type ServerConfig struct {
	// Environment is the operating mode: development, debug, staging, or production.
	Environment options.ServerEnvironment `koanf:"environment"`
	// EnableHTTP starts the HTTP/JSON gateway alongside gRPC.
	EnableHTTP bool `koanf:"enable_http"`
	// EnableHealth registers the standard gRPC health-check service.
	EnableHealth bool `koanf:"enable_health"`
	// EnableMCP starts the Model Context Protocol server.
	EnableMCP bool `koanf:"enable_mcp"`
	// ExperimentalHTTP3 enables experimental HTTP/3 on the HTTP port + 1. It
	// requires TLS (see TLS below).
	ExperimentalHTTP3 bool `koanf:"experimental_http3"`
	// TLS holds the certificate/key pair. When both paths are set, TLS is enabled
	// for gRPC and HTTP; it is required for ExperimentalHTTP3.
	TLS TLSConfig `koanf:"tls"`
	// GRPC is the gRPC listener host/port.
	GRPC ListenConfig `koanf:"grpc"`
	// HTTP is the HTTP gateway listener host/port.
	HTTP ListenConfig `koanf:"http"`
	// MCP is the MCP server listener and transport.
	MCP MCPConfig `koanf:"mcp"`
	// CORS lets browsers on other origins call the HTTP port (the REST
	// gateway, CalDAV and /health). Off by default.
	CORS CORSConfig `koanf:"cors"`
	// ShutdownTimeoutSeconds bounds how long a stop waits for in-flight
	// requests -- HTTP gateways first, then gRPC -- before cutting them off.
	// 0 keeps the runtime's default (20 s). Keep it below the orchestrator's
	// grace period minus any preStop delay, leaving room to flush telemetry.
	ShutdownTimeoutSeconds int `koanf:"shutdown_timeout_seconds"`
}

// ShutdownTimeout is ShutdownTimeoutSeconds as a duration; 0 means the
// runtime's default.
func (s ServerConfig) ShutdownTimeout() time.Duration {
	return time.Duration(s.ShutdownTimeoutSeconds) * time.Second
}

// TLSConfig is the TLS certificate/key pair. Leave both paths empty to serve
// plaintext; set both to enable TLS (and, with ExperimentalHTTP3, HTTP/3).
type TLSConfig struct {
	// CertFile is the path to the PEM-encoded certificate.
	CertFile string `koanf:"cert_file"`
	// KeyFile is the path to the PEM-encoded private key.
	KeyFile string `koanf:"key_file"`
}

// Enabled reports whether both a certificate and key are configured.
func (t TLSConfig) Enabled() bool { return t.CertFile != "" && t.KeyFile != "" }

// CORSConfig is the cross-origin policy for the HTTP port. The server maps it
// onto runtime-go's grpc.CORSPolicy; the MCP port is a separate listener and
// is not covered. Lists set from the environment are comma-separated:
// FREEBUSY_SERVER__CORS__ALLOWED_ORIGINS=https://a.example,https://b.example.
type CORSConfig struct {
	// Enabled turns the policy on. Off, no CORS headers are sent and browsers
	// on other origins cannot read responses.
	Enabled bool `koanf:"enabled"`
	// AllowedOrigins are exact origins (scheme://host[:port]) or "*" for any.
	AllowedOrigins []string `koanf:"allowed_origins"`
	// AllowedMethods answer preflights. Empty means GET, POST, PATCH, PUT,
	// DELETE, OPTIONS.
	AllowedMethods []string `koanf:"allowed_methods"`
	// AllowedHeaders answer preflights. Empty means what a GAPIC browser
	// client sends: content-type, authorization, x-goog-api-client,
	// x-goog-request-params, x-goog-user-project, x-api-key.
	AllowedHeaders []string `koanf:"allowed_headers"`
	// ExposedHeaders are response headers a browser script may read.
	ExposedHeaders []string `koanf:"exposed_headers"`
	// AllowCredentials lets browsers send cookies and HTTP authentication.
	// It cannot be combined with the "*" origin.
	AllowCredentials bool `koanf:"allow_credentials"`
	// MaxAgeSeconds is how long a browser may cache a preflight; 0 sends no
	// Access-Control-Max-Age.
	MaxAgeSeconds int `koanf:"max_age_seconds"`
}

// MaxAge is MaxAgeSeconds as a duration.
func (c CORSConfig) MaxAge() time.Duration {
	return time.Duration(c.MaxAgeSeconds) * time.Second
}

// ListenConfig is a network host/port pair.
type ListenConfig struct {
	// Host is the interface to bind (e.g. "0.0.0.0").
	Host string `koanf:"host"`
	// Port is the TCP port to listen on.
	Port int `koanf:"port"`
}

// MCPConfig is the MCP listener plus its transport protocol.
type MCPConfig struct {
	// Host is the interface to bind (ignored for the stdio transport).
	Host string `koanf:"host"`
	// Port is the TCP port for HTTP-based transports (ignored for stdio).
	Port int `koanf:"port"`
	// Transport is the MCP protocol: stdio, streamable-http, or sse.
	Transport options.MCPTransport `koanf:"transport"`
	// Path is where one MCP server carrying every service's tools answers on
	// the MCP port. Empty means "/mcp".
	Path string `koanf:"path"`
	// Stateless serves streamable HTTP without sessions, at Path and at each
	// service's own path: no Mcp-Session-Id is issued, so any replica answers
	// any request. It gives up server-to-client requests (elicitation,
	// sampling, roots), the standalone GET stream and DELETE.
	Stateless bool `koanf:"stateless"`
}
