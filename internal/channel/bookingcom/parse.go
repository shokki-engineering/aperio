// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package bookingcom

// parse.go reads an OTA_HotelResNotifRQ into channel notifications: the
// reservation id (ResGlobalInfo's HotelReservationID of ResID_Type 14, else
// UniqueID), the action (HotelReservation@ResStatus, else the message's),
// the hotel, the rooms by their codes, the dates, the guest and the totals.

import (
	"encoding/xml"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/shokki-engineering/aperio/internal/channel"
	"github.com/shokki-engineering/aperio/internal/pricing"
	"google.golang.org/genproto/googleapis/type/date"
	"google.golang.org/genproto/googleapis/type/money"
	"google.golang.org/protobuf/proto"
)

// actions are the OTA statuses a reservation is sent with: TransactionAction
// (Book, Commit, Modify, Cancel) and HotelResStatus (Reserved, Modified,
// Cancelled), by lower-cased name.
var actions = map[string]channel.Action{
	"book": channel.ActionNew, "commit": channel.ActionNew, "initiate": channel.ActionNew,
	"reserved": channel.ActionNew, "requested": channel.ActionNew, "new": channel.ActionNew,
	"modify": channel.ActionModify, "modified": channel.ActionModify,
	"cancel": channel.ActionCancel, "cancelled": channel.ActionCancel,
}

// parse reads body, an OTA_HotelResNotifRQ.
func parse(body []byte) (*channel.Inbound, error) {
	var rq resNotifRQ
	if err := xml.Unmarshal(body, &rq); err != nil {
		return nil, fmt.Errorf("bookingcom: not an OTA_HotelResNotifRQ: %w", err)
	}
	in := &channel.Inbound{Token: rq.EchoToken}
	for i, hr := range rq.Reservations {
		n, err := notification(rq.ResStatus, hr)
		if err != nil {
			return nil, fmt.Errorf("bookingcom: HotelReservation %d: %w", i+1, err)
		}
		in.Notifications = append(in.Notifications, n)
	}
	if len(in.Notifications) == 0 {
		return nil, fmt.Errorf("bookingcom: %w: no HotelReservation", channel.ErrMissing)
	}
	return in, nil
}

// notification is hr, sent in a message of status status.
func notification(status string, hr hotelReservation) (channel.Notification, error) {
	st := hr.ResStatus
	if st == "" {
		st = status
	}
	act, ok := actions[strings.ToLower(st)]
	if !ok {
		return channel.Notification{}, fmt.Errorf("%w: ResStatus %q", channel.ErrUnsupported, st)
	}
	n := channel.Notification{Action: act, Reference: reference(hr), Hotel: hr.GlobalInfo.Hotel.HotelCode, Total: amountOf(hr.GlobalInfo.Total)}
	var err error
	for _, rs := range hr.RoomStays {
		if n.Hotel == "" {
			n.Hotel = rs.Hotel.HotelCode
		}
		arrive, aerr := day(rs.TimeSpan.Start)
		depart, derr := day(rs.TimeSpan.End)
		if aerr != nil || derr != nil {
			return n, fmt.Errorf("%w: TimeSpan %q to %q", channel.ErrMissing, rs.TimeSpan.Start, rs.TimeSpan.End)
		}
		if n.Arrival == nil {
			n.Arrival, n.Departure = arrive, depart
		} else if !proto.Equal(n.Arrival, arrive) || !proto.Equal(n.Departure, depart) {
			// One stay is one span of nights; rooms on other dates are
			// another stay, which the channel would have to send as one.
			return n, fmt.Errorf("%w: rooms for different dates", channel.ErrUnsupported)
		}
		n.Rooms = append(n.Rooms, room(rs))
		for _, q := range rs.Requests {
			n.Requests = append(n.Requests, q.Text)
		}
	}
	if n.Total == nil {
		n.Total, err = sumRooms(n.Rooms)
	}
	for _, g := range hr.ResGuests {
		if g.Primary == "true" || g.Primary == "1" || n.Guest == (channel.Guest{}) {
			n.Guest = guest(g)
		}
	}
	var comments []string
	for _, c := range hr.GlobalInfo.Comments {
		comments = append(comments, strings.TrimSpace(c.Text))
	}
	n.Comments = strings.Join(comments, "\n")
	return n, err
}

// reference is the channel's id for hr.
func reference(hr hotelReservation) string {
	for _, id := range hr.GlobalInfo.IDs {
		if id.Type == "14" && id.Value != "" {
			return strings.TrimSpace(id.Value)
		}
	}
	return strings.TrimSpace(hr.UniqueID.ID)
}

// room is the rooms rs books.
func room(rs roomStay) channel.Room {
	r := channel.Room{Units: 1, Total: amountOf(rs.Total)}
	if len(rs.RoomTypes) > 0 {
		r.TypeCode, r.Units = rs.RoomTypes[0].Code, max(rs.RoomTypes[0].UnitCount, 1)
	}
	if len(rs.RatePlans) > 0 {
		r.RatePlanCode = rs.RatePlans[0].Code
	}
	if len(rs.RoomRates) > 0 {
		rr := rs.RoomRates[0]
		r.TypeCode = cmp(r.TypeCode, rr.RoomTypeCode)
		r.RatePlanCode = cmp(r.RatePlanCode, rr.RatePlanCode)
		if rr.NumberOfUnits > 0 {
			r.Units = rr.NumberOfUnits
		}
	}
	for _, gc := range rs.GuestCounts {
		switch gc.AgeQualifyingCode {
		case "10", "":
			r.Adults += gc.Count
		default:
			r.Children += gc.Count
		}
	}
	return r
}

// cmp is a, or b when a is empty.
func cmp(a, b string) string { return map[bool]string{true: b, false: a}[a == ""] }

// guest is the guest g's profile names.
func guest(g resGuest) channel.Guest {
	c := g.Customer
	out := channel.Guest{GivenName: strings.TrimSpace(c.GivenName), Surname: strings.TrimSpace(c.Surname), ArrivalTime: g.ArrivalTime}
	if len(c.Telephone) > 0 {
		out.Phone = c.Telephone[0].PhoneNumber
	}
	if len(c.Email) > 0 {
		out.Email = strings.TrimSpace(c.Email[0])
	}
	return out
}

// day is s, YYYY-MM-DD (or a date-time, whose date is taken).
func day(s string) (*date.Date, error) {
	t, err := time.Parse(time.DateOnly, s[:min(len(s), 10)])
	if err != nil {
		return nil, err
	}
	return &date.Date{Year: int32(t.Year()), Month: int32(t.Month()), Day: int32(t.Day())}, nil
}

// amountOf is a, after tax when it says, else before; nil when it says
// neither or is not a number.
func amountOf(a amount) *money.Money {
	v := cmp(a.AmountAfterTax, a.AmountBeforeTax)
	r, ok := new(big.Rat).SetString(v)
	if v == "" || !ok {
		return nil
	}
	return pricing.Money(r, a.CurrencyCode)
}

// sumRooms is the rooms' totals added, nil when any is unsaid.
func sumRooms(rooms []channel.Room) (*money.Money, error) {
	sum, cur := new(big.Rat), ""
	for _, r := range rooms {
		if r.Total == nil {
			return nil, nil
		}
		if cur != "" && r.Total.GetCurrencyCode() != cur {
			return nil, fmt.Errorf("%w: room totals in %s and %s", channel.ErrUnsupported, cur, r.Total.GetCurrencyCode())
		}
		cur = r.Total.GetCurrencyCode()
		sum.Add(sum, pricing.Rat(r.Total))
	}
	if cur == "" {
		return nil, nil
	}
	return pricing.Money(sum, cur), nil
}
