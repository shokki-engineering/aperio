// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package event

// retry.go is how the event service changes an event it did not just read
// from a client: read it, change it, and write it under the etag it read,
// reading again when another writer got in between. Every write of an
// event's own fields -- its state (moveEvent), its hall holds (HoldHalls,
// replaceHold, restoreHold), what freebusy did to them (RelinkEvent) --
// goes through editEvent, so a session replacing a hall, a webhook
// flagging one and an accept recording one can land together without one
// failing, or writing back what the other changed.

import (
	"context"
	"errors"

	"github.com/shokki-engineering/aperio/internal/database/repository/repox"
	"github.com/shokki-engineering/aperio/internal/service/generated/servicex"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/event/v1/eventpbv1"
)

// eventRetries is how many times editEvent reads and writes an event before
// it gives a conflict back.
const eventRetries = 5

// editEvent reads event name and has edit change it, then writes the fields
// edit names -- paths through the update mask, output the ones only the
// server writes -- guarded by the etag it read. A write that conflicts runs
// edit again on the event as it now stands, up to eventRetries times, so
// edit decides afresh from what it is given each time and must be safe to
// repeat. An error from edit is given back as it is, never retried: a
// client's own etag that no longer matches is its conflict, not ours.
// Naming no fields writes nothing and returns the event as read.
func (h *hooks) editEvent(ctx context.Context, name string,
	edit func(e *eventpbv1.Event) (paths, output []string, err error)) (*eventpbv1.Event, error) {
	for try := 1; ; try++ {
		e, err := h.repos.Events.Get(ctx, name)
		if err != nil {
			return nil, servicex.NotFoundAs(err, eventType, name)
		}
		paths, output, err := edit(e)
		if err != nil {
			return e, err
		}
		if len(paths) == 0 && len(output) == 0 {
			return e, nil
		}
		var out *eventpbv1.Event
		if len(paths) == 0 {
			out, err = h.repos.Events.UpdateOutputOnly(ctx, e, output)
		} else {
			out, err = h.repos.Events.UpdateWithOutputOnly(ctx, e, paths, output)
		}
		if err == nil || !errors.Is(err, repox.ErrConflict) || try == eventRetries {
			return out, err
		}
	}
}
