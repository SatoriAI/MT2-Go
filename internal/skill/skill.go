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
// themselves, and its buffs of speed and attack; the warrior's Mental attacks, which
// stun, and its buff of defence; the ninja's Dagger attacks, from behind, at a dash, and
// poisoning; the sura's Black Magic, magic attacks, one slowing, and a shield; and the
// shaman's spells, burning, stunning and chaining, and her buffs and heal for herself or
// another.
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
	Ambush          account.Skill = "ambush"
	FastAttack      account.Skill = "fast_attack"
	RollingDagger   account.Skill = "rolling_dagger"
	DarkStrike      account.Skill = "dark_strike"
	FlameStrike     account.Skill = "flame_strike"
	DarkProtection  account.Skill = "dark_protection"
	GhostStrike     account.Skill = "ghost_strike"
	DarkOrb         account.Skill = "dark_orb"
	Blessing        account.Skill = "blessing"
	Reflection      account.Skill = "reflection"
	DragonsHelp     account.Skill = "dragons_help"
	Heal            account.Skill = "heal"
	Swiftness       account.Skill = "swiftness"
	AttackUp        account.Skill = "attack_up"
	FlyingTalisman  account.Skill = "flying_talisman"
	ShootingDragon  account.Skill = "shooting_dragon"
	DragonRoar      account.Skill = "dragon_roar"
	LightningThrow  account.Skill = "lightning_throw"
	SummonLightning account.Skill = "summon_lightning"
	LightningClaw   account.Skill = "lightning_claw"
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
	// once.
	Target  bool
	Aim     int64
	Reach   int64
	Front   bool
	Victims int
	Hits    int
	Damage  Scaled
	// What else it does: stun, poison, burn or slow each one it leaves alive, strike FromBehind
	// percent as hard one struck from behind, none for no more, and with Dash move the
	// player beside the one aimed at first. A magic attack's damage is Magic's instead.
	Stuns      int // the base of its chance to stun, in thousandths; none for no stun
	Poisons    bool
	Burns      bool
	Chain      int64 // how far a chain jumps from each victim to the next
	Slows      bool
	FromBehind int
	Dash       bool
	Magic      Spell
	Dice       [2]int32 // the least and most of the number a spell's tooltip rolls
	DiceByIQ   bool     // that those are multiples of the caster's INT
	// Drain is the mana a toggle takes every five seconds it runs, at its power; it ends
	// once there is not enough.
	Drain Scaled
	// Ally is that it lands on the player picked within Aim, else on the caster; Heal is
	// what it heals instead of a buff.
	Ally bool
	Heal Spell
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
	Attack       Stat = "attack"
	Defence      Stat = "defence"
	AttackSpeed  Stat = "attack_speed"
	MoveSpeed    Stat = "move_speed"
	DamageTaken  Stat = "damage_taken"  // a percent more of every bite
	ManaShield   Stat = "mana_shield"   // a percent of every bite taken from mana instead
	Resistance   Stat = "resistance"    // a percent less of every bite, after armour
	Reflect      Stat = "reflect"       // a percent of every bite taken back to the biter
	Critical     Stat = "critical"      // a percent chance of a blow striking twice as hard
	CastingSpeed Stat = "casting_speed" // more than the 100 at which cooldowns run as listed
)

// Spell is a magic attack's damage, as its tooltip works it out, for its caster at a
// power k in hundredths, before the monster's resistance, of which the first maps have
// none.
type Spell func(c Caster, k int32) int32

// DiceOf is the range a spell's number is rolled from for a caster of intelligence.
func (p Proto) DiceOf(intelligence uint8) [2]int32 {
	if p.DiceByIQ {
		return [2]int32{p.Dice[0] * int32(intelligence), p.Dice[1] * int32(intelligence)}
	}
	return p.Dice
}

// Caster is what of a character a spell reads: its MTK is the weapon's magic attack and
// Dice the spell's number, each rolled, and its AR its hit rate, in hundredths, as the
// client works it out.
type Caster struct {
	Level, Strength, Vitality, Dexterity, Intelligence int32
	MTK, Dice, AR                                      int32
}

// HitRate is a character's hit rate in hundredths, the client's (min(90, (4·DEX +
// 2·level)/6) + 210)/300, which its spells' damage is multiplied by.
func HitRate(dexterity, level uint8) int32 {
	return 100 * (min(90, (4*int32(dexterity)+2*int32(level))/6) + 210) / 300
}

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
	DaggerOrSword    Wield = "dagger_or_sword"     // the client's DAGGER|DOUBLE_SWORD|SWORD
)

// Wieldable is whether the skill may be used, or its buff run, with the weapon in hand.
func (p Proto) Wieldable(weapon item.Proto) bool {
	switch p.Wields {
	case Sword:
		return weapon.Sword
	case SwordOrTwoHanded:
		return weapon.Sword || weapon.TwoHanded
	case DaggerOrSword:
		return weapon.Dagger || weapon.Sword
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
// it leaves alive: the client's (base + 1000k/6)/10 percent, its base Stuns.
func (p Proto) StunChance(level uint8) int { return p.Stuns + 10*int(Power(level))/6 }

// PoisonChance is the chance, in thousandths, a poisoning attack at level poisons each
// monster it leaves alive: its tooltip's 1 + 4k percent.
func PoisonChance(level uint8) int { return (100 + 4*int(Power(level))) / 10 }

// SlowChance is the chance, in thousandths, a slowing attack at level slows each monster
// it leaves alive: the client's (333 + 300k)/10 percent.
func SlowChance(level uint8) int { return 333 + 3*int(Power(level)) }

// BurnChance is the chance, in thousandths, a burning attack at level sets each monster it
// leaves alive alight, for a caster of intelligence: the skilltable's 0.2·iq·k percent.
func BurnChance(intelligence, level uint8) int {
	return 2 * int(intelligence) * int(Power(level)) / 100
}

// BurnDamage is what a burn at level takes in all, for a caster of level and
// intelligence: the skilltable's lv + 5·iq·k, the veteran's reading.
func BurnDamage(casterLevel, intelligence, level uint8) int {
	return int(casterLevel) + 5*int(intelligence)*int(Power(level))/100
}

// SlowsFor is how long a slow at level holds a monster: the skilltable's 10 + 10k seconds.
func SlowsFor(level uint8) time.Duration {
	return time.Duration(1000+10*Power(level)) * 10 * time.Millisecond
}

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
		Reach: 400, Victims: 10, Damage: Scaled{200, 200}, Stuns: 100},
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
		Aim: 1200, Reach: 200, Victims: 10, Damage: Scaled{200, 100}, Stuns: 100, Wields: SwordOrTwoHanded},
	// Dagger, settled by the veteran and the designer: the client's skilltable for mana
	// and cooldowns, the veteran's damage, which keeps Ambush the hardest blow, and its
	// weapons, a dagger or a sword. Ambush: 40 + 160k, 15 s, half as strong again from
	// behind the monster.
	Ambush: {Class: account.Ninja, Group: "dagger", Mana: Scaled{40, 160}, Cooldown: Scaled{15, 0},
		Target: true, Damage: Scaled{150, 350}, FromBehind: 150, Wields: DaggerOrSword},
	// Fast Attack: 40 + 160k, 20 s, on the monster picked within 800, with the ninja moved
	// beside it.
	FastAttack: {Class: account.Ninja, Group: "dagger", Mana: Scaled{40, 160}, Cooldown: Scaled{20, 0},
		Target: true, Aim: 800, Dash: true, Damage: Scaled{100, 250}, Wields: DaggerOrSword},
	// Rolling Dagger: 50 + 140k, 25 s, up to 12 within 250 around her, each perhaps
	// poisoned.
	RollingDagger: {Class: account.Ninja, Group: "dagger", Mana: Scaled{50, 140}, Cooldown: Scaled{25, 0},
		Reach: 250, Victims: 12, Damage: Scaled{150, 150}, Poisons: true, Wields: DaggerOrSword},
	// Black Magic, settled by the veteran and the designer: the client's skilltable for
	// mana, cooldowns, victims, range and splash, and its tooltips' damage, as numbers of
	// INT, the weapon's magic attack and the hit rate, which no armour cuts. Dark Strike:
	// 30 + 140k, 7 s, on the monster picked within 1500 and up to 4 within 200 of it.
	DarkStrike: {Class: account.Sura, Group: "black_magic", Mana: Scaled{30, 140}, Cooldown: Scaled{7, 0},
		Target: true, Aim: 1500, Reach: 200, Victims: 5, Dice: [2]int32{50, 100},
		Magic: func(c Caster, k int32) int32 {
			return 40 + 5*c.Level + 2*c.Intelligence + (20*c.Intelligence+7*c.MTK+c.Dice)*c.AR*k/10000
		}},
	// Flame Strike: 60 + 140k, 12 s, up to 15 within 500 around the sura; its tooltip has
	// no hit rate.
	FlameStrike: {Class: account.Sura, Group: "black_magic", Mana: Scaled{60, 140}, Cooldown: Scaled{12, 0},
		Reach: 500, Victims: 15, Dice: [2]int32{180, 200},
		Magic: func(c Caster, k int32) int32 {
			return 5*c.Level + 2*c.Intelligence + (20*c.Intelligence+8*c.MTK+4*c.Strength+2*c.Vitality+c.Dice)*k/100
		}},
	// Dark Protection: 20 + 30k, a toggle cooling down 63 + 120k seconds and running
	// 60 + 120k, draining 5 + 10k mana every five seconds; (0.5·INT + 15)·k more defence,
	// and 0.84·INT·k percent of every bite taken from mana, the veteran's reading.
	DarkProtection: {Class: account.Sura, Group: "black_magic", Mana: Scaled{20, 30}, Cooldown: Scaled{63, 120},
		Raises: map[Stat]Formula{
			Defence:    func(b Bearer, k int32) int32 { return (int32(b.Intelligence) + 30) * k / 2 },
			ManaShield: func(b Bearer, k int32) int32 { return 84 * int32(b.Intelligence) * k / 100 },
		},
		Lasts: Scaled{60, 120}, Toggle: true, Drain: Scaled{5, 10}},
	// Ghost Strike: 40 + 120k, 12 s, on the monster picked within 1200 and up to 8 within
	// 400 of it, each perhaps slowed.
	GhostStrike: {Class: account.Sura, Group: "black_magic", Mana: Scaled{40, 120}, Cooldown: Scaled{12, 0},
		Target: true, Aim: 1200, Reach: 400, Victims: 9, Slows: true, Dice: [2]int32{180, 200},
		Magic: func(c Caster, k int32) int32 {
			return 40 + 2*c.Level + 2*c.Intelligence + (2*c.Vitality+2*c.Dexterity+22*c.Intelligence+6*c.MTK+c.Dice)*c.AR*k/10000
		}},
	// Dark Orb: 80 + 220k, 24 s, on the monster picked within 1500 and up to 8 within 200
	// of it.
	DarkOrb: {Class: account.Sura, Group: "black_magic", Mana: Scaled{80, 220}, Cooldown: Scaled{24, 0},
		Target: true, Aim: 1500, Reach: 200, Victims: 9,
		Magic: func(c Caster, k int32) int32 {
			return 120 + 6*c.Level + (5*c.Vitality+5*c.Dexterity+40*c.Intelligence+12*c.MTK)*c.AR*k/10000
		}},
	// The shaman's buffs and heal, settled by the veteran and the designer: the client's
	// skilltable for mana, cooldowns, runs and range, its formulas of the caster's INT,
	// each landing on a player picked within range, else on her. Blessing: 40 + 160k, a
	// run of 60 + 200k; (0.3·iq + 5)(2k + 0.5)/(k + 1.5) percent less of every bite.
	Blessing: {Class: account.Shaman, Group: "dragon", Mana: Scaled{40, 160}, Cooldown: Scaled{10, 0}, Aim: 800, Ally: true,
		Raises: map[Stat]Formula{Resistance: func(b Bearer, k int32) int32 { return (30*int32(b.Intelligence) + 500) * (2*k + 50) / (k + 150) }},
		Lasts:  Scaled{60, 200}},
	// Reflection: 40 + 160k, a run of 60 + 200k; 5 + (0.3·iq + 5)·k percent of every bite
	// back to the biter.
	Reflection: {Class: account.Shaman, Group: "dragon", Mana: Scaled{40, 160}, Cooldown: Scaled{10, 0}, Aim: 1000, Ally: true,
		Raises: map[Stat]Formula{Reflect: func(b Bearer, k int32) int32 { return 500 + (30*int32(b.Intelligence)+500)*k/100 }},
		Lasts:  Scaled{60, 200}},
	// Dragon's Help: 40 + 160k, a run of 60 + 100k; (0.3·iq + 5)(2k + 0.5)/(k + 1.5)
	// percent chance of a blow striking twice as hard.
	DragonsHelp: {Class: account.Shaman, Group: "dragon", Mana: Scaled{40, 160}, Cooldown: Scaled{10, 0}, Aim: 1000, Ally: true,
		Raises: map[Stat]Formula{Critical: func(b Bearer, k int32) int32 { return (30*int32(b.Intelligence) + 500) * (2*k + 50) / (k + 150) }},
		Lasts:  Scaled{60, 100}},
	// Heal: 40 + 200k; 200 + 4·lv + (10·iq + 6·MTK + 600 to 800)·k hit points, no hit rate.
	Heal: {Class: account.Shaman, Group: "healing", Mana: Scaled{40, 200}, Cooldown: Scaled{10, 0}, Aim: 1000, Ally: true,
		Dice: [2]int32{600, 800},
		Heal: func(c Caster, k int32) int32 { return 200 + 4*c.Level + (10*c.Intelligence+6*c.MTK+c.Dice)*k/100 }},
	// Swiftness: 60 + 120k, a run of 60 + 100k; 5 + 35k on the move speed, and 3 + 33k on
	// the casting speed.
	Swiftness: {Class: account.Shaman, Group: "healing", Mana: Scaled{60, 120}, Cooldown: Scaled{10, 0}, Aim: 1000, Ally: true,
		Raises: map[Stat]Formula{
			MoveSpeed:    func(_ Bearer, k int32) int32 { return 500 + 35*k },
			CastingSpeed: func(_ Bearer, k int32) int32 { return 300 + 33*k },
		},
		Lasts: Scaled{60, 100}},
	// Attack Up: 60 + 120k, a run of 60 + 100k; 5 + (0.2·iq + 15)·k on the attack.
	AttackUp: {Class: account.Shaman, Group: "healing", Mana: Scaled{60, 120}, Cooldown: Scaled{10, 0}, Aim: 1000, Ally: true,
		Raises: map[Stat]Formula{Attack: func(b Bearer, k int32) int32 { return 500 + (20*int32(b.Intelligence)+1500)*k/100 }},
		Lasts:  Scaled{60, 100}},
	// The shaman's attacks, settled by the veteran and the designer: the client's
	// skilltable for mana, cooldowns, victims, range and splash, and its tooltips'
	// damage, as Black Magic's. Flying Talisman: 30 + 160k, 7 s, on the monster picked
	// within 1800 and up to 4 within 201 of it.
	FlyingTalisman: {Class: account.Shaman, Group: "dragon", Mana: Scaled{30, 160}, Cooldown: Scaled{7, 0},
		Target: true, Aim: 1800, Reach: 201, Victims: 5,
		Magic: func(c Caster, k int32) int32 { return 70 + 4*c.Level + (20*c.Intelligence+5*c.MTK+50)*c.AR*k/10000 }},
	// Shooting Dragon: 50 + 160k, 8 s, up to 10, each perhaps set alight. The skilltable
	// gives no range or splash for its frontal spray: this server's 1000 to the monster
	// picked and 300 around it.
	ShootingDragon: {Class: account.Shaman, Group: "dragon", Mana: Scaled{50, 160}, Cooldown: Scaled{8, 0},
		Target: true, Aim: 1000, Reach: 300, Victims: 10, Burns: true,
		Magic: func(c Caster, k int32) int32 { return 60 + 5*c.Level + (18*c.Intelligence+6*c.MTK+120)*c.AR*k/10000 }},
	// Dragon Roar: 50 + 160k, 20 s, up to 15 within 500 around her, each perhaps set alight.
	DragonRoar: {Class: account.Shaman, Group: "dragon", Mana: Scaled{50, 160}, Cooldown: Scaled{20, 0},
		Reach: 500, Victims: 15, Burns: true,
		Magic: func(c Caster, k int32) int32 { return 70 + 3*c.Level + (22*c.Intelligence+13*c.MTK+100)*c.AR*k/10000 }},
	// Lightning Throw: 30 + 150k, 7 s, on the monster picked within 1800 and up to 4 within
	// 200 of it; its number is 5 to 15 times her INT.
	LightningThrow: {Class: account.Shaman, Group: "healing", Mana: Scaled{30, 150}, Cooldown: Scaled{7, 0},
		Target: true, Aim: 1800, Reach: 200, Victims: 5, Dice: [2]int32{5, 15}, DiceByIQ: true,
		Magic: func(c Caster, k int32) int32 {
			return 60 + 4*c.Level + (7*c.Intelligence+8*c.MTK+c.Dice)*c.AR*k/10000
		}},
	// Summon Lightning: 50 + 150k, 15 s, on the monster picked within 1500 and up to 14
	// within 400 of it, each perhaps stunned at (50 + 1000k/6)/10 percent; its number is 5
	// to 16 times her INT.
	SummonLightning: {Class: account.Shaman, Group: "healing", Mana: Scaled{50, 150}, Cooldown: Scaled{15, 0},
		Target: true, Aim: 1500, Reach: 400, Victims: 15, Stuns: 50, Dice: [2]int32{5, 16}, DiceByIQ: true,
		Magic: func(c Caster, k int32) int32 {
			return 40 + 4*c.Level + (13*c.Intelligence+7*c.MTK+c.Dice)*c.AR*k/10000
		}},
	// Lightning Claw: 40 + 180k, 10 s, on the monster picked within 2500, then jumping from
	// each to the nearest not yet struck within 600, the veteran's reach, seven in all,
	// each jump 13 in a hundred weaker; its number is 1 to 800.
	LightningClaw: {Class: account.Shaman, Group: "healing", Mana: Scaled{40, 180}, Cooldown: Scaled{10, 0},
		Target: true, Aim: 2500, Chain: 600, Victims: 7, Dice: [2]int32{1, 800},
		Magic: func(c Caster, k int32) int32 { return 50 + 5*c.Level + (6*c.Intelligence+6*c.MTK+c.Dice)*c.AR*k/10000 }},
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
