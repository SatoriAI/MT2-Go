package game

import (
	"testing"
	"time"

	"github.com/SatoriAI/MT2-Go/internal/mob"
	"github.com/SatoriAI/MT2-Go/internal/protocol"
	"github.com/SatoriAI/MT2-Go/internal/world"
)

// The orc lord comes back 20 to 30 minutes after it dies, at a point the moment of its
// death picks, so two deaths need not wait alike.
func TestTheOrcLordComesBackTwentyToThirtyMinutesAfterItDies(t *testing.T) {
	lord, _ := mob.Lookup(691)
	waits := map[time.Duration]bool{}
	for _, ns := range []int{0, 1, 123456789, 599999999999, 600000000000, 987654321012} {
		m := &monster{proto: &lord, diedAt: noon.Add(time.Duration(ns))}
		wait := m.respawnAfter()
		if wait < 20*time.Minute || wait >= 30*time.Minute {
			t.Errorf("died at +%d ns, back after %v", ns, wait)
		}
		waits[wait] = true
	}
	if len(waits) < 4 {
		t.Errorf("waits %v, want them spread", waits)
	}
}

// When the orc lord appears, everyone on its map is told it has woken, even out of view.
func TestTheOrcLordsWakingIsToldToItsMap(t *testing.T) {
	s := worldWith(t, world.Spawn{VNum: 691, Count: 1, X: homeX, Y: homeY})
	far, _ := aPlayer(t, s, 1000, 50, "Hendra", 30000)
	drain(t, far)
	lord := s.monsters[0]

	s.place(lord, noon)

	notice := protocol.EncodeChat(protocol.ChatNotice, 0, 0, textToClient("Wódz Orków przebudził się!"))
	if got := packets(t, drain(t, far)); !holds(got, notice) {
		t.Fatalf("the far player got %x, want the notice", got)
	}
}
