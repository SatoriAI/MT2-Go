package game

import (
	"context"
	"testing"
	"time"

	"github.com/SatoriAI/MT2-Go/internal/account"
	"github.com/SatoriAI/MT2-Go/internal/item"
	"github.com/SatoriAI/MT2-Go/internal/protocol"
	"github.com/SatoriAI/MT2-Go/internal/world"
)

// dogsAt are wild dogs, one at each distance east of the monsters' home.
func dogsAt(xs ...int32) []world.Spawn {
	spawns := make([]world.Spawn, 0, len(xs))
	for _, x := range xs {
		spawns = append(spawns, world.Spawn{VNum: 101, X: homeX + x, Y: homeY, Count: 1})
	}
	return spawns
}

// caped is Hendra, a level-50 sura no bite can kill, at the monsters' home among spawns,
// with 5 capes at cell 0, at noon; the world loop has not yet ticked.
func caped(t *testing.T, spawns ...world.Spawn) (*session, *keptItems, *present) {
	t.Helper()
	svc := worldWith(t, spawns...)
	svc.now = func() time.Time { return noon }
	kept := &keptItems{moved: map[int64]uint16{}, worn: map[int64]item.Wear{}, count: map[int64]uint8{}}
	svc.items = kept
	conn, client := watcher(t)
	p := &present{out: conn, char: &account.Character{ID: 7, Name: "Hendra", Class: account.Sura, Level: 50, HP: 100_000},
		info: protocol.CharacterInfo{VID: 1000, Name: "Hendra"},
		add:  protocol.CharacterAdd{VID: 1000, X: homeX, Y: homeY, Type: protocol.EntityPlayer, MoveSpeed: 150}}
	svc.record.enter(p)
	drain(t, client)
	s := &session{svc: svc, conn: conn, phase: protocol.PhaseGame, playing: playingAs(p),
		inventory: item.Inventory{{ID: 1, VNum: item.BraveryCape, Count: 5, Cell: 0}}}
	return s, kept, p
}

func wearCape(t *testing.T, s *session) {
	t.Helper()
	if err := s.onItemUse(context.Background(), doubleClicked(0)); err != nil {
		t.Fatal(err)
	}
}

// chasing is how many of the monsters fight p.
func chasing(s *session, p *present) int {
	n := 0
	for _, m := range s.svc.monsters {
		if m.chase != nil && m.chase.target == p {
			n++
		}
	}
	return n
}

// A cape sets every idle monster within 2500 on her, however far below her level, and
// uses one of the stack up; one further off stays.
func TestACapeCallsTheMonstersAroundOntoThePlayer(t *testing.T) {
	s, kept, p := caped(t, dogsAt(500, 2400, 2600)...)

	wearCape(t, s)
	s.svc.tick(noon)

	near, edge, far := s.svc.monsters[0], s.svc.monsters[1], s.svc.monsters[2]
	if near.chase == nil || near.chase.target != p || edge.chase == nil || far.chase != nil {
		t.Fatalf("chasing: at 500 %v, at 2400 %v, at 2600 %v", near.chase != nil, edge.chase != nil, far.chase != nil)
	}
	if kept.count[1] != 4 || s.inventory[0].Count != 4 {
		t.Fatalf("kept %d, carrying %d, want one cape used", kept.count[1], s.inventory[0].Count)
	}
}

// A monster pulled from afar gives up no sooner than ten seconds after it could have
// walked to her, so it never turns back on the way.
func TestAPulledMonsterCountsItsFightFromWhenItArrives(t *testing.T) {
	s, _, _ := caped(t, dogsAt(2400)...)

	wearCape(t, s)
	s.svc.tick(noon)

	dog := s.svc.monsters[0]
	if want := noon.Add(walkTime(dog, 2400, 0)); !dog.chase.lastExchange.Equal(want) {
		t.Fatalf("fight counted from %v, want %v", dog.chase.lastExchange, want)
	}
}

// A stone never moves, so a cape leaves it be.
func TestACapeLeavesAStone(t *testing.T) {
	s, _, _ := caped(t, world.Spawn{VNum: 8001, Count: 1, Spots: []world.Spot{{X: homeX + 300, Y: homeY}}})

	wearCape(t, s)
	s.svc.tick(noon)

	if s.svc.monsters[0].chase != nil {
		t.Fatal("the stone was set on her")
	}
}

// A monster fighting another player stays on them, its fight with them as it was.
func TestACapeTakesNoMonsterFromAnotherPlayer(t *testing.T) {
	s, _, p := caped(t, dogsAt(500)...)
	_, kira := aPlayer(t, s.svc, 1001, 8, "Kira", 2000) // out of its bite
	dog := s.svc.monsters[0]
	s.svc.chaseOn(dog, kira, noon)

	wearCape(t, s)
	s.svc.tick(noon)

	if dog.chase.target != kira || !dog.chase.lastExchange.Equal(noon) || chasing(s, p) != 0 {
		t.Fatal("the dog left Kira, or its fight with her was renewed")
	}
}

// At most the nearest 50 come.
func TestACapeCallsAtMostTheNearestFifty(t *testing.T) {
	xs := make([]int32, 0, 52)
	for i := range int32(52) {
		xs = append(xs, 100+i*40)
	}
	s, _, p := caped(t, dogsAt(xs...)...)

	wearCape(t, s)
	s.svc.tick(noon)

	if n := chasing(s, p); n != 50 || s.svc.monsters[50].chase != nil || s.svc.monsters[51].chase != nil {
		t.Fatalf("%d came, the two furthest among them: %v", n, s.svc.monsters[51].chase != nil)
	}
}

// A cape is used up even with no monster near, as the original's were.
func TestACapeIsUsedUpWithNothingNear(t *testing.T) {
	s, kept, _ := caped(t, dogsAt(5000)...)

	wearCape(t, s)

	if kept.count[1] != 4 {
		t.Fatalf("kept %d capes, want one used", kept.count[1])
	}
}

// A second cape within two seconds of the last does nothing and is kept; one after them
// is worn.
func TestACapeTooSoonAfterTheLastIsKept(t *testing.T) {
	s, kept, _ := caped(t)
	wearCape(t, s)

	s.svc.now = func() time.Time { return noon.Add(2*time.Second - time.Millisecond) }
	wearCape(t, s)
	if kept.count[1] != 4 {
		t.Fatalf("kept %d capes after a second too soon, want 4", kept.count[1])
	}
	s.svc.now = func() time.Time { return noon.Add(2 * time.Second) }
	wearCape(t, s)
	if kept.count[1] != 3 {
		t.Fatalf("kept %d capes once the wait was over, want 3", kept.count[1])
	}
}

// The dead wear no cape.
func TestTheDeadWearNoCape(t *testing.T) {
	s, kept, p := caped(t, dogsAt(500)...)
	p.char.HP = 0

	wearCape(t, s)
	s.svc.tick(noon)

	if len(kept.count) != 0 || chasing(s, p) != 0 {
		t.Fatalf("kept %v, %d chasing", kept.count, chasing(s, p))
	}
}
