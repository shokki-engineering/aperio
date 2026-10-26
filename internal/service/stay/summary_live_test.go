// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package stay_test

// A live test of the front desk's day, read from the stay read model: who
// arrives, leaves and stays, rooms taken, a cancelled stay dropping out, and
// a stay found by the room type it booked. Run as stay_live_test.go says;
// with APERIO_TEST_OPENSEARCH_URL set (a single node, as in freebusy's
// compose), the same days are read from the read model's OpenSearch copy.

import (
	"context"
	"fmt"
	"testing"

	"github.com/shokki-engineering/aperio/protobuf/generated/go/stay/v1/staypbv1"
	"google.golang.org/genproto/googleapis/type/date"
)

func TestLiveSummarizeDay(t *testing.T) {
	checkDays(t, setup(t))
}

// checkDays books two stays with f and checks the days they make.
func checkDays(t *testing.T, f *fixture) {
	ctx := context.Background()
	book := func(id string, from, to int32) *staypbv1.Reservation {
		t.Helper()
		req := f.stay(id, 1)
		req.Reservation.ArrivalDate, req.Reservation.DepartureDate = &date.Date{Year: 2026, Month: 12, Day: from}, &date.Date{Year: 2026, Month: 12, Day: to}
		r, err := f.svc.CreateReservation(ctx, req)
		if err != nil {
			t.Fatalf("book %s: %v", id, err)
		}
		return r
	}
	a, b := book("a", 1, 3), book("b", 2, 4)
	day := func(d int32) *staypbv1.SummarizeDayResponse {
		t.Helper()
		got, err := f.svc.SummarizeDay(ctx, &staypbv1.SummarizeDayRequest{Venue: f.venue, Date: &date.Date{Year: 2026, Month: 12, Day: d}})
		if err != nil {
			t.Fatalf("summarize 12-%02d: %v", d, err)
		}
		return got
	}
	names := func(bs []*staypbv1.StayBrief) string {
		var out []string
		for _, x := range bs {
			out = append(out, x.GetReservation())
		}
		return fmt.Sprint(out)
	}

	got := day(2)
	if names(got.GetArrivals()) != fmt.Sprint([]string{b.GetName()}) || len(got.GetDepartures()) != 0 || len(got.GetStaying()) != 2 {
		t.Errorf("2 Dec: arrivals %s, departures %s, staying %s; want b arriving, a and b staying", names(got.GetArrivals()), names(got.GetDepartures()), names(got.GetStaying()))
	}
	if got.GetRoomsOccupied() != 2 || got.GetRoomsTotal() != 2 {
		t.Errorf("2 Dec: %d of %d rooms; want 2 of 2", got.GetRoomsOccupied(), got.GetRoomsTotal())
	}
	if s := got.GetArrivals()[0]; s.GetGuest() != "Test Guest" || s.GetRooms() != "1 Sea View" || s.GetNights() != 2 || len(s.GetReference()) != 6 {
		t.Errorf("b as the desk reads it: %+v", s)
	}
	if got := day(3); names(got.GetDepartures()) != fmt.Sprint([]string{a.GetName()}) || names(got.GetStaying()) != fmt.Sprint([]string{b.GetName()}) || got.GetRoomsOccupied() != 1 {
		t.Errorf("3 Dec: departures %s, staying %s, %d rooms; want a leaving, b staying, 1 room", names(got.GetDepartures()), names(got.GetStaying()), got.GetRoomsOccupied())
	}

	if _, err := f.svc.CancelReservation(ctx, &staypbv1.CancelReservationRequest{Name: b.GetName()}); err != nil {
		t.Fatalf("cancel b: %v", err)
	}
	if got := day(2); names(got.GetStaying()) != fmt.Sprint([]string{a.GetName()}) || len(got.GetArrivals()) != 0 {
		t.Errorf("2 Dec after b cancelled: staying %s, arrivals %s; want a alone", names(got.GetStaying()), names(got.GetArrivals()))
	}

	found, err := f.svc.ListReservations(ctx, &staypbv1.ListReservationsRequest{Parent: f.venue,
		Filter: fmt.Sprintf("contact = %q AND spaces:%q AND state != CANCELLED", f.contact, f.rooms)})
	if err != nil || len(found.GetReservations()) != 1 || found.GetReservations()[0].GetName() != a.GetName() {
		t.Errorf("stays by room type = %v, %v; want a", found.GetReservations(), err)
	}
}
