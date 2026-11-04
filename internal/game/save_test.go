package game

import (
	"context"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/SatoriAI/MT2-Go/internal/account"
	"github.com/SatoriAI/MT2-Go/internal/protocol"
)

// saves stands in for the store's saving: it keeps the last progress and position saved
// of each character. With hold set, the first position saved waits until hold closes,
// telling held as it starts to.
type saves struct {
	Accounts // the rest is never called
	mu       sync.Mutex
	progress map[uint32]account.Character
	position map[uint32][2]int32
	held     chan struct{}
	hold     chan struct{}
}

func newSaves() *saves {
	return &saves{progress: map[uint32]account.Character{}, position: map[uint32][2]int32{}}
}

func (s *saves) SavePosition(_ context.Context, id uint32, x, y int32) error {
	s.mu.Lock()
	hold := s.hold
	s.hold = nil
	s.mu.Unlock()
	if hold != nil {
		s.held <- struct{}{}
		<-hold
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.position[id] = [2]int32{x, y}
	return nil
}

func (s *saves) SaveProgress(_ context.Context, c account.Character) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.progress[c.ID] = c
	return nil
}

// ofHendra is what was last saved of Hendra, character 7.
func (s *saves) ofHendra() (account.Character, [2]int32, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.progress[7]
	return c, s.position[7], ok
}

var quiet = slog.New(slog.DiscardHandler)

// playing puts Hendra, character 7, in the world at the monsters' home, a known map,
// with the session's own copy of her as her connection keeps it.
func playing(t *testing.T) (*Service, *saves, *character, *present) {
	t.Helper()
	store := newSaves()
	s := worldWith(t)
	s.store = store
	c := &character{vid: 1000, Character: account.Character{ID: 7, Name: "Hendra", Level: 3, Exp: 40,
		StatPoints: 2, Stats: account.Stats{Strength: 8}, X: homeX, Y: homeY}}
	conn, _ := watcher(t)
	p := &present{out: conn, char: &c.Character, character: c.ID,
		add: protocol.CharacterAdd{VID: c.vid, X: homeX, Y: homeY}, info: protocol.CharacterInfo{Name: c.Name}}
	s.presence.enter(p)
	return s, store, c, p
}

// A player in the world is saved as they are now: their progress, and where the world
// record has them.
func TestAPlayerInTheWorldIsSavedAsTheyAreNow(t *testing.T) {
	s, store, _, p := playing(t)
	s.presence.move(1000, protocol.Move{X: homeX + 500, Y: homeY})

	s.saveInWorld(p, quiet)

	c, at, ok := store.ofHendra()
	if !ok || c.Level != 3 || c.Exp != 40 || c.StatPoints != 2 || c.Strength != 8 || at != [2]int32{homeX + 500, homeY} {
		t.Fatalf("saved %+v at %v (%v)", c, at, ok)
	}
}

// One who left was saved on leaving, and a save that finds them gone saves nothing.
func TestAPlayerWhoLeftIsNotSavedAgain(t *testing.T) {
	s, store, _, p := playing(t)
	s.presence.leave(1000)

	s.saveInWorld(p, quiet)

	if _, _, ok := store.ofHendra(); ok {
		t.Fatal("a player who left was saved")
	}
}

// A periodic save under way as the player leaves lands first, and the save on leaving,
// with what they earned since, lands last.
func TestTheSaveOnLeavingLandsAfterOneUnderWay(t *testing.T) {
	s, store, c, p := playing(t)
	release := make(chan struct{})
	store.held, store.hold = make(chan struct{}), release
	periodic := make(chan struct{})
	go func() { s.saveInWorld(p, quiet); close(periodic) }()
	<-store.held // the periodic save has its snapshot, at 40, and is writing

	s.presence.mu.Lock()
	c.Exp = 55 // a kill, under the lock as in play
	s.presence.mu.Unlock()
	leaving := make(chan struct{})
	go func() { s.leaveAndSave(c, quiet); close(leaving) }()
	select {
	case <-leaving:
		t.Fatal("the save on leaving did not wait for the one under way")
	case <-time.After(50 * time.Millisecond):
	}
	close(release)
	<-periodic
	<-leaving

	if got, _, _ := store.ofHendra(); got.Exp != 55 {
		t.Fatalf("kept %d exp, want the 55 saved on leaving", got.Exp)
	}
}

// The saver saves every player each time round, until it is stopped.
func TestTheSaverSavesEveryPlayerUntilStopped(t *testing.T) {
	s, store, _, _ := playing(t)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- s.SavePlayers(ctx, time.Millisecond, quiet) }()

	for deadline := time.Now().Add(time.Second); ; time.Sleep(time.Millisecond) {
		if _, _, ok := store.ofHendra(); ok {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("never saved")
		}
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("the saver ended with %v", err)
	}
}
