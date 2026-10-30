// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package freebusysync_test

// flaky_live_test.go is an event repository whose writes of an event fail,
// in a file of its own: the repository package is named event, as the
// service package the other tests import is.

import (
	"context"

	"github.com/shokki-engineering/aperio/internal/database/repository/aperio/event"
	"github.com/shokki-engineering/aperio/internal/service/generated/aperio/event/v1/eventpbv1svc"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/event/v1/eventpbv1"
)

// flakyEvents lets the first skip writes of an event through and fails the
// next fail (a negative fail, every one after): with err, or -- err nil --
// with a real conflict, another writer changing the event first.
type flakyEvents struct {
	event.EventRepository
	skip, fail int
	err        error
}

// trip is the error the write of in fails with, or nil to let it through.
func (r *flakyEvents) trip(ctx context.Context, in *eventpbv1.Event) error {
	if r.skip > 0 {
		r.skip--
		return nil
	}
	if r.fail == 0 {
		return nil
	}
	r.fail--
	if r.err != nil {
		return r.err
	}
	// Another writer gets in first: in's etag goes stale, and the write
	// below conflicts in the database, not here.
	now, err := r.EventRepository.Get(ctx, in.GetName())
	if err == nil {
		_, err = r.EventRepository.UpdateOutputOnly(ctx, now, []string{"freebusy_holds"})
	}
	return err
}

// UpdateOutputOnly fails as flakyEvents says, or writes.
func (r *flakyEvents) UpdateOutputOnly(ctx context.Context, in *eventpbv1.Event, paths []string) (*eventpbv1.Event, error) {
	if err := r.trip(ctx, in); err != nil {
		return nil, err
	}
	return r.EventRepository.UpdateOutputOnly(ctx, in, paths)
}

// UpdateWithOutputOnly fails as flakyEvents says, or writes.
func (r *flakyEvents) UpdateWithOutputOnly(ctx context.Context, in *eventpbv1.Event, paths, output []string) (*eventpbv1.Event, error) {
	if err := r.trip(ctx, in); err != nil {
		return nil, err
	}
	return r.EventRepository.UpdateWithOutputOnly(ctx, in, paths, output)
}

// flaky is repos with their events' writes failing as flakyEvents says.
func flaky(repos eventpbv1svc.EventServiceRepositories, skip, fail int, err error) eventpbv1svc.EventServiceRepositories {
	repos.Events = &flakyEvents{EventRepository: repos.Events, skip: skip, fail: fail, err: err}
	return repos
}
