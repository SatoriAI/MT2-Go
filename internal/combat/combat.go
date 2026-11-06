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

// Attacker is what a player's blow is made of: their level, class and stats, the attack
// of the weapon they hold, 0 to 0 bare-handed, and what a running buff adds.
type Attacker struct {
	Level uint8
	Class account.Class
	account.Stats
	WeaponMin, WeaponMax uint16
	Bonus                uint16
}

// Defender is what is set against a blow: a monster's, from its mob_proto, or a player's,
// their level and vitality, and the defence of the armour they wear.
type Defender struct {
	Level    uint8
	Vitality uint8  // a monster's HT
	Armour   uint16 // a monster's DEF
}

// Biter is what a monster's bite is made of: its level and strength, as the original
// counts a monster's attack, and the dice of its mob_proto damage.
type Biter struct {
	Level     uint8
	Strength  uint8
	DamageMin uint16
	DamageMax uint16
}

// Bite is what one of b's bites takes from d: twice its level and strength and a roll of
// its dice, close to the original's monster, less the share d's defence takes, rounded
// and never under 1. The roll is the bite's spread, so nothing is added to it.
func Bite(b Biter, d Defender, r *rand.Rand) int {
	roll := int(b.DamageMin) + r.IntN(int(b.DamageMax)-int(b.DamageMin)+1)
	attack := 2*float64(b.Level) + 2*float64(b.Strength) + float64(roll)
	return max(1, int(math.Round(attack*100/(100+d.Defence()))))
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
// plus twice the class's own stat, as the client's character window counts the stat part,
// and what a buff adds.
func Attack(a Attacker) float64 {
	return 2*float64(a.Level) + (4*float64(a.Strength)+2*float64(raceStat(a)))/3 + float64(a.Bonus)
}

// Defence is what a monster sets against every blow.
func (d Defender) Defence() float64 {
	return float64(d.Level) + float64(d.Vitality) + float64(d.Armour)
}

// Mean is the damage a's blows on d average: the attack, with twice the weapon's average
// attack, less the share defence takes, attack × 100 / (100 + defence).
func Mean(a Attacker, d Defender) float64 {
	return (Attack(a) + float64(a.WeaponMin) + float64(a.WeaponMax)) * 100 / (100 + d.Defence())
}

// Damage is what one blow of a on d takes from d's hit points: the attack and twice a
// roll of the weapon's attack, less the share d's defence takes, up to a tenth more or
// less at random, rounded, and never under 1.
func Damage(a Attacker, d Defender, r *rand.Rand) int {
	luck := 1 + spread*(2*r.Float64()-1)
	if a.WeaponMax > 0 {
		roll := a.WeaponMin + uint16(r.IntN(int(a.WeaponMax-a.WeaponMin)+1)) //nolint:gosec // within the weapon's range
		a.WeaponMin, a.WeaponMax = roll, roll
	}
	return max(1, int(math.Round(Mean(a, d)*luck)))
}
