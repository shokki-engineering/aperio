// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package config

// channels_test.go checks Booking.com's hotels_json: parsed into Hotels, and
// refused when malformed or naming no venue or space.

import (
	"testing"
	"time"
)

// TestBookingComHotelsJSON parses a good mapping and refuses bad ones.
func TestBookingComHotelsJSON(t *testing.T) {
	good := BookingComConfig{HotelsJSON: `[{"hotel_code": "7777001", "venue": "venues/sea-breeze",
		"rooms": [{"code": "SV-DLX", "space": "venues/sea-breeze/spaces/deluxe", "rate_plan": "BAR"}]}]`}
	if err := good.resolve(); err != nil {
		t.Fatal(err)
	}
	if len(good.Hotels) != 1 || good.Hotels[0].Rooms[0].RatePlan != "BAR" {
		t.Errorf("hotels = %+v; want the one from JSON", good.Hotels)
	}
	for _, bad := range []string{
		`{"hotel_code": "1"}`,
		`[{"hotel_code": "1", "venue": "sea-breeze"}]`,
		`[{"hotel_code": "1", "venue": "venues/a", "rooms": [{"code": "X", "space": "venues/b/spaces/c"}]}]`,
	} {
		c := BookingComConfig{HotelsJSON: bad}
		if err := c.resolve(); err == nil {
			t.Errorf("hotels_json %s was accepted", bad)
		}
	}
}

// TestHasuraConnectTimeout resolves the default and the single attempt.
func TestHasuraConnectTimeout(t *testing.T) {
	for in, want := range map[int]time.Duration{0: DefaultHasuraConnectTimeout, -1: 0, 5: 5 * time.Second} {
		if got := (HasuraConfig{ConnectTimeoutSeconds: in}).ConnectTimeout(); got != want {
			t.Errorf("ConnectTimeout(%d) = %v, want %v", in, got, want)
		}
	}
}
