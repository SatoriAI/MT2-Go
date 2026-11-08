// Package skill holds the skills the game has: whose they are, how far they rise, and how
// many skill points a character has to raise them with.
package skill

import (
	"time"

	"github.com/SatoriAI/MT2-Go/internal/account"
	"github.com/SatoriAI/MT2-Go/internal/item"
)

// The skills the game has so far: the sura's Weaponry attacks, on one and on many, and its
// buffs of attack and defence; the warrior's Body attacks, around a target and around
// themselves, and its buffs of speed and attack; and the warrior's Mental attacks, which
// stun, and its buff of defence.
const (
	FingerStrike    account.Skill = "finger_strike"
	DragonSwirl     account.Skill = "dragon_swirl"
	EnchantedBlade  account.Skill = "enchanted_blade"
	EnchantedArmour account.Skill = "enchanted_armour"
	ThreeWayCut     account.Skill = "three_way_cut"
	SwordSpin       account.Skill = "sword_spin"
	Berserk         account.Skill = "berserk"
	SwordAura       account.Skill = "sword_aura"
	SpiritStrike    account.Skill = "spirit_strike"
	Bash            account.Skill = "bash"
	Stump           account.Skill = "stump"
	StrongBody      account.Skill = "strong_body"
	SwordStrike     account.Skill = "sword_strike"
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
	// An attack strikes, for Damage percent of a blow: with Target, the one monster the
	// player picked within a blow's reach, or with Aim, the one picked within Aim or else
	// the nearest in front of them; and with Reach, every monster within Reach of it, or
	// without either, of the player, only those in front of them with Front. At most
	// Victims are struck, the nearest first, none for no limit, each Hits times, none for
	// once. With Stuns, each one left alive may be stunned.
	Target  bool
	Aim     int64
	Reach   int64
	Front   bool
	Victims int
	Hits    int
	Damage  Scaled
	Stuns   bool
	// A buff raises each stat Raises names, by its formula, for Lasts seconds. Toggle is
	// that using it again ends it.
	Raises map[Stat]Formula
	Lasts  Scaled
	Toggle bool
	// Wields is the weapon the skill needs in hand, and a buff runs only with it.
	Wields Wield
}

// Stat is what a buff raises.
type Stat string

const (
	Attack      Stat = "attack"
	Defence     Stat = "defence"
	AttackSpeed Stat = "attack_speed"
	MoveSpeed   Stat = "move_speed"
	DamageTaken Stat = "damage_taken" // a percent more of every bite
)

// Formula is what a buff adds to a stat, or takes off it, for its bearer at a power k,
// all in hundredths, before it is rounded towards zero.
type Formula func(b Bearer, k int32) int32

// Bearer is what of a character a buff's formula reads.
type Bearer struct {
	Strength, Vitality, Intelligence uint8
}

// Wield is the weapon a skill needs in hand.
type Wield string

const (
	AnyWeapon        Wield = ""
	Sword            Wield = "sword"               // a one-handed sword
	SwordOrTwoHanded Wield = "sword_or_two_handed" // the client's SWORD|TWO_HANDED
)

// Wieldable is whether the skill may be used, or its buff run, with the weapon in hand.
func (p Proto) Wieldable(weapon item.Proto) bool {
	switch p.Wields {
	case Sword:
		return weapon.Sword
	case SwordOrTwoHanded:
		return weapon.Sword || weapon.TwoHanded
	}
	return true
}

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

// Bonus is what the buff adds to the stat, or takes off it, at its level, for its bearer,
// rounded towards zero; 0 for a stat it does not change.
func (p Proto) Bonus(stat Stat, b Bearer, level uint8) int {
	f, ok := p.Raises[stat]
	if !ok {
		return 0
	}
	return int(f(b, int32(Power(level))) / 100) //nolint:gosec // the power is at most 50
}

// StunChance is the chance, in thousandths, a stunning attack at level stuns each monster
// it leaves alive: the client's (100 + 1000k/6)/10 percent.
func StunChance(level uint8) int { return 100 + 10*int(Power(level))/6 }

// Raise reports whether the buff raises the stat.
func (p Proto) Raise(stat Stat) bool {
	_, ok := p.Raises[stat]
	return ok
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
	EnchantedBlade: {Class: account.Sura, Group: "weaponry", Mana: Scaled{20, 240},
		Raises: map[Stat]Formula{Attack: func(b Bearer, k int32) int32 { return 700 + (5*int32(b.Intelligence)+13)*k }},
		Lasts:  Scaled{180, 0}, Toggle: true, Wields: Sword},
	// The client's skilltable, all of it: 70 + 170k mana, 33 + 140k seconds' cooldown, a
	// run of 30 + 120k seconds, so it drops for a few between casts.
	EnchantedArmour: {Class: account.Sura, Group: "weaponry", Mana: Scaled{70, 170}, Cooldown: Scaled{33, 140},
		Raises: map[Stat]Formula{Defence: func(b Bearer, k int32) int32 { return (int32(b.Intelligence) + 30) * k }},
		Lasts:  Scaled{30, 120}},
	// The client's skilltable: 40 + 100k mana, 12 s, up to 5 struck in a splash of 200,
	// a sword or two-handed weapon. Its tooltip is three cuts of 1.1 + 0.1k of the attack,
	// and STR: here three hits of (110 + 10k)% of a blow on each.
	ThreeWayCut: {Class: account.Warrior, Group: "body", Mana: Scaled{40, 100}, Cooldown: Scaled{12, 0},
		Target: true, Reach: 200, Victims: 5, Hits: 3, Damage: Scaled{110, 10}, Wields: SwordOrTwoHanded},
	// The client's skilltable: 50 + 130k mana, 15 s, up to 12 struck in a splash of 200
	// around the warrior. Its tooltip is 3 + 0.8k of the attack, and stats: here
	// (300 + 80k)% of a blow.
	SwordSpin: {Class: account.Warrior, Group: "body", Mana: Scaled{50, 130}, Cooldown: Scaled{15, 0},
		Reach: 200, Victims: 12, Damage: Scaled{300, 80}, Wields: SwordOrTwoHanded},
	// The client's skilltable: 50 + 140k mana, 63 + 90k seconds' cooldown, a run of
	// 60 + 90k, 50k attack speed and 20k move speed. The damage it costs the warrior, which
	// the client's description names but gives no number for, is the veteran's: (5 + 10k)%
	// more of every bite.
	Berserk: {Class: account.Warrior, Group: "body", Mana: Scaled{50, 140}, Cooldown: Scaled{63, 90},
		Raises: map[Stat]Formula{
			AttackSpeed: func(_ Bearer, k int32) int32 { return 50 * k },
			MoveSpeed:   func(_ Bearer, k int32) int32 { return 20 * k },
			DamageTaken: func(_ Bearer, k int32) int32 { return 500 + 10*k },
		},
		Lasts: Scaled{60, 90}},
	// The client's skilltable: 100 + 200k mana, 33 + 50k seconds' cooldown, a run of
	// 30 + 50k, a sword or two-handed weapon; (100 + STR)·k to the attack, its tooltip.
	SwordAura: {Class: account.Warrior, Group: "body", Mana: Scaled{100, 200}, Cooldown: Scaled{33, 50},
		Raises: map[Stat]Formula{Attack: func(b Bearer, k int32) int32 { return (100 + int32(b.Strength)) * k }},
		Lasts:  Scaled{30, 50}, Wields: SwordOrTwoHanded},
	// Mental, from the client's skilltable: mana, cooldowns, victims and splash. The damage
	// is its tooltips' attack terms, as for Body, and the stuns last two seconds, at the
	// client's chance. Spirit Strike: 60 + 120k, 15 s, on its target and up to 3 more
	// within 100 of it, a sword or two-handed weapon.
	SpiritStrike: {Class: account.Warrior, Group: "mental", Mana: Scaled{60, 120}, Cooldown: Scaled{15, 0},
		Target: true, Reach: 100, Victims: 4, Damage: Scaled{230, 400}, Wields: SwordOrTwoHanded},
	// Bash: 60 + 150k, 15 s, up to 8 within 200 in front of the warrior, the veteran's
	// reading of its slam.
	Bash: {Class: account.Warrior, Group: "mental", Mana: Scaled{60, 150}, Cooldown: Scaled{15, 0},
		Reach: 200, Front: true, Victims: 8, Damage: Scaled{230, 300}, Wields: SwordOrTwoHanded},
	// Stump: 50 + 140k, 25 s, up to 10 within 400 around him, each perhaps stunned.
	Stump: {Class: account.Warrior, Group: "mental", Mana: Scaled{50, 140}, Cooldown: Scaled{25, 0},
		Reach: 400, Victims: 10, Damage: Scaled{200, 200}, Stuns: true},
	// Strong Body: 80 + 220k mana, 63 + 90k seconds' cooldown, a run of 60 + 90k; its
	// tooltip's 7 + (40 + 0.2·STR + 0.4·CON)·k to the defence, where the skilltable's
	// (200 + ...)·k would make a warrior all but unbitable on the first maps, and its
	// skilltable's 1 + 9k off the move speed.
	StrongBody: {Class: account.Warrior, Group: "mental", Mana: Scaled{80, 220}, Cooldown: Scaled{63, 90},
		Raises: map[Stat]Formula{
			Defence:   func(b Bearer, k int32) int32 { return 700 + (200+int32(b.Strength)+2*int32(b.Vitality))*k/5 },
			MoveSpeed: func(_ Bearer, k int32) int32 { return -(100 + 9*k) },
		},
		Lasts: Scaled{60, 90}},
	// Sword Strike: 40 + 120k, 20 s, landing on the monster picked within 1200, or else the
	// nearest in front of him, and up to 9 more within 200 of it, each perhaps stunned; its
	// knockback is left out.
	SwordStrike: {Class: account.Warrior, Group: "mental", Mana: Scaled{40, 120}, Cooldown: Scaled{20, 0},
		Aim: 1200, Reach: 200, Victims: 10, Damage: Scaled{200, 100}, Stuns: true, Wields: SwordOrTwoHanded},
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
