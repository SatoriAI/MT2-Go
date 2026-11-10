package game

import (
	"context"
	"testing"
	"time"

	"github.com/SatoriAI/MT2-Go/internal/account"
	"github.com/SatoriAI/MT2-Go/internal/handshake"
	"github.com/SatoriAI/MT2-Go/internal/protocol"
)

// slowDelivery is a recipient's connection that notes when a line reached it, then holds
// the sender a moment, so a time taken after delivery cannot equal it.
type slowDelivery struct{ at time.Time }

func (d *slowDelivery) Send(...[]byte) error {
	d.at = time.Now()
	time.Sleep(time.Millisecond)
	return nil
}

// savedAt keeps the time a whisper was saved with.
type savedAt struct{ at time.Time }

func (w *savedAt) Save(_ context.Context, _, _ uint32, _ string, sentAt time.Time) error {
	w.at = sentAt
	return nil
}

// A reply cannot be sent before the line it answers arrives, so a line kept with a time
// from before it arrived reads before its reply, whichever of the two is saved first.
func TestAWhisperIsKeptWithATimeFromBeforeItReachedItsRecipient(t *testing.T) {
	saved := &savedAt{}
	s := NewService(Deps{Whispers: saved, Clock: handshake.NewClock()})
	kira := &slowDelivery{}
	s.record.enter(&present{out: kira, add: protocol.CharacterAdd{VID: 2},
		info: protocol.CharacterInfo{Name: "Kira"}, char: &account.Character{ID: 2, Name: "Kira"}})
	conn, _ := watcher(t)
	hendra := &session{svc: s, conn: conn, playing: &character{Character: account.Character{ID: 1, Name: "Hendra"}}}

	name := make([]byte, 25)
	copy(name, "Kira")
	if err := hendra.onWhisper(append(name, "hi\x00"...)); err != nil {
		t.Fatal(err)
	}

	if kira.at.IsZero() || saved.at.IsZero() {
		t.Fatalf("delivered at %v, saved at %v: want both", kira.at, saved.at)
	}
	if saved.at.After(kira.at) {
		t.Fatalf("saved with %v, after it was delivered at %v", saved.at, kira.at)
	}
}
