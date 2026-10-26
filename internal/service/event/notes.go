// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package event

// notes.go puts who a session is for on its hall's calendar (internal/calendar
// writes the note), and refreshes it when the session is edited.

import (
	"context"
	"time"

	"github.com/shokki-engineering/aperio/internal/calendar"
	"github.com/shokki-engineering/aperio/internal/service/generated/servicex"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/event/v1/eventpbv1"
)

// AfterUpdateSession refreshes the session's note.
func (h *hooks) AfterUpdateSession(ctx context.Context, m *eventpbv1.Session) error {
	h.noteSession(ctx, m)
	return nil
}

// noteSession notes s on its hall's calendar, in the venue's time zone.
func (h *hooks) noteSession(ctx context.Context, s *eventpbv1.Session) {
	if s.GetFreebusyReservation() == "" {
		return
	}
	eventName := servicex.ParentOf(s.GetName())
	venueName := servicex.ParentOf(eventName)
	ev, err := h.repos.Events.Get(ctx, eventName)
	if err != nil {
		h.NoteFailed(ctx, s.GetName(), err)
		return
	}
	w, err := sessionWindow(s)
	if err == nil {
		var loc *time.Location
		if loc, err = h.Zone(ctx, venueName); err == nil {
			w.Start, w.End = w.Start.In(loc), w.End.In(loc)
		}
	}
	if err != nil {
		h.NoteFailed(ctx, s.GetName(), err)
		return
	}
	n := calendar.Session(ev, s, h.Contact(ctx, ev.GetContact()), h.Where(ctx, venueName, s.GetSpace()), w)
	if err := h.Engine.Note(ctx, s.GetFreebusyReservation(), n); err != nil {
		h.NoteFailed(ctx, s.GetName(), err)
	}
}
