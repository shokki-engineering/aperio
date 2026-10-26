// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package event

import (
	"github.com/shokki-engineering/aperio/internal/engine"
	"github.com/shokki-engineering/aperio/internal/service/generated/servicex"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/event/v1/eventpbv1"
)

// window.go is the window a session takes its space for in freebusy.

// sessionWindow is the window a session takes its space for: from setup to
// teardown, or from its start to its end where those are not set.
func sessionWindow(s *eventpbv1.Session) (engine.Window, error) {
	w := engine.Window{Start: s.GetStartTime().AsTime(), End: s.GetEndTime().AsTime()}
	if s.GetSetupTime() != nil {
		w.Start = s.GetSetupTime().AsTime()
	}
	if s.GetTeardownTime() != nil {
		w.End = s.GetTeardownTime().AsTime()
	}
	if !s.GetEndTime().AsTime().After(s.GetStartTime().AsTime()) || !w.End.After(w.Start) {
		return engine.Window{}, servicex.BadField("INVALID_TIMES", "session.end_time",
			"the session ends before it starts, or is torn down before it is set up",
			"Set start_time before end_time, and setup_time and teardown_time around them")
	}
	return w, nil
}
