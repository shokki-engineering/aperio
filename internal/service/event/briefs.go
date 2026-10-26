// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package event

// briefs.go is a session as SummarizeSessions reports it (summary.go): its
// event, host, space and menu named from one read of each across all the
// sessions, and its plates counted by diet.

import (
	"cmp"
	"context"
	"slices"
	"strings"

	"github.com/shokki-engineering/aperio/internal/database/repository/repox"
	"github.com/shokki-engineering/aperio/internal/service/generated/servicex"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/event/v1/eventpbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/type/v1/typepbv1"
)

// sessionBriefs are sessions as the kitchen reads them, in their order, those
// of cancelled events left out. Events, hosts, spaces and menus are read in
// one call each.
func (h *hooks) sessionBriefs(ctx context.Context, sessions []*eventpbv1.Session) ([]*eventpbv1.SessionBrief, error) {
	if len(sessions) == 0 {
		return nil, nil
	}
	var eventNames, spaceNames, menuNames []string
	for _, s := range sessions {
		eventNames = append(eventNames, servicex.ParentOf(s.GetName()))
		spaceNames = append(spaceNames, s.GetSpace())
		if s.GetMenu() != "" {
			menuNames = append(menuNames, s.GetMenu())
		}
	}
	events, err := h.repos.Events.ListByKeys(ctx, "name", eventNames, repox.ListInput{Filter: "state != CANCELLED"})
	if err != nil {
		return nil, err
	}
	byEvent := map[string]*eventpbv1.Event{}
	var contactNames []string
	for _, e := range events {
		byEvent[e.GetName()] = e
		contactNames = append(contactNames, e.GetContact())
	}
	hosts := map[string]string{}
	contacts, err := h.Contacts.ListByKeys(ctx, "name", contactNames, repox.ListInput{})
	if err != nil {
		return nil, err
	}
	for _, c := range contacts {
		hosts[c.GetName()] = c.GetDisplayName()
	}
	spaces := map[string]string{}
	found, err := h.Spaces.ListByKeys(ctx, "name", spaceNames, repox.ListInput{})
	if err != nil {
		return nil, err
	}
	for _, sp := range found {
		spaces[sp.GetName()] = sp.GetDisplayName()
	}
	menus := map[string]string{}
	if h.menus != nil {
		found, err := h.menus.ListByKeys(ctx, "name", menuNames, repox.ListInput{})
		if err != nil {
			return nil, err
		}
		for _, m := range found {
			menus[m.GetName()] = m.GetDisplayName()
		}
	}
	var out []*eventpbv1.SessionBrief
	for _, s := range sessions {
		e := byEvent[servicex.ParentOf(s.GetName())]
		if e == nil {
			continue
		}
		out = append(out, sessionBrief(s, e, hosts[e.GetContact()], named(spaces, s.GetSpace()), named(menus, s.GetMenu())))
	}
	return out, nil
}

// sessionBrief is s of event e as the kitchen reads it, held in space for
// host, serving menu.
func sessionBrief(s *eventpbv1.Session, e *eventpbv1.Event, host, space, menu string) *eventpbv1.SessionBrief {
	b := &eventpbv1.SessionBrief{
		Session: s.GetName(), Event: e.GetDisplayName(), Title: cmp.Or(s.GetDisplayName(), e.GetDisplayName()),
		Host: host, Space: space, StartTime: s.GetStartTime(), EndTime: s.GetEndTime(),
		Guests: s.GetGuests(), GuaranteedGuests: s.GetGuaranteedGuests(), Menu: menu, State: e.GetState(),
	}
	counts := map[string]int32{}
	for _, dr := range s.GetDietaryRequirements() {
		if dr.GetDiet() != typepbv1.Diet_DIET_UNSPECIFIED {
			counts[said(dr.GetDiet().String(), "DIET_")] += dr.GetGuests()
		}
		for _, a := range dr.GetAllergens() {
			counts["allergic to "+said(a.String(), "ALLERGEN_")] += dr.GetGuests()
		}
	}
	b.Diets = dietCounts(counts)
	for _, sr := range s.GetSpecialRequests() {
		b.Requests = append(b.Requests, sr.GetSummary())
	}
	return b
}

// dietCounts are counts as DietCounts, most first, ties by name.
func dietCounts(counts map[string]int32) []*eventpbv1.DietCount {
	var out []*eventpbv1.DietCount
	for diet, n := range counts {
		out = append(out, &eventpbv1.DietCount{Diet: diet, Guests: n})
	}
	slices.SortFunc(out, func(a, b *eventpbv1.DietCount) int {
		return cmp.Or(cmp.Compare(b.GetGuests(), a.GetGuests()), cmp.Compare(a.GetDiet(), b.GetDiet()))
	})
	return out
}

// said is an enum value name as said: "DIET_NO_ONION_NO_GARLIC" with prefix
// "DIET_" is "no onion no garlic".
func said(value, prefix string) string {
	return strings.ToLower(strings.ReplaceAll(strings.TrimPrefix(value, prefix), "_", " "))
}

// named is the display name of name in names, or name's id when it is not
// there; empty for an empty name.
func named(names map[string]string, name string) string {
	if name == "" {
		return ""
	}
	return cmp.Or(names[name], servicex.IDOf(name))
}
