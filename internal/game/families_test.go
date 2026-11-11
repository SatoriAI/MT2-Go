package game

import (
	"fmt"
	"math/rand/v2"
	"testing"
	"time"

	"github.com/SatoriAI/MT2-Go/internal/handshake"
	"github.com/SatoriAI/MT2-Go/internal/world"
)

// wolfPack is a family's pack of wolves, three times as likely as alpha wolves, at home.
var wolfPack = world.Spawn{Kinds: []world.Kind{{VNum: 102, Weight: 3}, {VNum: 103, Weight: 1}}, MinCount: 3, Count: 5,
	X: homeX, Y: homeY, Spread: 800, Respawn: 30 * time.Second}

// seededWorld is a world of the spawns whose randomness starts from seed.
func seededWorld(t *testing.T, seed uint64, spawns ...world.Spawn) *Service {
	t.Helper()
	s := NewService(Deps{Clock: handshake.NewClock(), Spawns: spawns})
	s.rand = rand.New(rand.NewPCG(seed, seed)) //nolint:gosec // a fixed seed, so each run rolls the same
	if err := s.spawn(noon); err != nil {
		t.Fatal(err)
	}
	return s
}

// standing are the kinds of the pack's monsters in the world, in its order.
func standing(k *pack) []uint16 {
	var kinds []uint16
	for _, m := range k.members {
		if !m.gone {
			kinds = append(kinds, m.proto.VNum)
		}
	}
	return kinds
}

// A family's pack holds 3 to 5 of its kinds, each as likely as its weight: over many
// worlds every size comes, and wolves about three times as often as alpha wolves.
func TestAFamilysPackHoldsThreeToFiveOfItsKinds(t *testing.T) {
	sizes, kinds := map[int]int{}, map[uint16]int{}
	for seed := range uint64(300) {
		s := seededWorld(t, seed, wolfPack)
		got := standing(s.monsters[0].pack)
		sizes[len(got)]++
		for _, k := range got {
			kinds[k]++
		}
	}
	if len(sizes) != 3 || sizes[3] == 0 || sizes[4] == 0 || sizes[5] == 0 {
		t.Errorf("sizes %v, want 3, 4 and 5", sizes)
	}
	if len(kinds) != 2 || kinds[102] < 2*kinds[103] || kinds[102] > 4*kinds[103] {
		t.Errorf("kinds %v, want wolves about three times the alpha wolves", kinds)
	}
}

// A family's pack, once all of it has died, comes back its time later rolled afresh: its
// size and kinds change from one coming back to the next.
func TestAFamilysPackComesBackRolledAfresh(t *testing.T) {
	s := seededWorld(t, 1, wolfPack)
	_, hendra := aPlayer(t, s, 1000, 7, "Hendra", 0)
	k := s.monsters[0].pack
	seen := map[string]bool{}
	at := noon
	for range 20 {
		got := standing(k)
		if len(got) < 3 || len(got) > 5 {
			t.Fatalf("back with %v", got)
		}
		seen[fmt.Sprint(got)] = true
		for _, m := range k.members {
			if !m.gone {
				s.record.held(func(l locked) { l.kill(hendra, m.p, at) })
			}
		}
		s.tick(at)
		s.tick(at.Add(corpseTime))
		at = at.Add(corpseTime + 30*time.Second)
		s.tick(at)
	}
	if len(seen) < 2 {
		t.Fatalf("came back the same each time: %v", seen)
	}
}

// A family takes only monsters that wait to be hit: a cursed kind in one is refused.
func TestAFamilyRefusesAKindThatAttacksOnSight(t *testing.T) {
	cursed := wolfPack
	cursed.Kinds = []world.Kind{{VNum: 131, Weight: 1}}
	if err := NewService(Deps{Clock: handshake.NewClock(), Spawns: []world.Spawn{cursed}}).spawn(noon); err == nil {
		t.Fatal("a cursed wolf in a family's pack")
	}
}

// A family's pack comes back its time after the last of it died, not after the first or
// those a roll left out: the wolves killed a minute apart, 30 s after the last death, and
// not a moment before.
func TestAFamilysPackComesBackItsTimeAfterItsLastDied(t *testing.T) {
	s := seededWorld(t, 1, wolfPack)
	_, hendra := aPlayer(t, s, 1000, 7, "Hendra", 0)
	k := s.monsters[0].pack
	last := noon
	for _, m := range k.members {
		if !m.gone {
			last = last.Add(time.Minute)
			s.record.held(func(l locked) { l.kill(hendra, m.p, last) })
			s.tick(last)
		}
	}
	s.tick(last.Add(corpseTime))
	s.tick(last.Add(30*time.Second - time.Millisecond))
	if got := standing(k); len(got) != 0 {
		t.Fatalf("back too soon with %v", got)
	}
	s.tick(last.Add(30 * time.Second))
	if got := standing(k); len(got) < 3 {
		t.Fatalf("not back: %v", got)
	}
}

// The world's own spawns all go in: every family's kinds known and none attacking on sight.
func TestTheWorldsOwnSpawnsAllSpawn(t *testing.T) {
	if err := NewService(Deps{Clock: handshake.NewClock(), Spawns: world.Spawns()}).spawn(noon); err != nil {
		t.Fatal(err)
	}
}
