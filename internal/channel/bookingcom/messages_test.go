// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package bookingcom

// messages_test.go checks the messages aperio writes against golden files in
// testdata: the OTA_HotelResNotifRS for success and for errors, and the rate
// and availability notifications. go test -update rewrites them.

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/shokki-engineering/aperio/internal/channel"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/genproto/googleapis/type/date"
	"google.golang.org/genproto/googleapis/type/money"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var update = flag.Bool("update", false, "rewrite the golden files")

// fixed is an Adapter with a fixed clock and EchoToken, for golden files.
func fixed() *Adapter {
	return &Adapter{
		Now:   func() time.Time { return time.Date(2027, 2, 1, 10, 15, 0, 0, time.UTC) },
		Token: func() string { return "echo-out-0001" },
	}
}

// golden compares got with testdata/name.
func golden(t *testing.T, name string, got []byte, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join("testdata", name)
	if *update {
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("%s differs:\n%s", name, got)
	}
}

func d(m, day int32) *date.Date { return &date.Date{Year: 2027, Month: m, Day: day} }

func TestAnswerSuccess(t *testing.T) {
	in, err := parse(fixture(t, "new.xml"))
	if err != nil {
		t.Fatal(err)
	}
	out, err := fixed().AnswerReservations(in, []channel.Result{{
		Notification: in.Notifications[0], Reservation: "venues/v1/reservations/bookingcom-4105349872",
	}}, nil)
	golden(t, "rs_success.xml", out, err)
}

func TestAnswerErrors(t *testing.T) {
	in, err := parse(fixture(t, "new.xml"))
	if err != nil {
		t.Fatal(err)
	}
	n := in.Notifications[0]
	full, err := status.New(codes.FailedPrecondition, "1 of the space are not free for the dates").
		WithDetails(&errdetails.ErrorInfo{Reason: "NOT_AVAILABLE", Domain: "aperio"})
	if err != nil {
		t.Fatal(err)
	}
	out, err := fixed().AnswerReservations(in, []channel.Result{
		{Notification: n, Err: fmt.Errorf("%w: %q at hotel %q", channel.ErrUnknownRoomType, "SV-XXX", "7777001")},
		{Notification: n, Err: fmt.Errorf("%w: %q", channel.ErrUnknownHotel, "7777999")},
		{Notification: n, Err: full.Err()},
	}, nil)
	golden(t, "rs_errors.xml", out, err)

	out, err = fixed().AnswerReservations(nil, nil, errors.New("bookingcom: not an OTA_HotelResNotifRQ: EOF"))
	golden(t, "rs_unreadable.xml", out, err)
}

func TestRatesMessage(t *testing.T) {
	inr := func(u int64) *money.Money { return &money.Money{CurrencyCode: "INR", Units: u} }
	out, err := fixed().RatesMessage("7777001", []channel.Rate{
		{TypeCode: "SV-DLX", RatePlanCode: "BAR", Start: d(3, 1), End: d(3, 4), Amount: inr(8000)},
		{TypeCode: "SV-DLX", RatePlanCode: "BAR", Start: d(3, 5), End: d(3, 6), Amount: &money.Money{CurrencyCode: "INR", Units: 9600, Nanos: 500000000}},
	})
	golden(t, "rate_amount_notif.xml", out, err)
}

func TestAvailabilityMessage(t *testing.T) {
	out, err := fixed().AvailabilityMessage("7777001", []channel.Availability{
		{TypeCode: "SV-DLX", Start: d(3, 1), End: d(3, 9), Free: 2},
		{TypeCode: "SV-DLX", Start: d(3, 10), End: d(3, 11), Free: 1},
		{TypeCode: "GV-VIL", Start: d(3, 1), End: d(3, 11), Free: 0},
	})
	golden(t, "avail_notif.xml", out, err)
}
