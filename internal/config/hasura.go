// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package config

// hasura.go is [database.hasura]: the Hasura DDN engine aperio reads and
// writes through when database.provider is "hasura", and how long startup
// waits for it.

import "time"

// DefaultHasuraConnectTimeout is how long startup waits for the engine when
// connect_timeout_seconds is 0.
const DefaultHasuraConnectTimeout = 60 * time.Second

// HasuraConfig is the Hasura GraphQL backend connection and admin auth.
type HasuraConfig struct {
	// URL is the GraphQL endpoint, e.g. "http://localhost:8080/v1/graphql".
	URL string `koanf:"url"`
	// AdminSecret is sent as the x-hasura-admin-secret header (empty to omit).
	AdminSecret string `koanf:"admin_secret"`
	// ConnectTimeoutSeconds is how long startup keeps retrying an engine
	// that does not answer yet -- one still starting beside aperio in
	// Compose or a pod -- before it gives up: 0 is a minute, below 0 tries
	// once.
	ConnectTimeoutSeconds int `koanf:"connect_timeout_seconds"`
}

// ConnectTimeout is ConnectTimeoutSeconds resolved: the default for 0, and 0
// (a single attempt) below it.
func (h HasuraConfig) ConnectTimeout() time.Duration {
	switch {
	case h.ConnectTimeoutSeconds == 0:
		return DefaultHasuraConnectTimeout
	case h.ConnectTimeoutSeconds < 0:
		return 0
	}
	return time.Duration(h.ConnectTimeoutSeconds) * time.Second
}
