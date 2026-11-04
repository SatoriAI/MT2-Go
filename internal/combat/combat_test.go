package combat

import (
	"math/rand/v2"
	"testing"

	"github.com/SatoriAI/MT2-Go/internal/account"
)

// A wild dog (mob_proto 101): level 1, DX 6, HT 5, DEF 4.
var wildDog = Defender{Level: 1, Dexterity: 6, Vitality: 5, Armour: 4}

func TestAWildDogsDefenceIsItsLevelVitalityAndArmour(t *testing.T) {
	if got := wildDog.Defence(); got != 10 {
		t.Fatalf("got %d, want 1 + 5 + 4", got)
	}
}

// A level-1 sura bare-handed: ST 5, so a stat attack of (4*5 + 2*5) / 3 = 10; a hit rate of
// (14/6 + 210) / 300, less the dog's evasion of (2*26/6 + 5) / (26/6 + 95) * 0.3; floored,
// plus twice the level.
func TestALevelOneSurasAttackIsTheClientsLessTheDogsEvasion(t *testing.T) {
	sura := Attacker{Level: 1, Class: account.Sura, Stats: account.Stats{Strength: 5, Dexterity: 3, Intelligence: 5, Vitality: 3}}

	if got := Attack(sura, wildDog); got != 8 {
		t.Fatalf("got %d, want floor(10 * 0.6665) + 2 = 8", got)
	}
}

// Each class fights with strength and its own stat: a ninja's dexterity, a shaman's
// intelligence.
func TestEachClassAddsItsOwnStat(t *testing.T) {
	stats := account.Stats{Strength: 6, Dexterity: 30, Intelligence: 60}
	nobody := Defender{}
	warrior := Attack(Attacker{Level: 1, Class: account.Warrior, Stats: stats}, nobody)
	ninja := Attack(Attacker{Level: 1, Class: account.Ninja, Stats: stats}, nobody)
	shaman := Attack(Attacker{Level: 1, Class: account.Shaman, Stats: stats}, nobody)

	if warrior >= ninja || ninja >= shaman {
		t.Fatalf("warrior %d, ninja %d, shaman %d: each should follow its own stat", warrior, ninja, shaman)
	}
}

func TestABlowStrongerThanTheDefenceTakesTheDifference(t *testing.T) {
	strong := Attacker{Level: 20, Class: account.Warrior, Stats: account.Stats{Strength: 40, Dexterity: 20}}
	want := Attack(strong, wildDog) - 10

	if got := Damage(strong, wildDog, rand.New(rand.NewPCG(1, 2))); got != want || want <= 0 { //nolint:gosec // a fixed seed for a test
		t.Fatalf("got %d, want %d", got, want)
	}
}

// A blow the defence stops still grazes: 1 to 5, each in turn.
func TestABlowTheDefenceStopsGrazesForOneToFive(t *testing.T) {
	weak := Attacker{Level: 1, Class: account.Sura, Stats: account.Stats{Strength: 5, Dexterity: 3}}
	r := rand.New(rand.NewPCG(1, 2)) //nolint:gosec // a fixed seed for a test
	seen := map[int]bool{}
	for range 200 {
		got := Damage(weak, wildDog, r)
		if got < 1 || got > 5 {
			t.Fatalf("grazed for %d", got)
		}
		seen[got] = true
	}
	if len(seen) != 5 {
		t.Fatalf("grazes %v, want each of 1 to 5", seen)
	}
}

// Rating stops at 90, however high level and dexterity go.
func TestRatingStopsAtNinety(t *testing.T) {
	if got := rating(255, 255); got != 90 {
		t.Fatalf("got %v", got)
	}
}
