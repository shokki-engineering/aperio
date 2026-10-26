// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package engine

// notes.go keeps a booking's Note on its resources' freebusy calendars: one
// TRANSPARENT event per calendar, which freebusy's engine ignores (only the
// reservation blocks), named after the reservation so a retry or an edit
// replaces it. freebusy serves the calendars over CalDAV and as read-only
// .ics feeds a calendar app subscribes to (Google Calendar: From URL).

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/shokki-engineering/freebusy-go"
	"github.com/shokki-engineering/freebusy-go/pb/eventpb"
	"github.com/shokki-engineering/freebusy-go/pb/reservationpb"
	"github.com/shokki-engineering/freebusy-go/pb/resourcepb"
	"google.golang.org/genproto/googleapis/type/date"
	"google.golang.org/genproto/googleapis/type/datetime"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
)

// Note writes n on the calendar of each resource reservation holds.
func (c *Client) Note(ctx context.Context, reservation string, n Note) error {
	res, err := c.fb.Reservations.Get(ctx, &reservationpb.GetReservationRequest{Name: reservation})
	if err != nil {
		return fmt.Errorf("engine: get %s: %w", reservation, err)
	}
	cals, err := c.Calendars(ctx, res.GetResources())
	if err != nil {
		return err
	}
	ev := &eventpb.Event{
		Summary:      n.Summary,
		Description:  n.Description,
		Location:     n.Location,
		Start:        calendarTime(n.Window.Start, n.AllDay),
		Ends:         &eventpb.Event_End{End: calendarTime(n.Window.End, n.AllDay)},
		Transparency: eventpb.Transparency_TRANSPARENCY_TRANSPARENT,
	}
	id := reservationID(reservation)
	for _, cal := range cals {
		name := cal + "/events/" + id
		_, err := c.fb.Events.Get(ctx, &eventpb.GetEventRequest{Name: name})
		switch {
		case errors.Is(err, freebusy.ErrNotFound):
			_, err = c.fb.Events.New(ctx, &eventpb.CreateEventRequest{Parent: cal, EventId: id, Event: ev})
		case err == nil:
			upd := proto.Clone(ev).(*eventpb.Event)
			upd.Name = name
			_, err = c.fb.Events.Update(ctx, &eventpb.UpdateEventRequest{Event: upd, UpdateMask: &fieldmaskpb.FieldMask{
				Paths: []string{"summary", "description", "location", "start", "end"},
			}})
		}
		if err != nil {
			return fmt.Errorf("engine: note on %s: %w", cal, err)
		}
	}
	return nil
}

// unnote deletes res's notes from its resources' calendars.
func (c *Client) unnote(ctx context.Context, res *reservationpb.Reservation) error {
	cals, err := c.Calendars(ctx, res.GetResources())
	if err != nil {
		return err
	}
	for _, cal := range cals {
		name := cal + "/events/" + reservationID(res.GetName())
		err := c.fb.Events.Delete(ctx, &eventpb.DeleteEventRequest{Name: name})
		if err != nil && !errors.Is(err, freebusy.ErrNotFound) {
			return fmt.Errorf("engine: delete note %s: %w", name, err)
		}
	}
	return nil
}

// Calendars are the calendars of resources, those that have one.
func (c *Client) Calendars(ctx context.Context, resources []string) ([]string, error) {
	var out []string
	for _, r := range resources {
		res, err := c.fb.Resources.Get(ctx, &resourcepb.GetResourceRequest{Name: r})
		if err != nil {
			return nil, fmt.Errorf("engine: get %s: %w", r, err)
		}
		if res.GetCalendar() != "" {
			out = append(out, res.GetCalendar())
		}
	}
	return out, nil
}

// calendarTime is t as an iCalendar DATE (allDay) or as a DATE-TIME in its
// own zone, so the calendar shows the venue's wall clock.
func calendarTime(t time.Time, allDay bool) *eventpb.CalendarTime {
	if allDay {
		return &eventpb.CalendarTime{Value: &eventpb.CalendarTime_Date{Date: &date.Date{
			Year: int32(t.Year()), Month: int32(t.Month()), Day: int32(t.Day()),
		}}}
	}
	return &eventpb.CalendarTime{Value: &eventpb.CalendarTime_DateTime{DateTime: &datetime.DateTime{
		Year: int32(t.Year()), Month: int32(t.Month()), Day: int32(t.Day()),
		Hours: int32(t.Hour()), Minutes: int32(t.Minute()),
		TimeOffset: &datetime.DateTime_TimeZone{TimeZone: &datetime.TimeZone{Id: t.Location().String()}},
	}}}
}
