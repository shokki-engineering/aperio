// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package bookingcom

// resnotif.go is OTA_HotelResNotifRQ as Booking.com sends it: just the
// elements and attributes aperio reads, by their OTA names. Anything else in
// the message is ignored.

import "encoding/xml"

// resNotifRQ is OTA_HotelResNotifRQ.
type resNotifRQ struct {
	XMLName xml.Name `xml:"OTA_HotelResNotifRQ"`
	// EchoToken is the message's id, echoed in the answer.
	EchoToken string `xml:"EchoToken,attr"`
	// ResStatus is the transaction: Commit, Book, Modify, Cancel.
	ResStatus string `xml:"ResStatus,attr"`
	// Reservations are HotelReservations/HotelReservation.
	Reservations []hotelReservation `xml:"HotelReservations>HotelReservation"`
}

// hotelReservation is HotelReservation.
type hotelReservation struct {
	// ResStatus is the reservation's own status (Book, Modify, Cancel,
	// Reserved, Modified, Cancelled); the message's when absent.
	ResStatus string `xml:"ResStatus,attr"`
	// UniqueID is the channel's id for it, when there are no
	// HotelReservationIDs.
	UniqueID   uniqueID   `xml:"UniqueID"`
	RoomStays  []roomStay `xml:"RoomStays>RoomStay"`
	ResGuests  []resGuest `xml:"ResGuests>ResGuest"`
	GlobalInfo globalInfo `xml:"ResGlobalInfo"`
}

// uniqueID is UniqueID: Type 14 is a reservation.
type uniqueID struct {
	Type string `xml:"Type,attr"`
	ID   string `xml:"ID,attr"`
}

// roomStay is RoomStay: rooms of one type for a span of nights.
type roomStay struct {
	RoomTypes   []roomType   `xml:"RoomTypes>RoomType"`
	RatePlans   []ratePlan   `xml:"RatePlans>RatePlan"`
	RoomRates   []roomRate   `xml:"RoomRates>RoomRate"`
	GuestCounts []guestCount `xml:"GuestCounts>GuestCount"`
	TimeSpan    timeSpan     `xml:"TimeSpan"`
	Total       amount       `xml:"Total"`
	// Hotel is BasicPropertyInfo/@HotelCode.
	Hotel    basicPropertyInfo `xml:"BasicPropertyInfo"`
	Requests []text            `xml:"SpecialRequests>SpecialRequest"`
}

// roomType is RoomType.
type roomType struct {
	Code      string `xml:"RoomTypeCode,attr"`
	UnitCount int32  `xml:"NumberOfUnits,attr"`
}

// ratePlan is RatePlan.
type ratePlan struct {
	Code string `xml:"RatePlanCode,attr"`
}

// roomRate is RoomRate: the room type, rate plan and rooms a room stay is.
type roomRate struct {
	RoomTypeCode  string `xml:"RoomTypeCode,attr"`
	RatePlanCode  string `xml:"RatePlanCode,attr"`
	NumberOfUnits int32  `xml:"NumberOfUnits,attr"`
}

// guestCount is GuestCount: AgeQualifyingCode 10 adults, 8 children, 7
// infants.
type guestCount struct {
	AgeQualifyingCode string `xml:"AgeQualifyingCode,attr"`
	Count             int32  `xml:"Count,attr"`
}

// timeSpan is TimeSpan: Start the arrival, End the departure, YYYY-MM-DD.
type timeSpan struct {
	Start string `xml:"Start,attr"`
	End   string `xml:"End,attr"`
}

// amount is an OTA amount: Total, Base.
type amount struct {
	AmountAfterTax  string `xml:"AmountAfterTax,attr"`
	AmountBeforeTax string `xml:"AmountBeforeTax,attr"`
	CurrencyCode    string `xml:"CurrencyCode,attr"`
}

// basicPropertyInfo is BasicPropertyInfo.
type basicPropertyInfo struct {
	HotelCode string `xml:"HotelCode,attr"`
}

// text is an element with a Text child: SpecialRequest, Comment.
type text struct {
	Text string `xml:"Text"`
}

// resGuest is ResGuest: the guest's profile.
type resGuest struct {
	ArrivalTime string   `xml:"ArrivalTime,attr"`
	Primary     string   `xml:"PrimaryIndicator,attr"`
	Customer    customer `xml:"Profiles>ProfileInfo>Profile>Customer"`
}

// customer is Customer.
type customer struct {
	GivenName string      `xml:"PersonName>GivenName"`
	Surname   string      `xml:"PersonName>Surname"`
	Telephone []telephone `xml:"Telephone"`
	Email     []string    `xml:"Email"`
}

// telephone is Telephone.
type telephone struct {
	PhoneNumber string `xml:"PhoneNumber,attr"`
}

// globalInfo is ResGlobalInfo.
type globalInfo struct {
	IDs      []reservationID `xml:"HotelReservationIDs>HotelReservationID"`
	Total    amount          `xml:"Total"`
	Comments []text          `xml:"Comments>Comment"`
	// Hotel is BasicPropertyInfo/@HotelCode, where a cancellation carries it.
	Hotel basicPropertyInfo `xml:"BasicPropertyInfo"`
}

// reservationID is HotelReservationID: ResID_Type 14 is the channel's
// reservation id, 10 the hotel's confirmation number.
type reservationID struct {
	Type  string `xml:"ResID_Type,attr"`
	Value string `xml:"ResID_Value,attr"`
}
