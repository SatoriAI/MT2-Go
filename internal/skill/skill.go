// Package skill holds the skills the game has: whose they are, how far they rise, and how
// many skill points a character has to raise them with.
package skill

import (
	"time"

	"github.com/SatoriAI/MT2-Go/internal/account"
)

// The skills the game has so far: the sura's Weaponry attacks, on one and on many, and its
// buffs of attack and defence.
const (
	FingerStrike    account.Skill = "finger_strike"
	DragonSwirl     account.Skill = "dragon_swirl"
	EnchantedBlade  account.Skill = "enchanted_blade"
	EnchantedArmour account.Skill = "enchanted_armour"
)

// MaxLevel is the most a skill rises to with points; the client hides its + there.
const MaxLevel = 20

// FirstLevel is the level a character earns its first skill point at, and from which a
// teacher takes it on; each level after brings one more.
const FirstLevel = 5

// Proto is what one skill is: the class and skill group whose it is, and what using it
// takes and does. Its mana, cooldown, damage and buff grow with its power, as the client
// works them out.
type Proto struct {
	Class account.Class
	Group account.SkillGroup
	// Mana is what using it costs; Cooldown, in seconds, how long before it may be used
	// again.
	Mana, Cooldown Scaled
	// An attack strikes the one monster the player picked within a blow's reach, with
	// Target, or every monster within Reach, for Damage percent of a blow.
	Target bool
	Reach  int64
	Damage Scaled
	// A buff raises what Raises names for Lasts seconds. Toggle is that using it again
	// ends it; NeedsSword, that it runs only with a sword in hand.
	Raises     Stat
	Lasts      Scaled
	Toggle     bool
	NeedsSword bool
}

// Stat is what a buff raises.
type Stat string

const (
	Attack  Stat = "attack"
	Defence Stat = "defence"
)

// Scaled is a value of Base, plus PerPower times a skill's power in hundredths, divided
// by 100: the shape of the client's skill formulas.
type Scaled struct {
	Base, PerPower uint32
}

// At is the value at a power, rounded down as the client rounds it.
func (v Scaled) At(power uint32) uint32 { return v.Base + v.PerPower*power/100 }

// CooldownAt is how long the skill waits at level before it may be used again.
func (p Proto) CooldownAt(level uint8) time.Duration {
	return time.Duration(p.Cooldown.At(Power(level))) * time.Second
}

// LastsAt is how long the buff runs at level.
func (p Proto) LastsAt(level uint8) time.Duration {
	return time.Duration(p.Lasts.At(Power(level))) * time.Second
}

// Bonus is what the buff adds, at its level, for a character of intelligence, as the
// client shows it, rounded down: Enchanted Blade adds 7 + (5·INT + 13)·k to the attack,
// its tooltip; Enchanted Armour (INT + 30)·k to the defence, its skilltable.
func (p Proto) Bonus(intelligence, level uint8) uint16 {
	k := Power(level)
	switch p.Raises {
	case Attack:
		return uint16(7 + (5*uint32(intelligence)+13)*k/100) //nolint:gosec // at most 7 + 1288/2
	case Defence:
		return uint16((uint32(intelligence) + 30) * k / 100) //nolint:gosec // at most 285/2
	}
	return 0
}

var protos = map[account.Skill]Proto{
	// The client's skilltable: 30 + 140k mana, 10 s, one target. The damage is the
	// veteran's: (250 + 350k)% of a blow, harder than a swirl, on one.
	FingerStrike: {Class: account.Sura, Group: "weaponry", Mana: Scaled{30, 140}, Cooldown: Scaled{10, 0},
		Target: true, Damage: Scaled{250, 350}},
	// The client's skilltable: 50 + 150k mana, 15 s, a splash of 500. The damage is this
	// server's own: twice a blow at the first level, three and a half at 20.
	DragonSwirl: {Class: account.Sura, Group: "weaponry", Mana: Scaled{50, 150}, Cooldown: Scaled{15, 0},
		Reach: 500, Damage: Scaled{200, 300}},
	// The client's skilltable: 20 + 240k mana, a toggle. It lasts three minutes, as the
	// veteran argued and Dawid chose, where the client's lasts 50 + 100k seconds.
	EnchantedBlade: {Class: account.Sura, Group: "weaponry", Mana: Scaled{20, 240}, Raises: Attack,
		Lasts: Scaled{180, 0}, Toggle: true, NeedsSword: true},
	// The client's skilltable, all of it: 70 + 170k mana, 33 + 140k seconds' cooldown, a
	// run of 30 + 120k seconds, so it drops for a few between casts.
	EnchantedArmour: {Class: account.Sura, Group: "weaponry", Mana: Scaled{70, 170}, Cooldown: Scaled{33, 140},
		Raises: Defence, Lasts: Scaled{30, 120}},
}

// powers is a skill's power at each level, in hundredths: the client's own table, which
// its formulas for mana and cooldowns read too.
var powers = [MaxLevel + 1]uint32{0, 5, 6, 8, 10, 12, 14, 16, 18, 20, 22, 24, 26, 28, 30, 32, 34, 36, 38, 40, 50}

// Power is a skill's power at level, in hundredths; 0 for an unlearnt one.
func Power(level uint8) uint32 { return powers[min(level, MaxLevel)] }

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
