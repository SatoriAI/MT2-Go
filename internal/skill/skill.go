// Package skill holds the skills the game has: whose they are, how far they rise, and how
// many skill points a character has to raise them with.
package skill

import (
	"time"

	"github.com/SatoriAI/MT2-Go/internal/account"
)

// The skills the game has so far: the sura's Weaponry area attack and its attack buff.
const (
	DragonSwirl    account.Skill = "dragon_swirl"
	EnchantedBlade account.Skill = "enchanted_blade"
)

// MaxLevel is the most a skill rises to with points; the client hides its + there.
const MaxLevel = 20

// FirstLevel is the level a character earns its first skill point at, and from which a
// teacher takes it on; each level after brings one more.
const FirstLevel = 5

// Proto is what one skill is: the class and skill group whose it is, and what using it
// takes and does. Mana and damage grow with its power, as the client works them out.
type Proto struct {
	Class account.Class
	Group account.SkillGroup
	// Mana is what using it costs: Base, and PerPower hundredths of its power more.
	Mana     Scaled
	Cooldown time.Duration // before it may be used again
	// An attack hits every monster within Reach of the player, for Damage percent of a
	// blow, in hundredths of its power more as PerPower.
	Reach  int64
	Damage Scaled
	// A buff lasts for Lasts and adds to every blow 7 + (5·INT + 13), times its power, as
	// the client's tooltip shows it; NeedsSword is that it runs only with a sword in hand.
	Lasts      time.Duration
	NeedsSword bool
}

// Scaled is a value of Base, plus PerPower times a skill's power in hundredths, divided
// by 100: the shape of the client's skill formulas.
type Scaled struct {
	Base, PerPower uint32
}

// At is the value at a power, rounded down as the client rounds it.
func (v Scaled) At(power uint32) uint32 { return v.Base + v.PerPower*power/100 }

var protos = map[account.Skill]Proto{
	// The client's skilltable: 50 + 150k mana, 15 s, a splash of 500. The damage is this
	// server's own: twice a blow at the first level, three and a half at 20.
	DragonSwirl: {Class: account.Sura, Group: "weaponry", Mana: Scaled{50, 150}, Cooldown: 15 * time.Second,
		Reach: 500, Damage: Scaled{200, 300}},
	// The client's skilltable: 20 + 240k mana, a toggle. It lasts three minutes, as the
	// veteran argued and Dawid chose, where the client's lasts 50 + 100k seconds.
	EnchantedBlade: {Class: account.Sura, Group: "weaponry", Mana: Scaled{20, 240}, Lasts: 3 * time.Minute,
		NeedsSword: true},
}

// powers is a skill's power at each level, in hundredths: the client's own table, which
// its formulas for mana and cooldowns read too.
var powers = [MaxLevel + 1]uint32{0, 5, 6, 8, 10, 12, 14, 16, 18, 20, 22, 24, 26, 28, 30, 32, 34, 36, 38, 40, 50}

// Power is a skill's power at level, in hundredths; 0 for an unlearnt one.
func Power(level uint8) uint32 { return powers[min(level, MaxLevel)] }

// Bonus is what a running buff adds to a blow of a character of intelligence, at the
// buff's level: 7 + (5·INT + 13)·k, the client's skilldesc, so its tooltip is what is
// given, rounded down.
func Bonus(intelligence, skillLevel uint8) uint16 {
	return uint16(7 + (5*uint32(intelligence)+13)*Power(skillLevel)/100) //nolint:gosec // at most 7 + 1288/2
}

// Lookup returns the skill, or false if the game has none of it.
func Lookup(s account.Skill) (Proto, bool) {
	p, ok := protos[s]
	return p, ok
}

// Points is how many skill points a character of level has left once it has learnt what
// it has: one for every level from FirstLevel, less a point for every skill level.
func Points(level uint8, learnt map[account.Skill]uint8) uint32 {
	if level < FirstLevel {
		return 0
	}
	earned := int(level) - FirstLevel + 1
	for _, l := range learnt {
		earned -= int(l)
	}
	return uint32(max(0, earned)) //nolint:gosec // never more than a level
}
