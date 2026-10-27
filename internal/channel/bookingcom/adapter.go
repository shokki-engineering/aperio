// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package bookingcom is Booking.com as a channel: its OTA XML messages read
// and written for package channel. Reservations come in as
// OTA_HotelResNotifRQ and are answered with OTA_HotelResNotifRS (Handler);
// rates and rooms to sell go out as OTA_HotelRateAmountNotifRQ and
// OTA_HotelAvailNotifRQ. The XML is minimal hand-written structs, holding
// just the elements aperio reads or writes, named as OTA names them.
package bookingcom

import (
	"crypto/rand"
	"encoding/hex"
	"time"

	"github.com/shokki-engineering/aperio/internal/channel"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/type/v1/typepbv1"
)

const (
	// namespace is OTA 2003/05's, which every message is in.
	namespace = "http://www.opentravel.org/OTA/2003/05"
	// version is the message version sent.
	version = "1.0"
	// Prefix starts the ids of the stays made from Booking.com reservations.
	Prefix = "bookingcom"
)

// Adapter is Booking.com's channel.Adapter.
type Adapter struct {
	// Now is the clock messages are stamped with; time.Now when nil.
	Now func() time.Time
	// Token makes a sent message's EchoToken; random when nil.
	Token func() string
}

var _ channel.Adapter = (*Adapter)(nil)

// Name is "bookingcom".
func (a *Adapter) Name() string { return Prefix }

// Channel is CHANNEL_BOOKING_COM.
func (a *Adapter) Channel() typepbv1.Channel { return typepbv1.Channel_CHANNEL_BOOKING_COM }

// ParseReservations reads an OTA_HotelResNotifRQ.
func (a *Adapter) ParseReservations(body []byte) (*channel.Inbound, error) { return parse(body) }

// AnswerReservations is the OTA_HotelResNotifRS to in.
func (a *Adapter) AnswerReservations(in *channel.Inbound, results []channel.Result, parseErr error) ([]byte, error) {
	return a.answer(in, results, parseErr)
}

// RatesMessage is an OTA_HotelRateAmountNotifRQ.
func (a *Adapter) RatesMessage(hotel string, rates []channel.Rate) ([]byte, error) {
	return a.rates(hotel, rates)
}

// AvailabilityMessage is an OTA_HotelAvailNotifRQ.
func (a *Adapter) AvailabilityMessage(hotel string, avail []channel.Availability) ([]byte, error) {
	return a.availability(hotel, avail)
}

// stamp is now as an OTA TimeStamp.
func (a *Adapter) stamp() string {
	now := time.Now
	if a.Now != nil {
		now = a.Now
	}
	return now().UTC().Format(time.RFC3339)
}

// token is a new EchoToken.
func (a *Adapter) token() string {
	if a.Token != nil {
		return a.Token()
	}
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
