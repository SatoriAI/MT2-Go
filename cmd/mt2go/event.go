package main

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/SatoriAI/MT2-Go/internal/config"
	"github.com/SatoriAI/MT2-Go/internal/event"
	"github.com/SatoriAI/MT2-Go/internal/game"
	"github.com/SatoriAI/MT2-Go/internal/store"
)

// eventTime is how an event's start and end are written: to the minute, in event.Zone,
// which its notices name too.
const eventTime = "2006-01-02T15:04"

func runEvent(ctx context.Context, cfg config.Config, args []string) error {
	var do func(store.Events) error
	switch {
	case len(args) == 6 && args[0] == "create":
		do = func(events store.Events) error { return createEvent(ctx, events, args[1:]) }
	case len(args) == 1 && args[0] == "list":
		do = func(events store.Events) error { return listEvents(ctx, events) }
	case len(args) == 2 && args[0] == "delete":
		do = func(events store.Events) error { return deleteEvent(ctx, events, args[1]) }
	default:
		return errors.New(usage)
	}
	pool, err := store.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	return do(store.Events{Pool: pool})
}

func createEvent(ctx context.Context, events store.Events, args []string) error {
	if !game.ClientCanShow(args[0]) {
		return errors.New("the name has characters the game client cannot show")
	}
	exp, err := event.ParseMultiplier(args[1])
	if err != nil {
		return err
	}
	yang, err := event.ParseMultiplier(args[2])
	if err != nil {
		return err
	}
	start, err := time.ParseInLocation(eventTime, args[3], event.Zone)
	if err != nil {
		return fmt.Errorf("the start must be written as %s: %w", eventTime, err)
	}
	end, err := time.ParseInLocation(eventTime, args[4], event.Zone)
	if err != nil {
		return fmt.Errorf("the end must be written as %s: %w", eventTime, err)
	}
	e := event.Event{Name: args[0], ExpPercent: exp, YangPercent: yang, Start: start, End: end}
	if err := e.Check(); err != nil {
		return err
	}
	id, err := events.Create(ctx, e)
	if err != nil {
		return err
	}
	fmt.Printf("created event %d: %s\n", id, describe(e))
	return nil
}

func listEvents(ctx context.Context, events store.Events) error {
	all, err := events.List(ctx)
	if err != nil {
		return err
	}
	for _, e := range all {
		fmt.Printf("%d: %s\n", e.ID, describe(e))
	}
	return nil
}

func deleteEvent(ctx context.Context, events store.Events, arg string) error {
	id, err := strconv.ParseInt(arg, 10, 64)
	if err != nil {
		return fmt.Errorf("the id %q is not a number", arg)
	}
	deleted, err := events.Delete(ctx, id)
	if err != nil {
		return err
	}
	if !deleted {
		return fmt.Errorf("there is no event %d", id)
	}
	fmt.Printf("deleted event %d\n", id)
	return nil
}

func describe(e event.Event) string {
	return fmt.Sprintf("%q, experience ×%s, yang ×%s, %s to %s", e.Name, event.Multiplier(e.ExpPercent), event.Multiplier(e.YangPercent),
		e.Start.In(event.Zone).Format(eventTime), e.End.In(event.Zone).Format(eventTime))
}
