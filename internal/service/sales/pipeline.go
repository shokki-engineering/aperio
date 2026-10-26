// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package sales

// pipeline.go is SummarizePipeline: a venue's sales as the team reads them
// on a Monday -- open leads by stage, what was won and lost lately, the
// follow-ups due, the leads nobody has called, and the offers about to
// lapse. Each is one filtered read under its index: the counts read the
// stage alone, the lists a page of at most briefLimit, and the guests they
// name are read once, together.

import (
	"context"
	"fmt"
	"time"

	"github.com/shokki-engineering/aperio/internal/database/gorm/filterx"
	"github.com/shokki-engineering/aperio/internal/database/repository/aperio/guest"
	"github.com/shokki-engineering/aperio/internal/database/repository/repox"
	"github.com/shokki-engineering/aperio/internal/service/generated/servicex"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/guest/v1/guestpbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/sales/v1/salespbv1"
)

// briefLimit is how many leads or quotes each list of the summary holds.
const briefLimit = 20

// countPage is the page the stage counts read: the most a read returns.
const countPage = 1000

// ReadContactsFrom names the guests SummarizePipeline lists from contacts:
// the guest service's repository, built beside this one.
func (s *Server) ReadContactsFrom(contacts guest.ContactRepository) { s.h.contacts = contacts }

// SummarizePipeline summarizes req.Venue's pipeline over the next and last
// req.Days days (7 when unset).
func (s *Server) SummarizePipeline(ctx context.Context, req *salespbv1.SummarizePipelineRequest) (*salespbv1.SummarizePipelineResponse, error) {
	var out *salespbv1.SummarizePipelineResponse
	err := s.Options().Run(ctx, "SalesService", "SummarizePipeline", func(ctx context.Context) error {
		var err error
		out, err = s.h.summarizePipeline(ctx, req)
		return err
	})
	return out, err
}

func (h *hooks) summarizePipeline(ctx context.Context, req *salespbv1.SummarizePipelineRequest) (*salespbv1.SummarizePipelineResponse, error) {
	venueName := req.GetVenue()
	if _, err := h.venues.Get(ctx, venueName); err != nil {
		return nil, servicex.NotFoundAs(err, venueType, venueName)
	}
	days := req.GetDays()
	if days == 0 {
		days = 7
	}
	now := h.now().UTC()
	span := time.Duration(days) * 24 * time.Hour
	at := func(t time.Time) string { return fmt.Sprintf("%q", t.Format(time.RFC3339Nano)) }
	here := fmt.Sprintf("venue = %q", venueName)
	open := here + " AND stage != WON AND stage != LOST"

	stages, err := h.countStages(ctx, open)
	if err != nil {
		return nil, err
	}
	closed, err := h.countStages(ctx, here+" AND close_time >= "+at(now.Add(-span)))
	if err != nil {
		return nil, err
	}
	out := &salespbv1.SummarizePipelineResponse{Won: closed[salespbv1.Stage_STAGE_WON], Lost: closed[salespbv1.Stage_STAGE_LOST]}
	for st := salespbv1.Stage_STAGE_NEW; st <= salespbv1.Stage_STAGE_NEGOTIATION; st++ {
		if n := stages[st]; n > 0 {
			out.Stages = append(out.Stages, &salespbv1.StageCount{Stage: st, Leads: n})
		}
	}

	due, _, err := h.repos.Leads.List(ctx, repox.ListInput{PageSize: briefLimit, OrderBy: "next_follow_up_time",
		Filter: open + " AND next_follow_up_time < " + at(now.Add(span))})
	if err != nil {
		return nil, err
	}
	fresh, _, err := h.repos.Leads.List(ctx, repox.ListInput{PageSize: briefLimit, OrderBy: "create_time",
		Filter: here + " AND stage = NEW"})
	if err != nil {
		return nil, err
	}
	expiring, _, err := h.repos.Quotes.List(ctx, venueName, repox.ListInput{PageSize: briefLimit, OrderBy: "expire_time",
		Filter: "state = SENT AND expire_time > " + at(now) + " AND expire_time < " + at(now.Add(span))})
	if err != nil {
		return nil, err
	}

	var names []string
	for _, l := range append(append([]*salespbv1.Lead{}, due...), fresh...) {
		names = append(names, l.GetContact())
	}
	for _, q := range expiring {
		names = append(names, q.GetContact())
	}
	guests, err := h.guests(ctx, names)
	if err != nil {
		return nil, err
	}
	for _, l := range due {
		out.FollowUps = append(out.FollowUps, leadBrief(l, guests[l.GetContact()]))
	}
	for _, l := range fresh {
		out.Uncontacted = append(out.Uncontacted, leadBrief(l, guests[l.GetContact()]))
	}
	for _, q := range expiring {
		b := &salespbv1.QuoteBrief{Quote: q.GetName(), Title: q.GetDisplayName(), Contact: q.GetContact(), Total: q.GetTotal(), ExpireTime: q.GetExpireTime()}
		if c := guests[q.GetContact()]; c != nil {
			b.Contact = c.GetDisplayName()
		}
		out.Expiring = append(out.Expiring, b)
	}
	return out, nil
}

// countStages counts the leads filter matches by stage, reading only their
// stage, a page of countPage at a time.
func (h *hooks) countStages(ctx context.Context, filter string) (map[salespbv1.Stage]int32, error) {
	counts := map[salespbv1.Stage]int32{}
	in := repox.ListInput{PageSize: countPage, Filter: filter, ReadMask: []string{"stage"}}
	for {
		page, next, err := h.repos.Leads.List(ctx, in)
		if err != nil {
			return nil, err
		}
		for _, l := range page {
			counts[l.GetStage()]++
		}
		if next == "" {
			return counts, nil
		}
		in.PageToken = next
	}
}

// guests are the contacts names name, by name, read in one call; none when
// no contacts repository is set.
func (h *hooks) guests(ctx context.Context, names []string) (map[string]*guestpbv1.Contact, error) {
	out := map[string]*guestpbv1.Contact{}
	if h.contacts == nil || len(names) == 0 {
		return out, nil
	}
	cs, _, err := h.contacts.List(ctx, repox.ListInput{PageSize: int32(len(names)), Where: []filterx.Condition{filterx.In("name", names...)}})
	if err != nil {
		return nil, err
	}
	for _, c := range cs {
		out[c.GetName()] = c
	}
	return out, nil
}

// leadBrief is l as the sales team reads it, its guest c (nil when unread).
func leadBrief(l *salespbv1.Lead, c *guestpbv1.Contact) *salespbv1.LeadBrief {
	b := &salespbv1.LeadBrief{Lead: l.GetName(), Title: l.GetDisplayName(), Contact: l.GetContact(), Stage: l.GetStage(),
		NextFollowUpTime: l.GetNextFollowUpTime(), CreateTime: l.GetCreateTime()}
	if c != nil {
		b.Contact = c.GetDisplayName()
		if len(c.GetPhoneNumbers()) > 0 {
			b.Phone = c.GetPhoneNumbers()[0]
		}
	}
	return b
}
