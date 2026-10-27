// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package channel

// opentravel.go is a notification in OpenTravel's words, kept on the stay
// (its room_stays and res_guests) as the channel sent it: room types and
// rate plans by the channel's codes, guest counts, dates and totals -- what
// maps cleanly; the rest of the message is the channel's.

import (
	"fmt"

	"buf.build/gen/go/shokki-engineering/protobuf-ota/protocolbuffers/go/protobuf/opentravel/hotel/booking/reservation/v1/codes"
	"buf.build/gen/go/shokki-engineering/protobuf-ota/protocolbuffers/go/protobuf/opentravel/hotel/booking/reservation/v1/types"
	"google.golang.org/genproto/googleapis/type/date"
	"google.golang.org/genproto/googleapis/type/money"
	"google.golang.org/protobuf/proto"
)

// roomStays are n's rooms as OpenTravel RoomStays, one per room the channel
// sent, each for the primary guest (ResGuestRPH 1).
func roomStays(n Notification) []*types.RoomStay {
	var out []*types.RoomStay
	for i, r := range n.Rooms {
		rs := &types.RoomStay{
			IndexNumber:  proto.Int32(int32(i + 1)),
			RoomTypes:    []*types.RoomType{{RoomTypeCode: r.TypeCode, UnitCount: proto.Int32(max(r.Units, 1))}},
			GuestCounts:  guestCounts(r.Adults, r.Children),
			TimeSpan:     &types.DateTimeSpan{Start: day(n.Arrival), End: day(n.Departure)},
			ResGuestRphs: []string{"1"},
		}
		if r.RatePlanCode != "" {
			rs.RatePlans = []*types.RatePlan{{RatePlanCode: r.RatePlanCode}}
		}
		if r.Total != nil {
			rs.Total = &types.Total{PostTaxAmount: otaMoney(r.Total)}
		}
		if n.Hotel != "" {
			rs.BasicPropertyInfo = &types.RoomStay_BasicPropertyInfo{HotelCode: n.Hotel}
		}
		out = append(out, rs)
	}
	return out
}

// resGuests is g as the reservation's primary OpenTravel ResGuest; the
// contact holds their name, phone and email.
func resGuests(g Guest) []*types.ResGuest {
	rg := &types.ResGuest{
		ResGuestRph:       "1",
		AgeQualifyingCode: codes.AgeQualifyingCode_AGE_QUALIFYING_CODE_ADULT,
		PrimaryIndicator:  proto.Bool(true),
	}
	var h, m, s int32
	if n, _ := fmt.Sscanf(g.ArrivalTime, "%d:%d:%d", &h, &m, &s); n >= 2 {
		rg.ArrivalClock = &types.TimeOfDay{Hour: h, Minute: m, Second: s}
	}
	return []*types.ResGuest{rg}
}

// guestCounts are adults and children as OpenTravel GuestCounts (AQC 10 and
// 8); nil when neither is said.
func guestCounts(adults, children int32) *types.GuestCount {
	gc := &types.GuestCount{}
	if adults > 0 {
		gc.GuestCounts = append(gc.GuestCounts, &types.GuestCount_GuestCount{
			AgeQualifyingCode: codes.AgeQualifyingCode_AGE_QUALIFYING_CODE_ADULT, Count: proto.Int32(adults)})
	}
	if children > 0 {
		gc.GuestCounts = append(gc.GuestCounts, &types.GuestCount_GuestCount{
			AgeQualifyingCode: codes.AgeQualifyingCode_AGE_QUALIFYING_CODE_CHILD, Count: proto.Int32(children)})
	}
	if len(gc.GuestCounts) == 0 {
		return nil
	}
	return gc
}

// day is d as an OpenTravel date.
func day(d *date.Date) *types.DateOrTimeOrDateTime {
	return &types.DateOrTimeOrDateTime{Value: &types.DateOrTimeOrDateTime_Day{
		Day: &types.Date{Year: d.GetYear(), Month: d.GetMonth(), Day: d.GetDay()},
	}}
}

// otaMoney is m as OpenTravel money.
func otaMoney(m *money.Money) *types.Money {
	return &types.Money{CurrencyCode: m.GetCurrencyCode(), Units: m.GetUnits(), NanoUnits: m.GetNanos()}
}
