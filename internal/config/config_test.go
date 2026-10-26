// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package config

// The layers load as load.go says: the release defaults, then (none, under
// APERIO_PROFILE=release) the profile, then APERIO_ variables, which win.

import "testing"

func TestLoadReleaseWithOverrides(t *testing.T) {
	t.Setenv(ProfileVar, "release")
	t.Setenv("APERIO_DATABASE__POSTGRES__HOST", "db.internal")
	t.Setenv("APERIO_SERVER__CORS__ALLOWED_ORIGINS", "https://a.example, https://b.example")
	t.Setenv("APERIO_SERVER__CORS__ENABLED", "true")

	cfg, err := load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Meta.Name != "aperio" || cfg.Server.GRPC.Port != 50061 || cfg.Server.HTTP.Port != 8090 {
		t.Errorf("identity and ports: %q %d %d; want aperio 50061 8090", cfg.Meta.Name, cfg.Server.GRPC.Port, cfg.Server.HTTP.Port)
	}
	if cfg.Database.Postgres.Host != "db.internal" || cfg.Database.Postgres.SSLMode != "require" {
		t.Errorf("postgres: %q, sslmode %q; want the override, and the release sslmode", cfg.Database.Postgres.Host, cfg.Database.Postgres.SSLMode)
	}
	if got := cfg.Server.CORS.AllowedOrigins; len(got) != 2 || got[1] != "https://b.example" {
		t.Errorf("cors origins = %q; want the two from the variable", got)
	}
	if cfg.Freebusy.TimeoutSeconds != 10 {
		t.Errorf("freebusy timeout = %d; want the release default, 10", cfg.Freebusy.TimeoutSeconds)
	}
}
