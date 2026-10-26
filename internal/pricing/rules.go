// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package pricing

// rules.go is when a season covers a date, which price rule prices an item
// on a date, and what it makes the price. Prices are never stored per date:
// they follow the rules as the seasons turn.

import (
	"math/big"
	"slices"
	"strings"
	"time"

	"github.com/shokki-engineering/aperio/protobuf/generated/go/sales/v1/salespbv1"
	"google.golang.org/genproto/googleapis/type/date"
	"google.golang.org/genproto/googleapis/type/dayofweek"
)

// Covers reports whether season s covers day d. A season with no dates
// covers every day its weekdays allow; one with dates, the days within any
// of them -- on month and day alone when it recurs yearly, so "1 Nov to
// 28 Feb" wraps the new year and holds every year.
func Covers(s *salespbv1.Season, d *date.Date) bool {
	if wd := s.GetWeekdays(); len(wd) > 0 {
		t := time.Date(int(d.GetYear()), time.Month(d.GetMonth()), int(d.GetDay()), 0, 0, 0, 0, time.UTC)
		iso := dayofweek.DayOfWeek(t.Weekday()) // google.type.DayOfWeek: MONDAY 1 .. SUNDAY 7
		if iso == 0 {
			iso = dayofweek.DayOfWeek_SUNDAY
		}
		if !slices.Contains(wd, iso) {
			return false
		}
	}
	if len(s.GetDates()) == 0 {
		return true
	}
	for _, r := range s.GetDates() {
		from, to := r.GetStartDate(), r.GetEndDate()
		if s.GetRecursYearly() {
			md := monthDay(d)
			lo, hi := monthDay(from), monthDay(to)
			if (lo <= hi && md >= lo && md <= hi) || (lo > hi && (md >= lo || md <= hi)) {
				return true
			}
		} else if ymd(d) >= ymd(from) && ymd(d) <= ymd(to) {
			return true
		}
	}
	return false
}

// Match is what a rule is matched against.
type Match struct {
	Target  salespbv1.PriceTarget
	Item    string                       // the item's resource name
	Date    *date.Date                   // the service date
	Guests  int32                        // the guests it is for
	Seasons map[string]*salespbv1.Season // the venue's seasons, by name
}

// Pick is the rule that prices m: of the active rules for its target and
// item, whose season covers its date and whose min_guests it meets, the one
// of the highest priority; at equal priority one naming the item beats one
// for every item, and then the first by name, so the choice never wavers.
// nil when none matches: the base price stands.
func Pick(rules []*salespbv1.PriceRule, m Match) *salespbv1.PriceRule {
	var best *salespbv1.PriceRule
	for _, r := range rules {
		if !r.GetActive() || r.GetTarget() != m.Target || r.GetMinGuests() > m.Guests {
			continue
		}
		if len(r.GetItems()) > 0 && !slices.Contains(r.GetItems(), m.Item) {
			continue
		}
		if s := r.GetSeason(); s != "" {
			season, ok := m.Seasons[s]
			if !ok || !Covers(season, m.Date) {
				continue
			}
		}
		if best == nil || better(r, best) {
			best = r
		}
	}
	return best
}

// better reports whether a wins over b.
func better(a, b *salespbv1.PriceRule) bool {
	if a.GetPriority() != b.GetPriority() {
		return a.GetPriority() > b.GetPriority()
	}
	if sa, sb := len(a.GetItems()) > 0, len(b.GetItems()) > 0; sa != sb {
		return sa
	}
	return strings.Compare(a.GetName(), b.GetName()) < 0
}

// Apply is base with rule r applied, unrounded: its fixed price, base plus
// its percentage, or base plus its amount. base may be nil -- an item with no
// price of its own, a hall hired only at a rule's fixed price -- and then
// only a fixed price can price it. ok is false when nothing does.
func Apply(r *salespbv1.PriceRule, base *big.Rat) (price *big.Rat, ok bool, err error) {
	switch {
	case r.GetFixedPrice() != nil:
		return Rat(r.GetFixedPrice()), true, nil
	case base == nil:
		return nil, false, nil
	case r.GetPercent() != nil:
		p, err := Decimal(r.GetPercent())
		if err != nil {
			return nil, false, err
		}
		return Sum(base, Percent(base, p)), true, nil
	case r.GetAmount() != nil:
		return Sum(base, Rat(r.GetAmount())), true, nil
	}
	return base, true, nil
}

// Scales reports whether r scales a price, as a percentage does: the one
// kind of rule that also applies to a package's extra-guest price.
func Scales(r *salespbv1.PriceRule) bool { return r.GetPercent() != nil }

func monthDay(d *date.Date) int32 { return d.GetMonth()*100 + d.GetDay() }
func ymd(d *date.Date) int32      { return d.GetYear()*10000 + monthDay(d) }
