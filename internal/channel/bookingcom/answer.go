// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package bookingcom

// answer.go is OTA_HotelResNotifRS, the answer to a reservation message.
//
// OTA's RS is either Success, with each reservation and the hotel's
// confirmation number for it, or Errors -- never both. Every notification is
// idempotent, so when any of a message's reservations fails, the answer is
// Errors (one per failure, naming its reservation id) and the channel
// delivers the whole message again: the ones already applied are found, not
// applied twice.

import (
	"encoding/xml"
	"errors"
	"fmt"

	"github.com/shokki-engineering/aperio/internal/booking"
	"github.com/shokki-engineering/aperio/internal/channel"
)

// resNotifRS is OTA_HotelResNotifRS.
type resNotifRS struct {
	XMLName      xml.Name        `xml:"OTA_HotelResNotifRS"`
	Xmlns        string          `xml:"xmlns,attr"`
	EchoToken    string          `xml:"EchoToken,attr,omitempty"`
	TimeStamp    string          `xml:"TimeStamp,attr"`
	Version      string          `xml:"Version,attr"`
	Success      *struct{}       `xml:"Success"`
	Errors       *otaErrors      `xml:"Errors"`
	Reservations *rsReservations `xml:"HotelReservations"`
}

// otaErrors is Errors; absent on success.
type otaErrors struct {
	Errors []otaError `xml:"Error"`
}

// rsReservations is HotelReservations; absent on errors.
type rsReservations struct {
	Reservations []rsReservation `xml:"HotelReservation"`
}

// otaError is Error: Type from OTA's EWT list (3 business rule, 10 required
// field missing, 12 processing exception), Code from its ERR list.
type otaError struct {
	Type      string `xml:"Type,attr"`
	Code      string `xml:"Code,attr"`
	ShortText string `xml:"ShortText,attr"`
	Text      string `xml:",chardata"`
}

// rsReservation is HotelReservation in the answer: the channel's id and the
// hotel's confirmation number (ResID_Type 10, the stay's reference).
type rsReservation struct {
	UniqueID uniqueID        `xml:"UniqueID"`
	IDs      []reservationID `xml:"ResGlobalInfo>HotelReservationIDs>HotelReservationID"`
}

// answer is the RS to in, whose results are results, or to a message that
// could not be read (parseErr).
func (a *Adapter) answer(in *channel.Inbound, results []channel.Result, parseErr error) ([]byte, error) {
	rs := resNotifRS{Xmlns: namespace, TimeStamp: a.stamp(), Version: version}
	if in != nil {
		rs.EchoToken = in.Token
	}
	errs := &otaErrors{}
	if parseErr != nil {
		errs.Errors = append(errs.Errors, otaErrorOf("", parseErr))
	}
	for _, r := range results {
		if r.Err != nil {
			errs.Errors = append(errs.Errors, otaErrorOf(r.Notification.Reference, r.Err))
		}
	}
	if len(errs.Errors) > 0 {
		rs.Errors = errs
		return marshal(rs)
	}
	rs.Success, rs.Reservations = &struct{}{}, &rsReservations{}
	for _, r := range results {
		out := rsReservation{
			UniqueID: uniqueID{Type: "14", ID: r.Notification.Reference},
			IDs:      []reservationID{{Type: "14", Value: r.Notification.Reference}},
		}
		if r.Reservation != "" {
			out.IDs = append(out.IDs, reservationID{Type: "10", Value: booking.Reference(r.Reservation)})
		}
		rs.Reservations.Reservations = append(rs.Reservations.Reservations, out)
	}
	return marshal(rs)
}

// otaErrorOf is err as an OTA Error about reservation ref.
func otaErrorOf(ref string, err error) otaError {
	e := otaError{Type: "12", Code: "450", ShortText: "Unable to process", Text: err.Error()}
	switch {
	case errors.Is(err, channel.ErrUnknownHotel):
		e.Type, e.Code, e.ShortText = "3", "392", "Invalid hotel code"
	case errors.Is(err, channel.ErrUnknownRoomType):
		e.Type, e.Code, e.ShortText = "3", "402", "Invalid room type"
	case errors.Is(err, channel.ErrMissing):
		e.Type, e.Code, e.ShortText = "10", "321", "Required field missing"
	case errors.Is(err, channel.ErrUnknownReservation):
		e.Type, e.Code, e.ShortText = "3", "245", "Invalid confirmation or cancellation number"
	case channel.Unavailable(err):
		e.Type, e.Code, e.ShortText = "3", "322", "No availability"
	}
	if ref != "" {
		e.Text = fmt.Sprintf("reservation %s: %s", ref, e.Text)
	}
	return e
}

// marshal is v as an XML document.
func marshal(v any) ([]byte, error) {
	out, err := xml.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, err
	}
	return append([]byte(xml.Header), append(out, '\n')...), nil
}
