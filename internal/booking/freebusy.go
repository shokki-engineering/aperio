// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package booking

// freebusy.go is what the booking services share in staying in step with
// freebusy: the keys of the holds and reservations aperio takes besides a
// stay's rooms and a session's hall -- an accepted event's halls, a lead's
// dates -- the window of a span of days, and the flags left on a booking
// freebusy changed under it.

import (
	"fmt"
	"strings"
	"time"

	"github.com/shokki-engineering/aperio/internal/engine"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/type/v1/typepbv1"
)

// SpaceKey is the freebusy booking key of the hold an accepted event keeps on
// space for its dates, until a session there replaces it.
func SpaceKey(event, space string) string {
	return event + "#space/" + space
}

// LeadKey is the freebusy hold key of a lead's dates, taken at t: a lapsed
// hold cannot be taken again under its key, so each take has its own.
func LeadKey(lead string, t time.Time) string {
	return fmt.Sprintf("%s#hold/%d", lead, t.UnixNano())
}

// Days is the window of the whole days of d in loc, [start_date, end_date]:
// from the first day's midnight to the midnight after the last.
func Days(d *typepbv1.DateRange, loc *time.Location) engine.Window {
	return engine.Window{
		Start: engine.Midnight(d.GetStartDate(), loc),
		End:   engine.Midnight(engine.AddDays(d.GetEndDate(), 1), loc),
	}
}

// Mark is how a flag about reservation starts, so a booking is flagged about
// it once however often it is found.
func Mark(reservation string) string { return "[freebusy " + reservation + "]" }

// Flag is notes with "mark text" added on a line of its own, and whether it
// was added: notes that already carry mark are returned as they are.
func Flag(notes, mark, text string) (string, bool) {
	if strings.Contains(notes, mark) {
		return notes, false
	}
	line := mark + " " + text
	if notes == "" {
		return line, true
	}
	return notes + "\n" + line, true
}
