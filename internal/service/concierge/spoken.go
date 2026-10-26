// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package concierge

// spoken.go words a reply as a person says it, in English for now: amounts
// in rupees as an Indian reads them ("31,360 rupees", "1,25,000 rupees"),
// dates as "12 December", counts with their nouns.

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"google.golang.org/genproto/googleapis/type/date"
	"google.golang.org/genproto/googleapis/type/money"
)

// amount is m to say: whole units in the Indian grouping for rupees, the
// paise only when there are some.
func amount(m *money.Money) string {
	units := groupDigits(m.GetUnits(), m.GetCurrencyCode() == "INR")
	if p := m.GetNanos() / 10_000_000; p != 0 {
		units += fmt.Sprintf(".%02d", p)
	}
	switch m.GetCurrencyCode() {
	case "INR", "":
		return units + " rupees"
	default:
		return units + " " + m.GetCurrencyCode()
	}
}

// groupDigits is n with thousands separators: 1,25,000 in the Indian
// grouping (lakhs, crores), 125,000 otherwise.
func groupDigits(n int64, indian bool) string {
	s := strconv.FormatInt(n, 10)
	neg := strings.HasPrefix(s, "-")
	s = strings.TrimPrefix(s, "-")
	if len(s) > 3 {
		head, tail := s[:len(s)-3], s[len(s)-3:]
		step := 3
		if indian {
			step = 2
		}
		var parts []string
		for len(head) > step {
			parts = append([]string{head[len(head)-step:]}, parts...)
			head = head[:len(head)-step]
		}
		s = strings.Join(append(append([]string{head}, parts...), tail), ",")
	}
	if neg {
		return "-" + s
	}
	return s
}

// day is d to say, "12 December".
func day(d *date.Date) string {
	return fmt.Sprintf("%d %s", d.GetDay(), time.Month(d.GetMonth()))
}

// count is n with its noun, "1 night", "2 nights".
func count(n int32, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	return fmt.Sprintf("%d %ss", n, noun)
}

// minutesLeft is how long until t, to say: "15 minutes".
func minutesLeft(t, now time.Time) string {
	m := int32(t.Sub(now).Round(time.Minute) / time.Minute)
	return count(max(m, 1), "minute")
}
