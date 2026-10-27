// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package channel

// book.go is a channel's new reservation made a stay, and a modification
// applied: in place when only its guests, requests or price changed, else --
// its dates and rooms are booked in freebusy and locked (the stay service's
// BeforeUpdateReservation) -- by cancelling the stay and booking the
// reservation as it now is. The channel's reservation is the one that
// stands, so the old rooms are given back first: the new stay may need them.
//
// A channel's stay is confirmed as soon as it is made: the channel has taken
// the guest's guarantee, and no advance is asked of them through aperio.

import (
	"context"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/shokki-engineering/aperio/protobuf/generated/go/stay/v1/staypbv1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
)

// book makes the stay n is, or finds the one an earlier delivery made.
func (s *Sync) book(ctx context.Context, h Hotel, n Notification) (string, error) {
	f, err := s.find(ctx, h.Venue, n.Reference)
	if err != nil {
		return "", err
	}
	if f.live != nil {
		return s.confirm(ctx, f.live)
	}
	return s.create(ctx, h, n, len(f.all))
}

// modify changes the stay n names to be n.
func (s *Sync) modify(ctx context.Context, h Hotel, n Notification) (string, error) {
	f, err := s.find(ctx, h.Venue, n.Reference)
	if err != nil {
		return "", err
	}
	if f.live == nil {
		// Never had (its new reservation was lost) or cancelled: book it as
		// it now is.
		return s.create(ctx, h, n, len(f.all))
	}
	want, err := s.reservation(h, n)
	if err != nil {
		return "", err
	}
	if sameBooking(f.live, want) {
		want.Name, want.Contact = f.live.GetName(), f.live.GetContact()
		out, err := s.Stays.UpdateReservation(ctx, &staypbv1.UpdateReservationRequest{
			Reservation: want,
			UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{
				"adults", "children", "special_requests", "total", "room_stays", "res_guests", "notes",
			}},
		})
		if err != nil {
			return "", err
		}
		return out.GetName(), nil
	}
	if _, err := s.cancelStay(ctx, f.live, "changed on "+s.Prefix); err != nil {
		return "", err
	}
	return s.create(ctx, h, n, len(f.all))
}

// sameBooking reports whether want books what r does: the same dates and
// room lines, which a stay cannot change in place.
func sameBooking(r, want *staypbv1.Reservation) bool {
	return proto.Equal(r.GetArrivalDate(), want.GetArrivalDate()) &&
		proto.Equal(r.GetDepartureDate(), want.GetDepartureDate()) &&
		slices.EqualFunc(r.GetRooms(), want.GetRooms(), func(a, b *staypbv1.RoomLine) bool { return proto.Equal(a, b) })
}

// create books n as a new stay, the earlier'th made for its reference, and
// confirms it.
func (s *Sync) create(ctx context.Context, h Hotel, n Notification, earlier int) (string, error) {
	r, err := s.reservation(h, n)
	if err != nil {
		return "", err
	}
	contact, err := s.guest(ctx, n.Guest, n.Reference)
	if err != nil {
		return "", err
	}
	r.Contact = contact
	id := s.id(n.Reference, earlier)
	out, err := s.Stays.CreateReservation(ctx, &staypbv1.CreateReservationRequest{Parent: h.Venue, ReservationId: id, Reservation: r})
	if status.Code(err) == codes.AlreadyExists {
		// Another delivery of the same notification made it first.
		f, ferr := s.find(ctx, h.Venue, n.Reference)
		if ferr != nil || f.live == nil {
			return "", err
		}
		out = f.live
	} else if err != nil {
		return "", err
	}
	return s.confirm(ctx, out)
}

// confirm confirms r if it is still pending.
func (s *Sync) confirm(ctx context.Context, r *staypbv1.Reservation) (string, error) {
	if r.GetState() != staypbv1.Reservation_STATE_PENDING {
		return r.GetName(), nil
	}
	out, err := s.Stays.ConfirmReservation(ctx, &staypbv1.ConfirmReservationRequest{Name: r.GetName()})
	if err != nil {
		return "", err
	}
	return out.GetName(), nil
}

// notID is what a reservation id may not hold.
var notID = regexp.MustCompile(`[^a-z0-9]+`)

// id is the id of the stay made for reference after earlier others:
// "bookingcom-4105349872", then "bookingcom-4105349872-2" when it is rebooked.
func (s *Sync) id(reference string, earlier int) string {
	ref := strings.Trim(notID.ReplaceAllString(strings.ToLower(reference), "-"), "-")
	id := s.Prefix + "-" + ref[:min(len(ref), 48)]
	if earlier > 0 {
		id += fmt.Sprintf("-%d", earlier+1)
	}
	return strings.Trim(id, "-")
}
