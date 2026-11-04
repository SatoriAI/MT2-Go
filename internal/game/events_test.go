package game

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"testing"
	"time"

	"github.com/SatoriAI/MT2-Go/internal/account"
	"github.com/SatoriAI/MT2-Go/internal/event"
	"github.com/SatoriAI/MT2-Go/internal/protocol"
)

var christmas = event.Event{ID: 1, Name: "Boże Narodzenie", ExpPercent: 200,
	Start: time.Date(2026, 12, 24, 0, 0, 0, 0, event.Zone), End: time.Date(2026, 12, 27, 0, 0, 0, 0, event.Zone)}

var newYear = event.Event{ID: 2, Name: "Nowy Rok", ExpPercent: 150,
	Start: time.Date(2026, 12, 31, 18, 0, 0, 0, event.Zone), End: time.Date(2027, 1, 1, 12, 0, 0, 0, event.Zone)}

// The notices as players read them, in the client's code page from the server's vid 0.
var (
	christmasRuns = protocol.EncodeChat(protocol.ChatNotice, 0, 0,
		textToClient("Wydarzenie „Boże Narodzenie” trwa do 27.12 00:00: doświadczenie ×2."))
	christmasEnded = protocol.EncodeChat(protocol.ChatNotice, 0, 0,
		textToClient("Wydarzenie „Boże Narodzenie” dobiegło końca."))
	newYearRuns = protocol.EncodeChat(protocol.ChatNotice, 0, 0,
		textToClient("Wydarzenie „Nowy Rok” trwa do 01.01 12:00: doświadczenie ×1,5."))
)

// namedPlayer enters a player at x, on no map and far from any monster; players entered
// further apart than the view range are shown nothing.
func (s *Service) namedPlayer(t *testing.T, vid uint32, name string, x int32) net.Conn {
	t.Helper()
	conn, client := watcher(t)
	s.presence.enter(&present{out: conn, add: protocol.CharacterAdd{VID: vid, X: x}, info: protocol.CharacterInfo{Name: name}})
	return client
}

func expectBytes(t *testing.T, c net.Conn, want ...[]byte) {
	t.Helper()
	for _, w := range want {
		if got := read(t, c, len(w)); string(got) != string(w) {
			t.Fatalf("got %x, want %x", got, w)
		}
	}
}

func TestAnEventStartingAndEndingIsAnnouncedToEveryPlayer(t *testing.T) {
	s := worldWith(t)
	hendra, botka := s.namedPlayer(t, 1000, "Hendra", 0), s.namedPlayer(t, 1001, "Botka", 2*viewRange)

	s.presence.switchEvent(christmas, true)
	expectBytes(t, hendra, christmasRuns)
	expectBytes(t, botka, christmasRuns)

	s.presence.switchEvent(christmas, true)
	expectNothing(t, hendra)

	s.presence.switchEvent(event.Event{}, false)
	expectBytes(t, hendra, christmasEnded)
	expectBytes(t, botka, christmasEnded)

	s.presence.switchEvent(event.Event{}, false)
	expectNothing(t, hendra)
}

// One event following straight on another ends the first and starts the second.
func TestAnEventFollowingAnotherEndsItFirst(t *testing.T) {
	s := worldWith(t)
	hendra := s.namedPlayer(t, 1000, "Hendra", 0)
	s.presence.switchEvent(christmas, true)
	expectBytes(t, hendra, christmasRuns)

	s.presence.switchEvent(newYear, true)

	expectBytes(t, hendra, christmasEnded, newYearRuns)
}

func TestAPlayerEnteringWhileAnEventRunsIsTold(t *testing.T) {
	s := worldWith(t)
	s.presence.switchEvent(christmas, true)

	hendra := s.namedPlayer(t, 1000, "Hendra", 0)

	expectBytes(t, hendra, christmasRuns)
	expectNothing(t, hendra)
}

// A dog's 15, doubled.
func TestAnEventMultipliesWhatAKillGives(t *testing.T) {
	s, dog, client := aDog(t, homeX+100)
	s.presence.switchEvent(christmas, true)
	expectBytes(t, client, christmasRuns)
	c := &account.Character{HP: 1000, SP: 1000, Level: 1, Class: account.Warrior, Stats: account.Stats{Strength: 90}}
	s.presence.all[1000].char = c

	s.presence.attack(1000, dog.p.add.VID, 0, someRand, noon)
	read(t, client, damageSize+deadSize)

	if point, amount, value := pointChange(t, client, 1000); point != protocol.PointExp || amount != 30 || value != 30 {
		t.Fatalf("got point %d, +%d to %d; want exp +30 to 30", point, amount, value)
	}
}

// scriptedEvents answers each look with the next of its answers, waiting for one.
type scriptedEvents struct {
	answers chan running
}

type running struct {
	event.Event
	on  bool
	err error
}

func (s scriptedEvents) Running(ctx context.Context, _ time.Time) (event.Event, bool, error) {
	select {
	case r := <-s.answers:
		return r.Event, r.on, r.err
	case <-ctx.Done():
		return event.Event{}, false, ctx.Err()
	}
}

// The watch switches what the store says runs, and a failed look changes nothing until
// the next.
func TestTheWatchSwitchesEventsAsTheStoreSaysAndRidesOutAFailedLook(t *testing.T) {
	s := worldWith(t)
	hendra := s.namedPlayer(t, 1000, "Hendra", 0)
	events := scriptedEvents{answers: make(chan running)}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- s.WatchEvents(ctx, events, time.Millisecond, slog.New(slog.DiscardHandler)) }()

	events.answers <- running{Event: christmas, on: true}
	expectBytes(t, hendra, christmasRuns)
	events.answers <- running{err: errors.New("the database is away")}
	events.answers <- running{Event: christmas, on: true}
	expectNothing(t, hendra)
	events.answers <- running{}
	expectBytes(t, hendra, christmasEnded)

	cancel()
	if err := <-done; err != nil {
		t.Fatalf("the watch ended with %v", err)
	}
}

// hangingOnce never answers its first look, as a database that stopped answering, and
// says Christmas runs at every look after.
type hangingOnce struct {
	looked chan struct{}
}

func (h hangingOnce) Running(ctx context.Context, _ time.Time) (event.Event, bool, error) {
	select {
	case h.looked <- struct{}{}:
		<-ctx.Done()
		return event.Event{}, false, ctx.Err()
	default:
		return christmas, true, nil
	}
}

// A look that hangs is given up at the next, and the watch carries on.
func TestTheWatchGivesUpALookThatHangs(t *testing.T) {
	s := worldWith(t)
	hendra := s.namedPlayer(t, 1000, "Hendra", 0)
	events := hangingOnce{looked: make(chan struct{}, 1)}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = s.WatchEvents(ctx, events, 10*time.Millisecond, slog.New(slog.DiscardHandler)) }()

	expectBytes(t, hendra, christmasRuns)
}
