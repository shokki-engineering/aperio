// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package config

// concierge.go configures the concierge (internal/service/concierge): the
// guest-facing find, hold and book of the website and of voice agents, and
// the advance a booked stay asks for.

// ConciergeConfig is the concierge's settings.
type ConciergeConfig struct {
	// HoldSecret seals the hold tokens HoldStay gives and ConfirmStay takes,
	// so a hold's price and guest cannot change between the two: 32 or more
	// random bytes (openssl rand -hex 32), the same on every replica. Empty,
	// each process makes its own, and a token outlives neither a restart nor
	// a move to another replica. A secret: set it through
	// APERIO_CONCIERGE__HOLD_SECRET, not a file.
	HoldSecret string `koanf:"hold_secret"`
	// AdvancePercent is the share of a stay's proforma, taxes included,
	// that ConfirmStay asks the guest to pay as the advance that confirms
	// it: 1 to 100. 0 (unset) means 100, the whole stay up front.
	// APERIO_CONCIERGE__ADVANCE_PERCENT.
	AdvancePercent int `koanf:"advance_percent"`
}

// Advance is AdvancePercent within 1 to 100: 100 when unset or out of
// range.
func (c ConciergeConfig) Advance() int {
	if c.AdvancePercent <= 0 || c.AdvancePercent > 100 {
		return 100
	}
	return c.AdvancePercent
}
