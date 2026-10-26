// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package booking

// reference.go is a booking's short reference: what a guest is given and the
// desk reads back. Nothing is stored for it; it is the last six letters and
// digits of the booking's id, upper-cased, and among one guest's bookings it
// is unique enough.

import (
	"strings"

	"github.com/shokki-engineering/aperio/internal/service/generated/servicex"
)

// Reference is the short reference of the booking name.
func Reference(name string) string {
	id := strings.ReplaceAll(servicex.IDOf(name), "-", "")
	return strings.ToUpper(id[max(0, len(id)-6):])
}
