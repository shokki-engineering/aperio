// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package channel connects aperio to the channels that sell its rooms --
// Booking.com first, Airbnb and MakeMyTrip once their route is chosen -- in
// aperio's own terms, whatever each channel speaks.
//
// Each channel is an Adapter: it reads the channel's reservation messages
// into Notifications and answers them, and writes aperio's rates and
// availability as the channel's messages. What a notification does to aperio
// is the same for every channel and is this package's (Sync): a new
// reservation becomes a stay through the StayService's gRPC methods, its
// guest found or added through the GuestService, idempotent on the
// channel and its reservation id; a modification updates the stay, or
// rebooks it when its dates or rooms change; a cancellation cancels it. What
// is sent out is computed here too (Outbound): rates from the PricingService,
// so a season or a rule reaches every channel at once, and rooms left from
// freebusy.
package channel

import (
	"github.com/shokki-engineering/aperio/protobuf/generated/go/type/v1/typepbv1"
	"google.golang.org/genproto/googleapis/type/date"
	"google.golang.org/genproto/googleapis/type/money"
)

// Adapter is one channel's messages: the channel's side of what Sync and
// Outbound do in aperio's.
type Adapter interface {
	// Name is the channel as logs and errors name it, e.g. "bookingcom".
	Name() string
	// Channel is the channel aperio records its reservations as coming from.
	Channel() typepbv1.Channel

	// ParseReservations reads a reservation message the channel sent.
	ParseReservations(body []byte) (*Inbound, error)
	// AnswerReservations is the answer to in: what became of each of its
	// notifications. A message that could not be read (parseErr) is
	// answered too, with in nil.
	AnswerReservations(in *Inbound, results []Result, parseErr error) ([]byte, error)

	// RatesMessage is the message that sets rates on the channel for hotel,
	// the channel's code for a venue.
	RatesMessage(hotel string, rates []Rate) ([]byte, error)
	// AvailabilityMessage is the message that sets the rooms left to sell on
	// the channel for hotel.
	AvailabilityMessage(hotel string, avail []Availability) ([]byte, error)
}

// Inbound is one reservation message from a channel.
type Inbound struct {
	// Token is the message's own id, echoed in the answer (OTA EchoToken).
	Token string
	// Notifications are its reservations, in the order sent.
	Notifications []Notification
}

// Action is what a notification does to a reservation.
type Action int

// The actions a channel sends.
const (
	// ActionNew is a new reservation.
	ActionNew Action = iota + 1
	// ActionModify changes a reservation the channel sent before.
	ActionModify
	// ActionCancel cancels one.
	ActionCancel
)

// String is the action as logs say it.
func (a Action) String() string {
	return map[Action]string{ActionNew: "new", ActionModify: "modify", ActionCancel: "cancel"}[a]
}

// Notification is one reservation as a channel sent it, in aperio's terms.
type Notification struct {
	Action Action
	// Hotel is the channel's code for the venue; empty when the message
	// does not say (a cancellation may not).
	Hotel string
	// Reference is the channel's reservation id (OTA ResID_Value), which
	// every later message about the reservation names.
	Reference string
	// Arrival and Departure are the stay's dates; the nights are those
	// before Departure.
	Arrival, Departure *date.Date
	// Rooms are the rooms booked, one entry per room stay the channel sent.
	Rooms []Room
	// Guest is who booked.
	Guest Guest
	// Total is what the channel says the stay costs; nil when unsaid.
	Total *money.Money
	// Requests are what the guest asked for, one per request.
	Requests []string
	// Comments are the channel's remarks on the reservation.
	Comments string
}

// Room is rooms of one room type, as the channel codes it.
type Room struct {
	// TypeCode is the channel's room type code (OTA RoomTypeCode).
	TypeCode string
	// RatePlanCode is the rate plan sold (OTA RatePlanCode).
	RatePlanCode string
	// Units is how many rooms; one when unsaid.
	Units int32
	// Adults and Children are the guests in them.
	Adults, Children int32
	// Total is what they cost; nil when unsaid.
	Total *money.Money
}

// Guest is the person a reservation is for.
type Guest struct {
	GivenName, Surname string
	// Phone is as the channel sent it; Email may be the channel's alias.
	Phone, Email string
	// ArrivalTime is when the guest said they would arrive, "20:00:00".
	ArrivalTime string
}

// DisplayName is the guest's name as aperio shows it.
func (g Guest) DisplayName() string {
	switch {
	case g.GivenName == "":
		return g.Surname
	case g.Surname == "":
		return g.GivenName
	}
	return g.GivenName + " " + g.Surname
}

// Result is what became of one notification.
type Result struct {
	Notification Notification
	// Reservation is the aperio stay it made, changed or cancelled.
	// Format: venues/{venue}/reservations/{reservation}
	Reservation string
	// Err is why it was not applied; nil when it was.
	Err error
}

// Rate is a room type's price for a run of dates on a rate plan.
type Rate struct {
	TypeCode, RatePlanCode string
	// Start and End are the first and the last date it is for (inclusive).
	Start, End *date.Date
	// Amount is the price of a room for a night, before tax.
	Amount *money.Money
}

// Availability is the rooms of a room type left to sell for a run of dates.
type Availability struct {
	TypeCode string
	// Start and End are the first and the last night it is for (inclusive).
	Start, End *date.Date
	// Free is how many rooms are left.
	Free int32
}
