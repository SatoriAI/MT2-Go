package item

import (
	"math/rand/v2"
	"slices"

	"github.com/SatoriAI/MT2-Go/internal/account"
)

// Rolled is one bonus an item carries, at its value.
type Rolled struct {
	Kind  account.Bonus
	Value int16
}

// Bonuses are what an item carries, in the order rolled: at most MaxBonuses.
type Bonuses []Rolled

// MaxBonuses is how many bonus slots an item has, the client's seven.
const MaxBonuses = 7

// Sum is what the bonuses add to the totals.
func (b Bonuses) Sum(t *account.Totals) {
	for _, r := range b {
		t.Add(r.Kind, int(r.Value))
	}
}

// The values each bonus rolls at, by grade, lowest first, as the debaters settled for
// levels up to 36 (docs/mechanics/bonuses.md).
var bonusValues = map[account.Bonus][5]int16{
	account.BonusMaxHP:       {50, 100, 180, 280, 400},
	account.BonusAttackSpeed: {1, 2, 3, 4, 5},
	account.BonusCritical:    {1, 2, 3, 5, 8},
	account.BonusPierce:      {1, 2, 3, 5, 8},
	account.BonusAnimals:     {2, 4, 6, 8, 10},
	account.BonusResist:      {1, 2, 3, 4, 5},
}

// gradeWeights are how likely each grade is, in hundredths: the top one rare.
var gradeWeights = [5]int{40, 30, 18, 9, 3}

// topFrom is the level gear rolls the top grade from; below it, the top grade never comes.
const topFrom = 25

// allowed are the bonuses each place may roll.
var allowed = map[Wear][]account.Bonus{
	Weapon: {account.BonusAnimals, account.BonusCritical, account.BonusPierce},
	Body:   {account.BonusMaxHP, account.BonusResist},
	Head:   {account.BonusAttackSpeed, account.BonusResist, account.BonusAnimals},
	Shield: {account.BonusMaxHP, account.BonusResist, account.BonusAnimals},
	Feet:   {account.BonusAttackSpeed, account.BonusMaxHP, account.BonusCritical},
}

// countWeights are how likely a stone's piece of gear carries one, two or three bonuses,
// in hundredths.
var countWeights = [3]int{50, 35, 15}

// The pair: a level-30 or level-36 weapon carries, pairChance times in 100, Średnie
// Obrażenia or Obrażenie Umiejętności at a grade's strong value, and the other at its weak
// one taken off, whichever way round at even odds: a weapon for hitting or for skills,
// never for both.
const pairChance = 15

var (
	pairStrong = map[account.Bonus][5]int16{account.BonusAverage: {10, 15, 20, 25, 30}, account.BonusSkill: {5, 8, 10, 12, 15}}
	pairWeak   = [5]int16{5, 8, 10, 12, 15}
	pairLevels = []uint8{30, 36}
)

// Roll is what a piece of gear of kind p carries as it drops: from a Metin stone, one to
// three bonuses its place allows, each of its own kind; and for a level-30 or level-36
// weapon, from anything, sometimes the pair. Anything else carries none.
func Roll(p Proto, fromStone bool, r *rand.Rand) Bonuses {
	kinds, worn := allowed[p.Wear]
	if !worn {
		return nil
	}
	var b Bonuses
	if fromStone {
		kinds = slices.Clone(kinds)
		r.Shuffle(len(kinds), func(i, j int) { kinds[i], kinds[j] = kinds[j], kinds[i] })
		for _, kind := range kinds[:min(len(kinds), weighted(countWeights[:], r)+1)] {
			b = append(b, Rolled{kind, bonusValues[kind][grade(p.Level, r)]})
		}
	}
	if p.Wear == Weapon && slices.Contains(pairLevels, p.Level) && r.IntN(100) < pairChance {
		g := grade(p.Level, r)
		strong, weak := account.BonusAverage, account.BonusSkill
		if r.IntN(2) == 1 {
			strong, weak = weak, strong
		}
		b = append(b, Rolled{strong, pairStrong[strong][g]}, Rolled{weak, -pairWeak[g]})
	}
	return b
}

// grade rolls a grade, never the top one below topFrom.
func grade(level uint8, r *rand.Rand) int {
	if level < topFrom {
		return weighted(gradeWeights[:4], r)
	}
	return weighted(gradeWeights[:], r)
}

// weighted picks an index with the weights' odds.
func weighted(weights []int, r *rand.Rand) int {
	total := 0
	for _, w := range weights {
		total += w
	}
	roll := r.IntN(total)
	for i, w := range weights {
		if roll < w {
			return i
		}
		roll -= w
	}
	return len(weights) - 1
}

// Remarkable is whether the bonuses are worth telling everyone about: three besides a
// pair, or any at its top grade.
func (b Bonuses) Remarkable() bool {
	if len(slices.DeleteFunc(slices.Clone(b), func(r Rolled) bool { _, paired := pairStrong[r.Kind]; return paired })) >= 3 {
		return true
	}
	for _, r := range b {
		if top, graded := bonusValues[r.Kind]; graded && r.Value == top[4] {
			return true
		}
		if top, strong := pairStrong[r.Kind]; strong && r.Value == top[4] {
			return true
		}
	}
	return false
}
