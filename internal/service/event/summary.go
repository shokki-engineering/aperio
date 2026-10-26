// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package event

// summary.go is SummarizeSessions: a venue's sessions, day by day, as the
// kitchen and the banquet team read them -- who, where, when, how many, what
// menu, and the plates by diet -- with each day's covers. It reads the days'
// sessions across every event under the (start_time, end_time) index, then
// their events, hosts, spaces and menus in one read each (briefs.go).

import (
	"context"
	"fmt"
	"time"

	"github.com/shokki-engineering/aperio/internal/database/repository/aperio/catalog"
	"github.com/shokki-engineering/aperio/internal/database/repository/repox"
	"github.com/shokki-engineering/aperio/internal/service/generated/servicex"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/event/v1/eventpbv1"
	"google.golang.org/genproto/googleapis/type/date"
)

// maxSummaryDays is the most days one SummarizeSessions call covers.
const maxSummaryDays = 14

// ReadMenusFrom has SummarizeSessions name each session's menu from menus;
// without it, a menu is reported by its id.
func (s *Server) ReadMenusFrom(menus catalog.MenuRepository) { s.h.menus = menus }

// SummarizeSessions summarizes req.Days days -- one, when unset -- of
// req.Venue's sessions from req.StartDate, today at the venue when unset.
func (s *Server) SummarizeSessions(ctx context.Context, req *eventpbv1.SummarizeSessionsRequest) (*eventpbv1.SummarizeSessionsResponse, error) {
	var out *eventpbv1.SummarizeSessionsResponse
	err := s.Options().Run(ctx, "EventService", "SummarizeSessions", func(ctx context.Context) error {
		var err error
		out, err = s.h.summarizeSessions(ctx, req)
		return err
	})
	return out, err
}

func (h *hooks) summarizeSessions(ctx context.Context, req *eventpbv1.SummarizeSessionsRequest) (*eventpbv1.SummarizeSessionsResponse, error) {
	days := int(req.GetDays())
	if days < 0 || days > maxSummaryDays {
		return nil, servicex.BadField("INVALID_DAYS", "days", "days must be from 0 to 14",
			"Ask for at most 14 days; 0 means one")
	}
	days = max(days, 1)
	zone, err := h.Zone(ctx, req.GetVenue())
	if err != nil {
		return nil, err
	}
	first := h.Now().In(zone)
	if d := req.GetStartDate(); d != nil {
		first = time.Date(int(d.GetYear()), time.Month(d.GetMonth()), int(d.GetDay()), 0, 0, 0, 0, zone)
	}
	first = time.Date(first.Year(), first.Month(), first.Day(), 0, 0, 0, 0, zone)
	sessions, err := h.sessionsBetween(ctx, req.GetVenue(), first, first.AddDate(0, 0, days))
	if err != nil {
		return nil, err
	}
	briefs, err := h.sessionBriefs(ctx, sessions)
	if err != nil {
		return nil, err
	}
	out := &eventpbv1.SummarizeSessionsResponse{}
	byDay := map[string]*eventpbv1.SessionDay{}
	for i := range days {
		d := first.AddDate(0, 0, i)
		day := &eventpbv1.SessionDay{Date: &date.Date{Year: int32(d.Year()), Month: int32(d.Month()), Day: int32(d.Day())}}
		out.Days = append(out.Days, day)
		byDay[d.Format(time.DateOnly)] = day
	}
	for _, b := range briefs {
		day := byDay[b.GetStartTime().AsTime().In(zone).Format(time.DateOnly)]
		if day == nil {
			continue
		}
		day.Sessions = append(day.Sessions, b)
		day.Covers += b.GetGuests()
	}
	for _, day := range out.Days {
		counts := map[string]int32{}
		for _, b := range day.GetSessions() {
			for _, c := range b.GetDiets() {
				counts[c.GetDiet()] += c.GetGuests()
			}
		}
		day.Diets = dietCounts(counts)
	}
	return out, nil
}

// sessionsBetween reads venueName's sessions starting in [from, to), across
// every event (AIP-159), by start time, page by page.
func (h *hooks) sessionsBetween(ctx context.Context, venueName string, from, to time.Time) ([]*eventpbv1.Session, error) {
	in := repox.ListInput{
		PageSize: 1000,
		OrderBy:  "start_time",
		Filter: fmt.Sprintf("venue = %q AND start_time >= %q AND start_time < %q",
			venueName, from.Format(time.RFC3339), to.Format(time.RFC3339)),
	}
	var out []*eventpbv1.Session
	for {
		page, next, err := h.repos.Sessions.List(ctx, venueName+"/events/-", in)
		if err != nil {
			return nil, err
		}
		out = append(out, page...)
		if next == "" {
			return out, nil
		}
		in.PageToken = next
	}
}
