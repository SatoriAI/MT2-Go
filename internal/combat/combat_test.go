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

// A wild dog's bite (mob_proto 101: level 1, ST 3, damage 20–24): 2 + 6 and its roll, 28 to
// 32, of which a level-1 sura's defence of 1 + 3 takes a share, ×100/104: 27 to 31.
var wildDogBite = Biter{Level: 1, Strength: 3, DamageMin: 20, DamageMax: 24}

var levelOneSuraDefence = Defender{Level: 1, Vitality: 3}

func TestAWildDogBitesALevelOneSuraForTwentySevenToThirtyOne(t *testing.T) {
	r := fixedRand()
	seen := map[int]bool{}
	for range 1000 {
		got := Bite(wildDogBite, levelOneSuraDefence, r)
		if got < 27 || got > 31 {
			t.Fatalf("bit for %d, want 27 to 31", got)
		}
		seen[got] = true
	}
	if len(seen) != 5 {
		t.Fatalf("bit for %v over 1000 bites, want every one of 27 to 31", seen)
	}
}

// A boar (level 7, ST 14, damage 30–37) bites harder than a dog: 14 + 28 and its roll.
func TestAStrongerMonsterBitesHarder(t *testing.T) {
	boar := Biter{Level: 7, Strength: 14, DamageMin: 30, DamageMax: 37}
	if got := Bite(boar, levelOneSuraDefence, fixedRand()); got < 69 || got > 76 {
		t.Fatalf("a boar bit for %d, want 72 to 79 less the share defence takes, 69 to 76", got)
	}
}

// Defence takes its share of a bite too: a level-30 player with vitality 60 takes about
// half of a dog's.
func TestDefenceTakesAShareOfABite(t *testing.T) {
	sturdy := Defender{Level: 30, Vitality: 60}
	if got := Bite(wildDogBite, sturdy, fixedRand()); got < 15 || got > 17 {
		t.Fatalf("got %d, want 28 to 32 × 100/190", got)
	}
}
