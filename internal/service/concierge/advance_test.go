// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package concierge

// advance_test.go checks the room tariff's GST slab at its edge.

import (
	"math/big"
	"testing"
)

func TestRoomGST(t *testing.T) {
	for _, c := range []struct {
		perNight *big.Rat
		want     string
	}{
		{big.NewRat(4000, 1), "5"},
		{big.NewRat(7500, 1), "5"},
		{big.NewRat(750001, 100), "18"},
		{big.NewRat(14000, 1), "18"},
	} {
		if got := roomGST(c.perNight); got != c.want {
			t.Errorf("roomGST(%s) = %s; want %s", c.perNight.FloatString(2), got, c.want)
		}
	}
}
