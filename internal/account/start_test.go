package account

import "testing"

// The client's START_STAT table in introcreate.py, as CON, INT, STR, DEX; the same for
// both sexes.
func TestEveryClassStartsWithTheStatsTheCreateScreenShows(t *testing.T) {
	want := map[Class]Stats{
		Warrior: {Vitality: 4, Intelligence: 3, Strength: 6, Dexterity: 3},
		Ninja:   {Vitality: 3, Intelligence: 3, Strength: 4, Dexterity: 6},
		Sura:    {Vitality: 3, Intelligence: 5, Strength: 5, Dexterity: 3},
		Shaman:  {Vitality: 4, Intelligence: 6, Strength: 3, Dexterity: 3},
	}
	for class, stats := range want {
		if got, ok := class.StartingStats(); !ok || got != stats {
			t.Errorf("%s: got %+v, want %+v", class, got, stats)
		}
	}
	if _, ok := Class("archer").StartingStats(); ok {
		t.Error("a class the game does not have has starting stats")
	}
}

// Each kingdom's first village, from open-mt2's empire.json; each lies inside that
// village's map in the client's atlasinfo.txt (metin2_map_a1, b1, c1).
func TestEveryKingdomStartsInItsFirstVillage(t *testing.T) {
	want := map[Empire][2]int32{
		EmpireShinsoo: {475000, 966100},
		EmpireChunjo:  {60000, 156000},
		EmpireJinno:   {963400, 278200},
	}
	for empire, at := range want {
		if x, y, ok := empire.StartPosition(); !ok || x != at[0] || y != at[1] {
			t.Errorf("%s: got %d, %d", empire, x, y)
		}
	}
	if _, _, ok := EmpireNone.StartPosition(); ok {
		t.Error("no kingdom has a start position")
	}
}
