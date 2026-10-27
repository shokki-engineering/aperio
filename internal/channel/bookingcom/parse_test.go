// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package bookingcom

// parse_test.go reads the sample OTA_HotelResNotifRQ messages in testdata:
// a new reservation, a change of guests, a change of dates, a cancellation,
// and messages that cannot be applied.

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/shokki-engineering/aperio/internal/channel"
)

// fixture reads testdata/name.
func fixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestParseNew(t *testing.T) {
	in, err := parse(fixture(t, "new.xml"))
	if err != nil {
		t.Fatal(err)
	}
	if in.Token != "echo-new-0001" || len(in.Notifications) != 1 {
		t.Fatalf("inbound = %+v", in)
	}
	n := in.Notifications[0]
	if n.Action != channel.ActionNew || n.Reference != "4105349872" || n.Hotel != "7777001" {
		t.Errorf("action %v, reference %q, hotel %q", n.Action, n.Reference, n.Hotel)
	}
	if n.Arrival.GetDay() != 10 || n.Departure.GetDay() != 12 || n.Arrival.GetMonth() != 3 {
		t.Errorf("dates %v to %v", n.Arrival, n.Departure)
	}
	r := n.Rooms[0]
	if len(n.Rooms) != 1 || r.TypeCode != "SV-DLX" || r.RatePlanCode != "BAR" || r.Units != 1 || r.Adults != 2 {
		t.Errorf("rooms = %+v", n.Rooms)
	}
	if n.Total.GetUnits() != 18000 || n.Total.GetCurrencyCode() != "INR" {
		t.Errorf("total = %v", n.Total)
	}
	g := n.Guest
	if g.DisplayName() != "John Doe" || g.Phone != "+91 98000 00001" || g.Email != "john.doe@guest.example.com" || g.ArrivalTime != "20:00:00" {
		t.Errorf("guest = %+v", g)
	}
	if len(n.Requests) != 1 || n.Comments == "" {
		t.Errorf("requests %q, comments %q", n.Requests, n.Comments)
	}
}

func TestParseModifyAndCancel(t *testing.T) {
	for _, tc := range []struct {
		file   string
		action channel.Action
	}{{"modify.xml", channel.ActionModify}, {"modify_dates.xml", channel.ActionModify}, {"cancel.xml", channel.ActionCancel}} {
		in, err := parse(fixture(t, tc.file))
		if err != nil {
			t.Fatalf("%s: %v", tc.file, err)
		}
		n := in.Notifications[0]
		if n.Action != tc.action || n.Reference != "4105349872" || n.Hotel != "7777001" {
			t.Errorf("%s: action %v, reference %q, hotel %q", tc.file, n.Action, n.Reference, n.Hotel)
		}
	}
	in, _ := parse(fixture(t, "modify.xml"))
	if a := in.Notifications[0].Rooms[0].Adults; a != 3 {
		t.Errorf("modify: adults = %d, want 3", a)
	}
	in, _ = parse(fixture(t, "modify_dates.xml"))
	if d := in.Notifications[0].Departure.GetDay(); d != 13 {
		t.Errorf("modify_dates: departure day = %d, want 13", d)
	}
}

func TestParseRefuses(t *testing.T) {
	for name, body := range map[string]string{
		"not xml":   "hello",
		"empty":     `<OTA_HotelResNotifRQ xmlns="http://www.opentravel.org/OTA/2003/05"/>`,
		"bad state": `<OTA_HotelResNotifRQ><HotelReservations><HotelReservation ResStatus="Quote"/></HotelReservations></OTA_HotelResNotifRQ>`,
	} {
		if _, err := parse([]byte(body)); err == nil {
			t.Errorf("%s: parsed", name)
		}
	}
	split := `<OTA_HotelResNotifRQ><HotelReservations><HotelReservation ResStatus="Book"><RoomStays>
	<RoomStay><TimeSpan Start="2027-03-10" End="2027-03-12"/></RoomStay>
	<RoomStay><TimeSpan Start="2027-03-11" End="2027-03-12"/></RoomStay></RoomStays></HotelReservation></HotelReservations></OTA_HotelResNotifRQ>`
	if _, err := parse([]byte(split)); !errors.Is(err, channel.ErrUnsupported) {
		t.Errorf("rooms on different dates: err = %v, want ErrUnsupported", err)
	}
}
