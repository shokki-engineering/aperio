// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package config

// concierge.go configures the concierge (internal/service/concierge): the
// guest-facing find, hold and book of the website and of voice agents.

// ConciergeConfig is the concierge's settings.
type ConciergeConfig struct {
	// HoldSecret seals the hold tokens HoldStay gives and ConfirmStay takes,
	// so a hold's price and guest cannot change between the two: 32 or more
	// random bytes (openssl rand -hex 32), the same on every replica. Empty,
	// each process makes its own, and a token outlives neither a restart nor
	// a move to another replica. A secret: set it through
	// APERIO_CONCIERGE__HOLD_SECRET, not a file.
	HoldSecret string `koanf:"hold_secret"`
}
