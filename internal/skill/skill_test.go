package skill

import (
	"slices"
	"testing"

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
