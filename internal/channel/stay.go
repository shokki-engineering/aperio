// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package channel

// stay.go is a notification in aperio's terms: the stay it books, its room
// types mapped from the channel's codes, and the guest it is for -- the
// contact with the guest's phone or email, or one added.

import (
	"context"
	"fmt"
	"strings"

	"github.com/shokki-engineering/aperio/internal/service/guest"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/guest/v1/guestpbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/stay/v1/staypbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/type/v1/typepbv1"
)

// reservation is the stay n books at h, without its contact.
func (s *Sync) reservation(h Hotel, n Notification) (*staypbv1.Reservation, error) {
	if n.Arrival == nil || n.Departure == nil {
		return nil, fmt.Errorf("%w: the stay's dates", ErrMissing)
	}
	if len(n.Rooms) == 0 {
		return nil, fmt.Errorf("%w: the rooms", ErrMissing)
	}
	r := &staypbv1.Reservation{
		Channel: s.Channel, ChannelReference: n.Reference,
		ArrivalDate: n.Arrival, DepartureDate: n.Departure,
		Total: n.Total, Notes: n.Comments,
		RoomStays: roomStays(n), ResGuests: resGuests(n.Guest),
	}
	for _, room := range n.Rooms {
		space := h.space(room.TypeCode)
		if space == "" {
			return nil, fmt.Errorf("%w: %q at hotel %q", ErrUnknownRoomType, room.TypeCode, h.Code)
		}
		r.Adults += room.Adults
		r.Children += room.Children
		r.Rooms = addRooms(r.Rooms, space, room.RatePlanCode, max(room.Units, 1))
	}
	if r.Adults == 0 {
		// A channel that sends no guest counts: a guest a room.
		for _, line := range r.Rooms {
			r.Adults += line.GetRooms()
		}
	}
	for _, q := range n.Requests {
		if q = strings.TrimSpace(q); q != "" {
			r.SpecialRequests = append(r.SpecialRequests, &typepbv1.SpecialRequest{Summary: q})
		}
	}
	return r, nil
}

// addRooms adds units rooms of space on ratePlan to lines: to the line of
// the same room type and rate plan, else as a line of their own.
func addRooms(lines []*staypbv1.RoomLine, space, ratePlan string, units int32) []*staypbv1.RoomLine {
	for _, l := range lines {
		if l.GetSpace() == space && l.GetRatePlanCode() == ratePlan {
			l.Rooms += units
			return lines
		}
	}
	return append(lines, &staypbv1.RoomLine{Space: space, Rooms: units, RatePlanCode: ratePlan})
}

// guest is the contact g is: the one with g's phone, else with g's email,
// else one added for g. reference names a guest the channel gave no name.
func (s *Sync) guest(ctx context.Context, g Guest, reference string) (string, error) {
	phone := guest.Normalize(g.Phone)
	email := strings.ToLower(strings.TrimSpace(g.Email))
	for _, filter := range []string{phoneFilter(phone), emailFilter(email)} {
		if filter == "" {
			continue
		}
		found, err := s.Guests.ListContacts(ctx, &guestpbv1.ListContactsRequest{PageSize: 1, Filter: filter})
		if err != nil {
			return "", err
		}
		if len(found.GetContacts()) > 0 {
			return found.GetContacts()[0].GetName(), nil
		}
	}
	c := &guestpbv1.Contact{Kind: guestpbv1.ContactKind_CONTACT_KIND_PERSON, DisplayName: g.DisplayName()}
	if c.DisplayName == "" {
		c.DisplayName = s.Prefix + " guest " + reference
	}
	if phone != "" {
		c.Phones = []*typepbv1.Phone{{Number: phone, Kind: typepbv1.PhoneKind_PHONE_KIND_MOBILE}}
	}
	if email != "" {
		c.Emails = []string{email}
	}
	made, err := s.Guests.CreateContact(ctx, &guestpbv1.CreateContactRequest{Contact: c})
	if err != nil {
		return "", err
	}
	return made.GetName(), nil
}

// phoneFilter finds a contact by phone; "" for no phone.
func phoneFilter(phone string) string {
	if len(strings.TrimPrefix(phone, "+")) < 6 {
		return "" // too short to be anyone's alone
	}
	return fmt.Sprintf("phone_numbers:%q", phone)
}

// emailFilter finds a contact by email; "" for no email.
func emailFilter(email string) string {
	if !strings.Contains(email, "@") {
		return ""
	}
	return fmt.Sprintf("emails:%q", email)
}
