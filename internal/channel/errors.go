// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package channel

// errors.go is why a notification was not applied, in terms an adapter can
// answer the channel in: each channel has its own codes for an unknown
// hotel, an unknown room type or a full date.

import (
	"errors"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// The errors Sync wraps, which an adapter maps onto its channel's codes.
var (
	// ErrUnknownHotel is a hotel code no venue is mapped to.
	ErrUnknownHotel = errors.New("channel: unknown hotel")
	// ErrUnknownRoomType is a room type code no space is mapped to.
	ErrUnknownRoomType = errors.New("channel: unknown room type")
	// ErrMissing is a notification without something it needs: a
	// reservation id, dates, rooms.
	ErrMissing = errors.New("channel: required field missing")
	// ErrUnsupported is a reservation aperio cannot keep as one stay: rooms
	// for different dates.
	ErrUnsupported = errors.New("channel: unsupported reservation")
	// ErrUnknownReservation is a modification or cancellation of a
	// reservation aperio never had, at no hotel it can tell.
	ErrUnknownReservation = errors.New("channel: unknown reservation")
)

// Unavailable reports whether err is the stay service refusing rooms that are
// not free (its NOT_AVAILABLE).
func Unavailable(err error) bool {
	return status.Code(err) == codes.FailedPrecondition && Reason(err) == "NOT_AVAILABLE"
}

// Reason is the ErrorInfo reason of a service's error, "" when it has none.
func Reason(err error) string {
	s, ok := status.FromError(err)
	if !ok {
		return ""
	}
	for _, d := range s.Details() {
		if info, ok := d.(*errdetails.ErrorInfo); ok {
			return info.GetReason()
		}
	}
	return ""
}
