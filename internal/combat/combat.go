// Package combat is how much a blow takes: the attack a character puts into it, the
// defence of what it hits, and the damage that is left. The formula is this server's own,
// not the original's: defence takes a share of a blow rather than a fixed amount, so every
// blow counts and armour matters at every level. docs/mechanics/combat.md has the reasoning
// and the original's formula it replaces.
package combat

import (
	"math"
	"math/rand/v2"

	"github.com/SatoriAI/MT2-Go/internal/account"
)

// Attacker is what a player's blow is made of. Bare hands for now: a weapon will add twice
// its attack value once characters carry one.
type Attacker struct {
	Level uint8
	Class account.Class
	account.Stats
}

// Defender is what a monster sets against a blow.
type Defender struct {
	Level    uint8
	Vitality uint8 // the mob_proto's HT
	Armour   uint8 // the mob_proto's DEF
}

// spread is how far a blow's damage strays either way from its mean, as a share of it.
const spread = 0.1

// raceStat is the stat a class fights with besides strength.
func raceStat(a Attacker) uint8 {
	switch a.Class {
	case account.Ninja:
		return a.Dexterity
	case account.Shaman:
		return a.Intelligence
	default:
		return a.Strength
	}
}

// Attack is the strength of a's blow: twice the level, and a third of four times strength
// plus twice the class's own stat, as the client's character window counts the stat part.
func Attack(a Attacker) float64 {
	return 2*float64(a.Level) + (4*float64(a.Strength)+2*float64(raceStat(a)))/3
}

// Defence is what a monster sets against every blow.
func (d Defender) Defence() float64 {
	return float64(d.Level) + float64(d.Vitality) + float64(d.Armour)
}

// Mean is the damage a's blows on d average: the attack less the share defence takes,
// attack × 100 / (100 + defence).
func Mean(a Attacker, d Defender) float64 { return Attack(a) * 100 / (100 + d.Defence()) }

// Damage is what one blow of a on d takes from d's hit points: the mean, up to a tenth
// more or less at random, rounded, and never under 1.
func Damage(a Attacker, d Defender, r *rand.Rand) int {
	luck := 1 + spread*(2*r.Float64()-1)
	return max(1, int(math.Round(Mean(a, d)*luck)))
}
