// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

// Command aperio-calendar prints, for every bookable space of every venue,
// the links to subscribe to its rooms' or halls' calendars. Each calendar is
// served by freebusy as a read-only .ics feed holding the bookings and the
// notes aperio puts on them -- guest count, names, emails, phones, address.
// In Google Calendar: Settings > Add calendar > From URL, and paste a link.
//
//	APERIO_DATABASE_URL     postgres DSN or URL of the aperio database
//	FREEBUSY_ENDPOINT       freebusy's gRPC address (read by the SDK)
//	FREEBUSY_PUBLIC_URL     freebusy's public HTTPS address, where Google
//	                        fetches the feeds; a localhost link will not
//	                        subscribe
//	FREEBUSY_FEEDS_SECRET   freebusy's [feeds] secret (FREEBUSY_FEEDS__SECRET
//	                        on the freebusy server)
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"

	"github.com/shokki-engineering/aperio/internal/database/repository/aperio/venue"
	"github.com/shokki-engineering/aperio/internal/database/repository/repox"
	"github.com/shokki-engineering/aperio/internal/engine"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/venue/v1/venuepbv1"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func main() {
	if err := run(context.Background()); err != nil {
		slog.Error("aperio-calendar", "err", err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	dsn, public, secret := os.Getenv("APERIO_DATABASE_URL"), os.Getenv("FREEBUSY_PUBLIC_URL"), os.Getenv("FREEBUSY_FEEDS_SECRET")
	if dsn == "" || public == "" || secret == "" {
		return errors.New("set APERIO_DATABASE_URL, FREEBUSY_PUBLIC_URL and FREEBUSY_FEEDS_SECRET")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		return err
	}
	fb, err := engine.Dial(ctx)
	if err != nil {
		return err
	}
	defer fb.Close()
	repos := venue.New(repox.Conn{Gorm: db})

	venues, err := listAll(func(in repox.ListInput) ([]*venuepbv1.Venue, string, error) {
		return repos.Venues.List(ctx, in)
	})
	if err != nil {
		return err
	}
	for _, v := range venues {
		fmt.Printf("\n%s\n", v.GetDisplayName())
		spaces, err := listAll(func(in repox.ListInput) ([]*venuepbv1.Space, string, error) {
			return repos.Spaces.List(ctx, v.GetName(), in)
		})
		if err != nil {
			return err
		}
		for _, sp := range spaces {
			cals, err := fb.Calendars(ctx, sp.GetResources())
			if err != nil {
				return err
			}
			if len(cals) == 0 {
				continue
			}
			fmt.Printf("  %s\n", sp.GetDisplayName())
			for _, cal := range cals {
				fmt.Printf("    %s\n", engine.FeedURL(public, cal, []byte(secret)))
			}
		}
	}
	fmt.Println()
	return nil
}

// listAll reads every page of a List.
func listAll[T any](list func(repox.ListInput) ([]T, string, error)) ([]T, error) {
	var out []T
	in := repox.ListInput{PageSize: 200}
	for {
		page, next, err := list(in)
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
