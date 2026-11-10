package upgrade

import (
	"testing"

	"github.com/SatoriAI/MT2-Go/internal/item"
)

// A sword is upgraded from +0 to +9 by the veteran's table: yang and chance for every
// grade, and from the step to +5 one wolf's claw more each, a failure there costing a
// grade.
func TestASwordIsUpgradedByTheVeteransTable(t *testing.T) {
	want := []Step{
		{Into: 11, Yang: 200, Chance: 100},
		{Into: 12, Yang: 400, Chance: 90},
		{Into: 13, Yang: 800, Chance: 80},
		{Into: 14, Yang: 1500, Chance: 70},
		{Into: 15, Yang: 3000, Chance: 60, Material: WolfClaw, Count: 1, Downgrade: true},
		{Into: 16, Yang: 5000, Chance: 50, Material: WolfClaw, Count: 2, Downgrade: true},
		{Into: 17, Yang: 8000, Chance: 40, Material: WolfClaw, Count: 3, Downgrade: true},
		{Into: 18, Yang: 12000, Chance: 30, Material: WolfClaw, Count: 4, Downgrade: true},
		{Into: 19, Yang: 20000, Chance: 20, Material: WolfClaw, Count: 5, Downgrade: true},
	}
	for g, w := range want {
		p, _ := item.Lookup(10 + uint32(g))
		if got, ok := Of(p); !ok || got != w {
			t.Errorf("from +%d: %+v (%v), want %+v", g, got, ok, w)
		}
	}
}

// Armour, a helmet, shoes and the shield take a boar's tooth where a weapon takes a claw.
func TestArmourTakesABoarsTooth(t *testing.T) {
	for _, vnum := range []uint32{11604, 12484, 15004, 13004} {
		p, _ := item.Lookup(vnum)
		if got, _ := Of(p); got.Material != WildBoarTooth || got.Count != 1 || got.Into != vnum+1 {
			t.Errorf("%d: %+v", vnum, got)
		}
	}
}

// A failure leaves the item as it was below +4, and a grade lower from the step to +5 on;
// it is never destroyed.
func TestAFailureCostsAGradeOnlyFromTheStepToFive(t *testing.T) {
	for from, want := range map[uint32]uint32{11: 11, 14: 13, 18: 17} {
		p, _ := item.Lookup(from)
		if s, _ := Of(p); s.Failed() != want {
			t.Errorf("from %d: %d, want %d", from, s.Failed(), want)
		}
	}
}

// A +9, a potion and a material are not upgraded.
func TestOnlyGearBelowNineIsUpgraded(t *testing.T) {
	for _, vnum := range []uint32{19, 11809, 27001, WolfClaw} {
		p, _ := item.Lookup(vnum)
		if _, ok := Of(p); ok {
			t.Errorf("kind %d upgraded", vnum)
		}
	}
}

// Gear past the first costs more to upgrade the higher its level: the table's yang once
// below 20, twice from 20, four times from 25 and eight times from 34; the chance and the
// materials stay the table's.
func TestHigherGearCostsMoreToUpgrade(t *testing.T) {
	for vnum, want := range map[uint32]uint32{30: 200, 50: 400, 60: 800, 5030: 800, 80: 1600, 11240: 1600, 11220: 200} {
		p, _ := item.Lookup(vnum)
		if s, ok := Of(p); !ok || s.Yang != want || s.Chance != 100 {
			t.Errorf("%d, level %d: %+v", vnum, p.Level, s)
		}
	}
	top, _ := item.Lookup(88)
	if s, _ := Of(top); s.Yang != 20000*8 || s.Material != WolfClaw || s.Count != 5 {
		t.Errorf("+8 to +9 at 36: %+v", s)
	}
}
