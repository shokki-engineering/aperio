// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package config

// freebusy.go is how aperio reaches freebusy, the availability engine every
// stay and session is booked in: its gRPC endpoint for the SDK, and its
// public address and feeds secret for the calendar links aperio hands out.

// FreebusyConfig is the freebusy server aperio books through.
type FreebusyConfig struct {
	// Endpoint is freebusy's gRPC address, host:port. Empty leaves the SDK
	// to its FREEBUSY_ENDPOINT variable.
	Endpoint string `koanf:"endpoint"`
	// Insecure dials without TLS: a local freebusy.
	Insecure bool `koanf:"insecure"`
	// CAFile is a PEM file of the CA that signed freebusy's certificate, for
	// one the system does not trust: mkcert's, in development.
	CAFile string `koanf:"ca_file"`
	// TimeoutSeconds bounds each call (0: the SDK's default).
	TimeoutSeconds int `koanf:"timeout_seconds"`
	// PublicURL is freebusy's public HTTPS address, where calendar apps
	// fetch the .ics feeds of the rooms' and halls' calendars.
	PublicURL string `koanf:"public_url"`
	// FeedsSecret is freebusy's [feeds] secret, which signs those links. A
	// secret: set it through APERIO_FREEBUSY__FEEDS_SECRET, not a file.
	FeedsSecret string `koanf:"feeds_secret"`
}
