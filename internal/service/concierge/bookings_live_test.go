// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package concierge_test

// A live test of a guest finding and cancelling their stay: never by a phone
// alone, by the phone and the reference ConfirmStay gave or the day of
// arrival; cancelling gives the rooms back. Run as test/live says.

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/concierge/v1/conciergepbv1"
	"github.com/the-protobuf-project/runtime-go/agents/mcp"
	"google.golang.org/genproto/googleapis/type/date"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestLiveFindCancelBooking(t *testing.T) {
	r, ctx := newResort(t), context.Background()
	phone := "98765" + r.run[len(r.run)-5:]
	arrive := &date.Date{Year: int32(time.Now().Year()) + 1, Month: 11, Day: 3}
	held, err := r.svc.HoldStay(ctx, &conciergepbv1.HoldStayRequest{Venue: r.venue.GetName(), Space: r.space.GetName(), ArrivalDate: arrive,
		Nights: 1, Adults: 2, GuestDisplayName: "Anil Rao", Phone: phone, RequestId: uuid.NewString()})
	if err != nil {
		t.Fatalf("hold: %v", err)
	}
	booked, err := r.svc.ConfirmStay(ctx, &conciergepbv1.ConfirmStayRequest{Venue: r.venue.GetName(), HoldToken: held.GetHoldToken(), Confirmed: true})
	if err != nil {
		t.Fatalf("confirm: %v", err)
	}
	ref := booked.GetReference()
	if len(ref) != 6 || !strings.Contains(booked.GetSpoken(), strings.Join(strings.Split(ref, ""), " ")) {
		t.Fatalf("reference %q, spoken %q; want 6 characters, spelt out", ref, booked.GetSpoken())
	}

	find := func(req *conciergepbv1.FindBookingRequest) (*conciergepbv1.FindBookingResponse, error) {
		req.Venue = r.venue.GetName()
		return r.svc.FindBooking(ctx, req)
	}
	if _, err := find(&conciergepbv1.FindBookingRequest{Phone: phone}); status.Code(err) != codes.InvalidArgument {
		t.Errorf("find by phone alone = %v; want the reference or arrival date asked for", err)
	}
	for name, req := range map[string]*conciergepbv1.FindBookingRequest{
		"reference":            {Phone: phone, Reference: strings.ToLower(ref)},
		"arrival date":         {Phone: "+91" + phone, ArrivalDate: arrive},
		"spaced phone and ref": {Phone: "98765 " + r.run[len(r.run)-5:], Reference: ref},
	} {
		got, err := find(req)
		if err != nil || len(got.GetBookings()) != 1 || got.GetBookings()[0].GetReservation() != booked.GetReservation() || got.GetBookings()[0].GetState() != "PENDING" {
			t.Errorf("find by %s = %v, %v; want the one pending stay", name, got.GetBookings(), err)
		}
	}
	if got, _ := find(&conciergepbv1.FindBookingRequest{Phone: "9000000001", Reference: ref}); len(got.GetBookings()) != 0 {
		t.Errorf("another phone found the stay: %v", got.GetBookings())
	}

	cancel := &conciergepbv1.CancelBookingRequest{Venue: r.venue.GetName(), Phone: phone, Reference: "ZZZZZZ"}
	if _, err := r.svc.CancelBooking(ctx, cancel); status.Code(err) != codes.InvalidArgument {
		t.Errorf("cancel with a wrong reference = %v; want refused", err)
	}
	cancel.Reference, cancel.Reason = ref, "plans changed"
	_, err = r.svc.CancelBooking(ctx, cancel)
	if q, _, ok := mcp.Asked(err); !ok || !strings.Contains(q, "Cancel your stay from 3 November") || r.fb.Active() != 1 {
		t.Errorf("cancel without consent = %v, %d held; want the stay asked about, and kept", err, r.fb.Active())
	}
	cancel.Confirmed = true
	if _, err := r.svc.CancelBooking(ctx, cancel); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	got, err := find(&conciergepbv1.FindBookingRequest{Phone: phone, Reference: ref})
	if err != nil || got.GetBookings()[0].GetState() != "CANCELLED" || r.fb.Active() != 0 {
		t.Errorf("after cancel: %v, %v, %d rooms held; want CANCELLED and none held", got.GetBookings(), err, r.fb.Active())
	}
}
