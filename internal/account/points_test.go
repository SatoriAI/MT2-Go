package account

import "testing"

// A new character of each class, with its starting stats: HP is 700 + vitality × 50 +
// level × 100, SP the class's base + intelligence × 20 + level × 50.
func TestANewCharacterOfEachClassHasItsHitPointsAndMana(t *testing.T) {
	cases := []struct {
		class        Class
		maxHP, maxSP uint32
	}{
		{Warrior, 700 + 4*50 + 100, 500 + 3*20 + 50},
		{Ninja, 700 + 3*50 + 100, 500 + 3*20 + 50},
		{Sura, 700 + 3*50 + 100, 700 + 5*20 + 50},
		{Shaman, 700 + 4*50 + 100, 700 + 6*20 + 50},
	}
	for _, c := range cases {
		stats, _ := c.class.StartingStats()
		p, ok := Character{Class: c.class, Level: 1, Stats: stats}.Points()
		if !ok || p.MaxHP != c.maxHP || p.MaxSP != c.maxSP {
			t.Errorf("%s: got %d HP, %d SP; want %d, %d", c.class, p.MaxHP, p.MaxSP, c.maxHP, c.maxSP)
		}
		if p.NextExp != 300 || p.MaxStamina != 1000 || p.AttackSpeed != 150 || p.MoveSpeed != 150 {
			t.Errorf("%s: %+v", c.class, p)
		}
	}
}

func TestPointsGrowWithLevel(t *testing.T) {
	low, _ := Character{Class: Sura, Level: 1}.Points()
	high, _ := Character{Class: Sura, Level: 2}.Points()

	if high.MaxHP-low.MaxHP != 100 || high.MaxSP-low.MaxSP != 50 {
		t.Errorf("level 2 adds %d HP and %d SP", high.MaxHP-low.MaxHP, high.MaxSP-low.MaxSP)
	}
	if high.NextExp != 800 {
		t.Errorf("level 2 needs %d to reach 3, want 800", high.NextExp)
	}
}

func TestAClassTheGameDoesNotHaveHasNoPoints(t *testing.T) {
	if _, ok := (Character{Class: "archer", Level: 1}).Points(); ok {
		t.Error("got points")
	}
}
