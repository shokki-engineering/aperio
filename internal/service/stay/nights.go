// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package stay

// nights.go is the stay read model's writer: stay.v1.StayNight rows, one per
// night of every stay that is not cancelled or a no-show. The generated
// projection runtime calls it with the reservations each write changed and
// stores what it returns in the same transaction (repox.ProjectionWriter);
// RebuildNights calls it for every stay, to fill the table where it is empty.
//
// This file imports the generated stay repository package, which shares this
// package's name; nothing else here does.

import (
	"context"
	"strings"
	"time"

	"github.com/shokki-engineering/aperio/internal/database/gorm/filterx"
	"github.com/shokki-engineering/aperio/internal/database/repository/aperio/stay"
	"github.com/shokki-engineering/aperio/internal/database/repository/repox"
	"github.com/shokki-engineering/aperio/internal/search"
	"github.com/shokki-engineering/aperio/internal/service/generated/servicex"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/stay/v1/staypbv1"
	"google.golang.org/genproto/googleapis/type/date"
)

// NightsProjector is the read model's projector, for repox.NewProjections.
func NightsProjector() repox.Projector {
	return stay.StayNights.Projector(stay.StayNightWriterFunc(projectNights))
}

// projectNights replaces the nights of every reservation changes touch. On
// GORM it reads each through read, which sees the write; over GraphQL read
// does not, and the change's record is the reservation as written.
func projectNights(ctx context.Context, read repox.Conn, changes []repox.Change) (stay.StayNightRows, error) {
	var out stay.StayNightRows
	repos := stay.New(read)
	for _, c := range changes {
		if c.Type != reservationType {
			continue
		}
		out.Prune = append(out.Prune, []filterx.Condition{{Field: "reservation", Op: filterx.OpEq, Value: c.Name}})
		if c.Op == repox.ChangeDelete || c.Op == repox.ChangePurge {
			continue
		}
		r, ok := c.Record.(*staypbv1.Reservation)
		if read.Gorm != nil || !ok || r.GetArrivalDate() == nil {
			got, err := repos.Reservations.Get(ctx, c.Name)
			if err != nil {
				return out, err
			}
			r = got
		}
		out.Upsert = append(out.Upsert, nightsOf(r)...)
	}
	return out, nil
}

// nightsOf are r's nights: none when it is cancelled or a no-show.
func nightsOf(r *staypbv1.Reservation) []*staypbv1.StayNight {
	switch r.GetState() {
	case staypbv1.Reservation_STATE_CANCELLED, staypbv1.Reservation_STATE_NO_SHOW, staypbv1.Reservation_STATE_UNSPECIFIED:
		return nil
	}
	var rooms int32
	for _, line := range r.GetRooms() {
		rooms += line.GetRooms()
	}
	first, last := dayOf(r.GetArrivalDate()), dayOf(r.GetDepartureDate()).AddDate(0, 0, -1)
	var out []*staypbv1.StayNight
	for d := first; !d.After(last); d = d.AddDate(0, 0, 1) {
		out = append(out, &staypbv1.StayNight{
			Reservation: r.GetName(), Venue: servicex.ParentOf(r.GetName()), Contact: r.GetContact(),
			Day:    &date.Date{Year: int32(d.Year()), Month: int32(d.Month()), Day: int32(d.Day())},
			Spaces: r.GetSpaces(), Rooms: rooms, Guests: r.GetAdults() + r.GetChildren(),
			FirstNight: d.Equal(first), LastNight: d.Equal(last),
			State: strings.TrimPrefix(r.GetState().String(), "STATE_"),
		})
	}
	return out
}

// dayOf is d as a time at UTC midnight.
func dayOf(d *date.Date) time.Time {
	return time.Date(int(d.GetYear()), time.Month(d.GetMonth()), int(d.GetDay()), 0, 0, 0, 0, time.UTC)
}

// NightsEmpty reports whether the read model has no rows: a fresh deployment,
// or one upgraded from a version without it.
func NightsEmpty(ctx context.Context, pg repox.Conn) (bool, error) {
	n, err := stay.StayNights.Count(ctx, pg, repox.SearchInput{})
	return n == 0, err
}

// RebuildNights projects every stay again, through conn, into the table pg
// reaches. Writes keep the read model current after; it runs while the
// server serves, and a day read meanwhile misses the stays not yet
// projected.
func RebuildNights(ctx context.Context, conn, pg repox.Conn) error {
	repos := stay.New(conn)
	batches := func(yield func([]repox.Change) bool) {
		in := repox.ListInput{PageSize: 200}
		for {
			page, next, err := repos.Reservations.List(ctx, "venues/-", in)
			if err != nil || len(page) == 0 {
				return
			}
			changes := make([]repox.Change, len(page))
			for i, r := range page {
				changes[i] = repox.Change{Type: reservationType, Name: r.GetName(), Op: repox.ChangeUpdate, Record: r}
			}
			if !yield(changes) || next == "" {
				return
			}
			in.PageToken = next
		}
	}
	_, err := stay.StayNights.Rebuild(ctx, pg, stay.StayNightWriterFunc(projectNights), batches, repox.RebuildOptions{})
	return err
}

// PostgresNights reads the read model from its table, through conn
// (database.Connection.SearchConn).
func PostgresNights(conn repox.Conn) search.Reader[*staypbv1.StayNight] {
	return search.Postgres(stay.StayNights, conn)
}

// nightsOn are the read model's nights at venueName on day ("2026-12-12"):
// with last, only the stays' last nights.
func (h *hooks) nightsOn(ctx context.Context, venueName, day string, last bool) ([]*staypbv1.StayNight, error) {
	in := repox.SearchInput{PageSize: 1000, Where: []filterx.Condition{
		{Field: "venue", Op: filterx.OpEq, Value: venueName},
		{Field: "day", Op: filterx.OpEq, Value: day},
	}}
	if last {
		in.Where = append(in.Where, filterx.Condition{Field: "last_night", Op: filterx.OpEq, Value: "true"})
	}
	return search.All(ctx, h.nights, in)
}
