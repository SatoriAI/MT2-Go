package combat

import (
	"math"
	"math/rand/v2"
	"testing"

	"github.com/SatoriAI/MT2-Go/internal/account"
)

// A wild dog (mob_proto 101): level 1, HT 5, DEF 4.
var wildDog = Defender{Level: 1, Vitality: 5, Armour: 4}

var levelOneSura = Attacker{Level: 1, Class: account.Sura, Stats: account.Stats{Strength: 5, Dexterity: 3, Intelligence: 5, Vitality: 3}}

func fixedRand() *rand.Rand { return rand.New(rand.NewPCG(1, 2)) } //nolint:gosec // a fixed seed for a test

func TestAWildDogsDefenceIsItsLevelVitalityAndArmour(t *testing.T) {
	if got := wildDog.Defence(); got != 10 {
		t.Fatalf("got %v, want 1 + 5 + 4", got)
	}
}

// A level-1 sura bare-handed: twice the level, and (4×5 + 2×5) / 3 for strength and the
// sura's own stat, strength again.
func TestALevelOneSurasAttackIsTwiceItsLevelAndItsStatPart(t *testing.T) {
	if got := Attack(levelOneSura); got != 12 {
		t.Fatalf("got %v, want 2 + 10", got)
	}
}

// Each class fights with strength and its own stat: a ninja's dexterity, a shaman's
// intelligence.
func TestEachClassAddsItsOwnStat(t *testing.T) {
	stats := account.Stats{Strength: 6, Dexterity: 30, Intelligence: 60}
	warrior := Attack(Attacker{Level: 1, Class: account.Warrior, Stats: stats})
	ninja := Attack(Attacker{Level: 1, Class: account.Ninja, Stats: stats})
	shaman := Attack(Attacker{Level: 1, Class: account.Shaman, Stats: stats})

	if warrior >= ninja || ninja >= shaman {
		t.Fatalf("warrior %v, ninja %v, shaman %v: each should follow its own stat", warrior, ninja, shaman)
	}
}

// Defence takes a share of the blow: 12 against a defence of 10 is 12 × 100 / 110.
func TestDefenceTakesAShareOfTheBlow(t *testing.T) {
	if got, want := Mean(levelOneSura, wildDog), 1200.0/110; math.Abs(got-want) > 1e-9 {
		t.Fatalf("got %v, want %v", got, want)
	}
}

// However high the defence, a share of the blow gets through: never nothing, as the
// original's subtraction gave.
func TestNoDefenceStopsABlowWhole(t *testing.T) {
	fortress := Defender{Level: 255, Vitality: 255, Armour: 255}
	if got := Mean(levelOneSura, fortress); got <= 1 {
		t.Fatalf("a level-1 sura's blow on a fortress averages %v", got)
	}
}

// A blow strays up to a tenth either way from its mean, and covers that range.
func TestABlowStraysUpToATenthEitherWay(t *testing.T) {
	strong := Attacker{Level: 30, Class: account.Warrior, Stats: account.Stats{Strength: 80}}
	mean := Mean(strong, wildDog)
	r := fixedRand()
	low, high := math.Inf(1), math.Inf(-1)
	for range 2000 {
		got := float64(Damage(strong, wildDog, r))
		low, high = math.Min(low, got), math.Max(high, got)
	}
	if low < math.Round(mean*0.9) || high > math.Round(mean*1.1) || low > mean*0.92 || high < mean*1.08 {
		t.Fatalf("blows from %v to %v, want the whole of %v to %v", low, high, mean*0.9, mean*1.1)
	}
}

// A blow always takes something.
func TestABlowTakesAtLeastOne(t *testing.T) {
	feeble := Attacker{Class: account.Warrior}
	fortress := Defender{Level: 255, Vitality: 255, Armour: 255}
	r := fixedRand()
	for range 100 {
		if got := Damage(feeble, fortress, r); got < 1 {
			t.Fatalf("a blow took %d", got)
		}
	}
}
