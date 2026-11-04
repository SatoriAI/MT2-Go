package mob

import (
	"testing"

	"github.com/SatoriAI/MT2-Go/internal/world"
)

// The client stands a mob's animation still at speed 0, so every kind the world spawns
// needs a proto with real speeds.
func TestEveryKindTheWorldSpawnsIsKnownAndMoves(t *testing.T) {
	for _, s := range world.Spawns() {
		p, ok := Lookup(s.VNum)
		if !ok {
			t.Errorf("%d is spawned but has no proto", s.VNum)
			continue
		}
		if p.VNum != s.VNum || p.MoveSpeed == 0 || p.AttackSpeed == 0 || p.Name == "" {
			t.Errorf("%d: %+v", s.VNum, p)
		}
	}
}

func TestOnlyMonstersWander(t *testing.T) {
	if p, _ := Lookup(101); !p.Wanders() {
		t.Error("a wild dog stands still")
	}
	if p, _ := Lookup(9001); p.Wanders() {
		t.Error("the weapon dealer wanders off")
	}
}

// Every monster the world spawns can be fought: it has hit points and a level; NPCs
// cannot be.
func TestEveryMonsterCanBeFoughtAndNoNPC(t *testing.T) {
	for _, s := range world.Spawns() {
		p, _ := Lookup(s.VNum)
		if p.Fightable() != (p.Kind == Monster) || (p.Fightable() && (p.MaxHP == 0 || p.Level == 0)) {
			t.Errorf("%d: %+v", s.VNum, p)
		}
	}
}
