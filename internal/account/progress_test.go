package account

import "testing"

func TestEachLevelNeedsTheOriginalsExperience(t *testing.T) {
	cases := map[uint8]uint32{1: 300, 2: 800, 10: 33000, 20: 472000, 30: 2122000, 90: 743730000, 98: 2029047116}
	for level, want := range cases {
		if got := NextExp(level); got != want {
			t.Errorf("level %d needs %d, want %d", level, got, want)
		}
	}
	if NextExp(MaxLevel) != 0 || NextExp(0) != 0 {
		t.Error("the top level and level 0 need nothing")
	}
}

// A monster far below the player gives almost nothing, one level with them all of it, one
// far above more; gaps past 15 count as 15.
func TestTheLevelGapScalesTheExperience(t *testing.T) {
	cases := []struct {
		monster, player uint8
		want            uint32
	}{
		{1, 30, 1}, {15, 30, 1}, {16, 30, 5}, {20, 30, 50}, {29, 30, 100}, {30, 30, 100},
		{31, 30, 105}, {44, 30, 170}, {45, 30, 180}, {90, 30, 180}, {255, 1, 180}, {1, 255, 1},
	}
	for _, c := range cases {
		if got := GapPercent(c.monster, c.player); got != c.want {
			t.Errorf("a level-%d monster for a level-%d player: %d%%, want %d%%", c.monster, c.player, got, c.want)
		}
	}
}

func TestExperienceShortOfTheNeedStaysAtTheLevel(t *testing.T) {
	c := Character{Level: 1}

	if levels := c.GainExp(299); levels != 0 || c.Level != 1 || c.Exp != 299 || c.StatPoints != 0 {
		t.Fatalf("got level %d, %d exp, %d points after %d levels", c.Level, c.Exp, c.StatPoints, levels)
	}
}

// Filling the need raises the level, carries the rest over, and brings three stat points.
func TestFillingTheNeedRaisesTheLevelAndCarriesTheRest(t *testing.T) {
	c := Character{Level: 1, Exp: 290}

	if levels := c.GainExp(15); levels != 1 || c.Level != 2 || c.Exp != 5 || c.StatPoints != 3 {
		t.Fatalf("got level %d, %d exp, %d points after %d levels", c.Level, c.Exp, c.StatPoints, levels)
	}
}

// One gain may fill several levels at once.
func TestOneGainCanRaiseSeveralLevels(t *testing.T) {
	c := Character{Level: 1}

	if levels := c.GainExp(300 + 800 + 1500 + 7); levels != 3 || c.Level != 4 || c.Exp != 7 || c.StatPoints != 9 {
		t.Fatalf("got level %d, %d exp, %d points after %d levels", c.Level, c.Exp, c.StatPoints, levels)
	}
}

func TestTheTopLevelGainsNoMore(t *testing.T) {
	c := Character{Level: MaxLevel - 1}

	c.GainExp(4_000_000_000)

	if c.Level != MaxLevel || c.Exp != 0 {
		t.Fatalf("got level %d with %d exp", c.Level, c.Exp)
	}
	if c.GainExp(1000); c.Level != MaxLevel || c.Exp != 0 {
		t.Fatalf("got level %d with %d exp after more", c.Level, c.Exp)
	}
}

func TestAStatPointGoesIntoAStatUpToItsCap(t *testing.T) {
	c := Character{StatPoints: 2, Stats: Stats{Strength: MaxStat - 1}}

	if !c.SpendStatPoint(&c.Strength) || c.Strength != MaxStat || c.StatPoints != 1 {
		t.Fatalf("got strength %d with %d points", c.Strength, c.StatPoints)
	}
	if c.SpendStatPoint(&c.Strength) || c.Strength != MaxStat || c.StatPoints != 1 {
		t.Fatalf("spent past the cap: strength %d with %d points", c.Strength, c.StatPoints)
	}
}

func TestNoStatPointNoSpending(t *testing.T) {
	c := Character{Stats: Stats{Dexterity: 3}}

	if c.SpendStatPoint(&c.Dexterity) || c.Dexterity != 3 {
		t.Fatalf("spent a point it did not have: dexterity %d", c.Dexterity)
	}
}

// Dying costs 2% of what the level needs: 6 of the 300 a level-1 character needs.
func TestDyingCostsTwoPercentOfTheLevelsNeed(t *testing.T) {
	c := Character{Level: 1, Exp: 100}
	if lost := c.LoseExpOnDeath(); lost != 6 || c.Exp != 94 {
		t.Fatalf("lost %d, left %d; want 6 and 94", lost, c.Exp)
	}
}

// The penalty never takes a level: it stops at no experience.
func TestDyingNeverTakesALevel(t *testing.T) {
	c := Character{Level: 2, Exp: 3}
	if lost := c.LoseExpOnDeath(); lost != 3 || c.Exp != 0 || c.Level != 2 {
		t.Fatalf("lost %d, left level %d with %d", lost, c.Level, c.Exp)
	}
}

// At the top level, with nothing to gain, nothing is lost.
func TestDyingAtTheTopLevelCostsNothing(t *testing.T) {
	c := Character{Level: MaxLevel}
	if lost := c.LoseExpOnDeath(); lost != 0 {
		t.Fatalf("lost %d", lost)
	}
}
