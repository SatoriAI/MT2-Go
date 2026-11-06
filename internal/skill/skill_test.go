package skill

import (
	"slices"
	"testing"
	"time"

	"github.com/SatoriAI/MT2-Go/internal/account"
)

// Every skill is of a class and one of that class's own skill groups.
func TestEverySkillIsOfAGroupOfItsClass(t *testing.T) {
	for s, p := range protos {
		if groups := p.Class.SkillGroups(); !slices.Contains(groups[:], p.Group) {
			t.Errorf("%s: %+v", s, p)
		}
	}
}

// A character earns a point at level 5 and one more each level, less the levels learnt;
// below 5 it has none, and it never has fewer than none.
func TestSkillPointsAreEarnedFromLevelFiveLessThoseSpent(t *testing.T) {
	cases := []struct {
		level  uint8
		learnt map[account.Skill]uint8
		want   uint32
	}{
		{4, nil, 0},
		{5, nil, 1},
		{10, nil, 6},
		{10, map[account.Skill]uint8{DragonSwirl: 2, EnchantedBlade: 1}, 3},
		{6, map[account.Skill]uint8{DragonSwirl: 5}, 0},
	}
	for _, c := range cases {
		if got := Points(c.level, c.learnt); got != c.want {
			t.Errorf("level %d with %v: %d points, want %d", c.level, c.learnt, got, c.want)
		}
	}
}

// A skill's power is the client's table: 0.05 at the first level, 0.40 at 19, 0.50 at 20.
func TestASkillsPowerIsTheClientsTable(t *testing.T) {
	for level, want := range map[uint8]uint32{0: 0, 1: 5, 2: 6, 3: 8, 10: 22, 19: 40, 20: 50} {
		if got := Power(level); got != want {
			t.Errorf("level %d: %d, want %d", level, got, want)
		}
	}
}

// What a skill costs is the client's formula at its power, rounded down: Dragon Swirl
// 50 + 150k, Enchanted Blade 20 + 240k, so the client's own check agrees.
func TestASkillCostsTheClientsMana(t *testing.T) {
	swirl, _ := Lookup(DragonSwirl)
	blade, _ := Lookup(EnchantedBlade)
	for level := uint8(1); level <= MaxLevel; level++ {
		k := float64(Power(level)) / 100
		if got, want := swirl.Mana.At(Power(level)), uint32(50+150*k); got != want {
			t.Errorf("Dragon Swirl at %d: %d, want %d", level, got, want)
		}
		if got, want := blade.Mana.At(Power(level)), uint32(20+240*k); got != want {
			t.Errorf("Enchanted Blade at %d: %d, want %d", level, got, want)
		}
	}
	if swirl.Cooldown != 15*time.Second || swirl.Reach != 500 || blade.Lasts != 3*time.Minute {
		t.Fatalf("swirl %+v, blade %+v", swirl, blade)
	}
}

// The blade adds 7 + (5·INT + 13)·k, as the client's tooltip shows it, rounded down: a
// sura of INT 5 adds 8 with it at 1 (8.9), and 26 at 20 (26.0).
func TestTheBladeAddsWhatItsTooltipSays(t *testing.T) {
	for level, want := range map[uint8]uint16{1: 8, 20: 26} {
		if got := Bonus(5, level); got != want {
			t.Errorf("at %d: %d, want %d", level, got, want)
		}
	}
}
