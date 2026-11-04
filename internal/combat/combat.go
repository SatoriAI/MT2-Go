// Package combat is how much a blow takes: the attack a character puts into it, the
// defence and evasion of what it hits, and the damage that is left. The formulas are the
// client's own where it shows them (its character window's attack, PythonPlayer.cpp) and
// open-mt2's for the rest (evasion and a monster's defence); docs/mechanics/combat.md has
// the sources.
package combat

import (
	"math"
	"math/rand/v2"

	"github.com/SatoriAI/MT2-Go/internal/account"
)

// Attacker is what a player's blow is made of. Bare hands for now: weapons add to it once
// characters carry them.
type Attacker struct {
	Level uint8
	Class account.Class
	account.Stats
}

// Defender is what a monster sets against a blow.
type Defender struct {
	Level     uint8
	Dexterity uint8
	Vitality  uint8 // the mob_proto's HT
	Armour    uint8 // the mob_proto's DEF
}

// rating is how sure-footed a fighter is, for hitting and for dodging alike.
func rating(level, dexterity uint8) float64 {
	return math.Min(90, (4*float64(dexterity)+2*float64(level))/6)
}

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

// Attack is the blow's strength against d, before d's defence: the client's attack value,
// with the victim's evasion taken off its hit rate.
func Attack(a Attacker, d Defender) int {
	statAttack := (4*float64(a.Strength) + 2*float64(raceStat(a))) / 3
	hit := (rating(a.Level, a.Dexterity) + 210) / 300
	evade := (rating(d.Level, d.Dexterity)*2 + 5) / (rating(d.Level, d.Dexterity) + 95) * 0.3
	return int(math.Floor(statAttack*(hit-evade))) + 2*int(a.Level)
}

// Defence is what a monster takes off every blow.
func (d Defender) Defence() int { return int(d.Level) + int(d.Vitality) + int(d.Armour) }

// Damage is what one blow of a on d takes from d's hit points. A blow its defence stops
// still grazes, for 1 to 5.
func Damage(a Attacker, d Defender, r *rand.Rand) int {
	if dmg := Attack(a, d) - d.Defence(); dmg > 0 {
		return dmg
	}
	return 1 + r.IntN(5)
}
