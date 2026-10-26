// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package booking

// errors.go is the errors and keys the booking services share.

import (
	"fmt"
	"slices"

	"github.com/shokki-engineering/aperio/internal/service/generated/servicex"
)

// RoomsKey is the freebusy booking key of room line i of reservation: what a
// hold made before the reservation exists (the concierge's) must be keyed by
// for the reservation to take its rooms from that hold.
func RoomsKey(reservation string, i int) string {
	return fmt.Sprintf("%s#rooms/%d", reservation, i)
}

// Unavailable is the error for n of space that are not free.
func Unavailable(space string, n int32) error {
	return servicex.Blocked("NOT_AVAILABLE", SpaceType, space,
		fmt.Sprintf("%d of the space are not free for the dates", n),
		"Choose other dates or another space, or book fewer")
}

// WrongState is the error for a state method called in a state it does not
// move from.
func WrongState(resourceType, name, method, state string) error {
	return servicex.Blocked("INVALID_STATE", resourceType, name,
		method+" cannot be called on one that is "+state, "Read it again and call the method its state allows")
}

// InState reports whether s is one of states.
func InState[S comparable](s S, states []S) bool { return slices.Contains(states, s) }
