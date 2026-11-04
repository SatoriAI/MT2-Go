package store

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/SatoriAI/MT2-Go/internal/event"
)

func freshEvents(t *testing.T) Events {
	t.Helper()
	pool, err := Open(context.Background(), freshSchema(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return Events{Pool: pool}
}

var christmasEve = time.Date(2026, 12, 24, 0, 0, 0, 0, time.UTC)

func christmas() event.Event {
	return event.Event{Name: "Boże Narodzenie", ExpPercent: 200, Start: christmasEve, End: christmasEve.AddDate(0, 0, 3)}
}

// An event runs from its first moment up to, not including, its end.
func TestAnEventRunsFromItsStartUntilItsEnd(t *testing.T) {
	ctx := context.Background()
	events := freshEvents(t)
	want := christmas()
	id, err := events.Create(ctx, want)
	if err != nil {
		t.Fatal(err)
	}
	want.ID = id
	for at, running := range map[time.Time]bool{
		christmasEve.Add(-time.Second): false,
		christmasEve:                   true,
		want.End.Add(-time.Second):     true,
		want.End:                       false,
	} {
		got, ok, err := events.Running(ctx, at)
		if err != nil || ok != running || (ok && !sameEvent(got, want)) {
			t.Errorf("at %v: got %+v, %v, %v; want running %v", at, got, ok, err, running)
		}
	}
}

func sameEvent(a, b event.Event) bool {
	return a.ID == b.ID && a.Name == b.Name && a.ExpPercent == b.ExpPercent && a.Start.Equal(b.Start) && a.End.Equal(b.End)
}

// One event runs at a time; one may start the moment another ends.
func TestAnEventOverlappingAnotherIsRefused(t *testing.T) {
	ctx := context.Background()
	events := freshEvents(t)
	if _, err := events.Create(ctx, christmas()); err != nil {
		t.Fatal(err)
	}
	overlapping := christmas()
	overlapping.Start, overlapping.End = christmasEve.AddDate(0, 0, 2), christmasEve.AddDate(0, 0, 8)
	if _, err := events.Create(ctx, overlapping); !errors.Is(err, event.ErrOverlap) {
		t.Fatalf("got %v, want event.ErrOverlap", err)
	}
	newYear := christmas()
	newYear.Name, newYear.Start, newYear.End = "Nowy Rok", christmasEve.AddDate(0, 0, 3), christmasEve.AddDate(0, 0, 9)
	if _, err := events.Create(ctx, newYear); err != nil {
		t.Fatalf("an event starting as another ends refused: %v", err)
	}
	all, err := events.List(ctx)
	if err != nil || len(all) != 2 || all[0].Name != "Boże Narodzenie" || all[1].Name != "Nowy Rok" {
		t.Fatalf("got %+v, %v; want both, by start", all, err)
	}
}

func TestADeletedEventNoLongerRuns(t *testing.T) {
	ctx := context.Background()
	events := freshEvents(t)
	id, err := events.Create(ctx, christmas())
	if err != nil {
		t.Fatal(err)
	}
	if deleted, err := events.Delete(ctx, id); err != nil || !deleted {
		t.Fatalf("got %v, %v; want it deleted", deleted, err)
	}
	if _, ok, err := events.Running(ctx, christmasEve); err != nil || ok {
		t.Fatalf("got running %v, %v; want none", ok, err)
	}
	if deleted, err := events.Delete(ctx, id); err != nil || deleted {
		t.Fatalf("deleting it again: got %v, %v; want nothing deleted", deleted, err)
	}
}
