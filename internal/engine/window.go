// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package engine

// window.go is the window a stay takes, computed in one place: a hold and the
// reservation made from it must ask freebusy for the same window, or the
// reservation would not find the hold.

import (
	"time"

	"google.golang.org/genproto/googleapis/type/date"
)

// Stay is the window a stay from arrive to depart takes in loc, the venue's
// time zone: from the arrival day's midnight to the departure day's, so the
// nights between are the guest's and a stay ending on a day never overlaps
// one starting on it.
func Stay(arrive, depart *date.Date, loc *time.Location) Window {
	return Window{Start: Midnight(arrive, loc), End: Midnight(depart, loc)}
}

// Midnight is the start of day d in loc.
func Midnight(d *date.Date, loc *time.Location) time.Time {
	return time.Date(int(d.GetYear()), time.Month(d.GetMonth()), int(d.GetDay()), 0, 0, 0, 0, loc)
}

// AddDays is d moved n days.
func AddDays(d *date.Date, n int) *date.Date {
	t := time.Date(int(d.GetYear()), time.Month(d.GetMonth()), int(d.GetDay())+n, 0, 0, 0, 0, time.UTC)
	return &date.Date{Year: int32(t.Year()), Month: int32(t.Month()), Day: int32(t.Day())}
}
