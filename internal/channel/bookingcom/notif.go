// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package bookingcom

// notif.go is the two messages aperio sends Booking.com:
// OTA_HotelRateAmountNotifRQ, a room type's price on a rate plan for runs of
// dates, and OTA_HotelAvailNotifRQ, the rooms of it left to sell
// (BookingLimit). StatusApplicationControl's Start and End are both
// inclusive. Rates are sent before tax (AmountBeforeTax): aperio's room
// prices are before GST, which Booking.com adds from the property's tax
// settings.

import (
	"encoding/xml"
	"fmt"

	"github.com/shokki-engineering/aperio/internal/channel"
	"github.com/shokki-engineering/aperio/internal/pricing"
	"google.golang.org/genproto/googleapis/type/date"
)

// rateAmountNotifRQ is OTA_HotelRateAmountNotifRQ.
type rateAmountNotifRQ struct {
	XMLName   xml.Name           `xml:"OTA_HotelRateAmountNotifRQ"`
	Xmlns     string             `xml:"xmlns,attr"`
	EchoToken string             `xml:"EchoToken,attr"`
	TimeStamp string             `xml:"TimeStamp,attr"`
	Version   string             `xml:"Version,attr"`
	Messages  rateAmountMessages `xml:"RateAmountMessages"`
}

// rateAmountMessages is RateAmountMessages: a hotel's rates.
type rateAmountMessages struct {
	HotelCode string              `xml:"HotelCode,attr"`
	Messages  []rateAmountMessage `xml:"RateAmountMessage"`
}

// rateAmountMessage is RateAmountMessage: one run's price.
type rateAmountMessage struct {
	Control statusApplicationControl `xml:"StatusApplicationControl"`
	// Amounts are Rates/Rate/BaseByGuestAmts/BaseByGuestAmt.
	Amounts []baseByGuestAmt `xml:"Rates>Rate>BaseByGuestAmts>BaseByGuestAmt"`
}

// baseByGuestAmt is BaseByGuestAmt: the room's price for a night.
type baseByGuestAmt struct {
	AmountBeforeTax string `xml:"AmountBeforeTax,attr"`
	CurrencyCode    string `xml:"CurrencyCode,attr"`
}

// statusApplicationControl is StatusApplicationControl: the dates and the
// room type (InvTypeCode) and rate plan a message is for.
type statusApplicationControl struct {
	Start        string `xml:"Start,attr"`
	End          string `xml:"End,attr"`
	InvTypeCode  string `xml:"InvTypeCode,attr"`
	RatePlanCode string `xml:"RatePlanCode,attr,omitempty"`
}

// availNotifRQ is OTA_HotelAvailNotifRQ.
type availNotifRQ struct {
	XMLName   xml.Name            `xml:"OTA_HotelAvailNotifRQ"`
	Xmlns     string              `xml:"xmlns,attr"`
	EchoToken string              `xml:"EchoToken,attr"`
	TimeStamp string              `xml:"TimeStamp,attr"`
	Version   string              `xml:"Version,attr"`
	Messages  availStatusMessages `xml:"AvailStatusMessages"`
}

// availStatusMessages is AvailStatusMessages: a hotel's rooms to sell.
type availStatusMessages struct {
	HotelCode string               `xml:"HotelCode,attr"`
	Messages  []availStatusMessage `xml:"AvailStatusMessage"`
}

// availStatusMessage is AvailStatusMessage: BookingLimit rooms to sell.
type availStatusMessage struct {
	BookingLimit int32                    `xml:"BookingLimit,attr"`
	Control      statusApplicationControl `xml:"StatusApplicationControl"`
}

// rates is the OTA_HotelRateAmountNotifRQ that sets rates at hotel.
func (a *Adapter) rates(hotel string, rates []channel.Rate) ([]byte, error) {
	rq := rateAmountNotifRQ{Xmlns: namespace, EchoToken: a.token(), TimeStamp: a.stamp(), Version: version}
	rq.Messages.HotelCode = hotel
	for _, r := range rates {
		rq.Messages.Messages = append(rq.Messages.Messages, rateAmountMessage{
			Control: statusApplicationControl{Start: ymd(r.Start), End: ymd(r.End), InvTypeCode: r.TypeCode, RatePlanCode: r.RatePlanCode},
			Amounts: []baseByGuestAmt{{
				AmountBeforeTax: pricing.Rat(r.Amount).FloatString(2), CurrencyCode: r.Amount.GetCurrencyCode(),
			}},
		})
	}
	return marshal(rq)
}

// availability is the OTA_HotelAvailNotifRQ that sets the rooms to sell at
// hotel.
func (a *Adapter) availability(hotel string, avail []channel.Availability) ([]byte, error) {
	rq := availNotifRQ{Xmlns: namespace, EchoToken: a.token(), TimeStamp: a.stamp(), Version: version}
	rq.Messages.HotelCode = hotel
	for _, av := range avail {
		rq.Messages.Messages = append(rq.Messages.Messages, availStatusMessage{
			BookingLimit: av.Free,
			Control:      statusApplicationControl{Start: ymd(av.Start), End: ymd(av.End), InvTypeCode: av.TypeCode},
		})
	}
	return marshal(rq)
}

// ymd is d as YYYY-MM-DD.
func ymd(d *date.Date) string {
	return fmt.Sprintf("%04d-%02d-%02d", d.GetYear(), d.GetMonth(), d.GetDay())
}
