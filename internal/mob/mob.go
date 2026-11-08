// Package mob is what each kind of monster and NPC is: the values the game needs to put
// one in the world. Only the kinds the world spawns are listed.
package mob

import (
	"time"

	"github.com/SatoriAI/MT2-Go/internal/account"
)

// Kind is what a mob is to the player.
type Kind string

const (
	Monster Kind = "monster"
	NPC     Kind = "npc" // a villager the player can talk to; it never moves
	// Stone is a Metin stone: fought, but never moving or biting, it calls monsters out
	// as it is worn down.
	Stone Kind = "stone"
)

// Proto is one kind of mob, as the client's mob_proto describes it.
type Proto struct {
	VNum        uint16 // the number the client knows the kind by, for its model and name
	Name        string
	Kind        Kind
	MoveSpeed   uint8 // 100 is normal; the client's animation stands still at 0
	AttackSpeed uint8

	// What a monster fights with; NPCs cannot be fought.
	Level       uint8
	MaxHP       uint32
	Vitality    uint8  // HT: defence
	Armour      uint8  // DEF
	Exp         uint32 // shared among those who killed it, by the damage each dealt
	Strength    uint8  // ST: part of its bite
	DamageMin   uint16 // the dice its bites roll
	DamageMax   uint16
	AttackRange uint16 // how far its bite reaches
	// It heals RegenPercent of its hit points every RegenCycle seconds while hurt.
	RegenCycle   uint8
	RegenPercent uint8
	Drops        []Drop // what it may leave when it dies, each rolled on its own
	Picks        []Pick // and these, each one kind picked from several
	Yang         Yang   // and the yang, rolled once
	Sells        []Ware // what an NPC's shop holds, for a shopkeeper
	// Teaches is the skill group a class teacher takes a character on in, or "".
	Teaches account.SkillGroup
	// Upgrades is that the NPC is a blacksmith, who upgrades gear given to them.
	Upgrades bool

	// A stone's own: the monsters each of its waves calls out, how long after it is
	// destroyed it comes back, and how long it must go unhit before it heals.
	Wave       []uint16
	Respawn    time.Duration
	HealsAfter time.Duration
}

// Ware is what one cell of a shop holds: Count of item VNum. Each cell is written out, so
// those a tall item covers below it are left empty on purpose, as the client draws it
// over them.
type Ware struct {
	VNum  uint32
	Count uint8
	Cell  uint8
}

// Yang is the yang a monster may leave when it dies: with a chance of Chance in 10,000, an
// amount from Min to Max, before an event multiplies it.
type Yang struct {
	Chance   uint16
	Min, Max uint32
}

// Drop is an item a monster may leave when it dies: Count of item VNum, with a chance of
// Chance in 10,000.
type Drop struct {
	VNum   uint32
	Count  uint8
	Chance uint16
}

// Pick is Count of one kind of item, picked evenly from From, that a monster may leave
// when it dies, with a chance of Chance in 10,000.
type Pick struct {
	From   []uint32
	Count  uint8
	Chance uint16
}

// The village shops: the weapon trader's first weapons and the armour trader's first
// armours along the top row, the cells below left to the taller ones, the helmets and
// shoes on the row under the armours and the shield below them, and the general
// store's potions, singly on the top row and twenty to a pack on the next.
var (
	weapons = []Ware{{VNum: 10, Count: 1, Cell: 0}, {VNum: 1000, Count: 1, Cell: 1}, {VNum: 2000, Count: 1, Cell: 2}, {VNum: 3000, Count: 1, Cell: 3}, {VNum: 7000, Count: 1, Cell: 4}}
	armours = []Ware{{VNum: 11200, Count: 1, Cell: 0}, {VNum: 11400, Count: 1, Cell: 1}, {VNum: 11600, Count: 1, Cell: 2}, {VNum: 11800, Count: 1, Cell: 3},
		{VNum: 12200, Count: 1, Cell: 10}, {VNum: 12340, Count: 1, Cell: 11}, {VNum: 12480, Count: 1, Cell: 12}, {VNum: 12620, Count: 1, Cell: 13},
		{VNum: 15000, Count: 1, Cell: 14}, {VNum: 13000, Count: 1, Cell: 15}}
	sundries = []Ware{{VNum: 27001, Count: 1, Cell: 0}, {VNum: 27002, Count: 1, Cell: 1}, {VNum: 27004, Count: 1, Cell: 2},
		{VNum: 27001, Count: 20, Cell: 5}, {VNum: 27002, Count: 20, Cell: 6}, {VNum: 27004, Count: 20, Cell: 7}}
)

// potions is what every monster of the first villages may leave: a red and a blue potion,
// each 8 times in 100.
var potions = []Drop{{VNum: 27001, Count: 1, Chance: 800}, {VNum: 27004, Count: 1, Chance: 800}}

// gear is what the stronger ones may also leave: each class's first weapon and armour, each
// 2 times in 1,000, so about 2 in 100 for any one of them.
var gear = append(append([]Drop{}, potions...),
	Drop{VNum: 10, Count: 1, Chance: 20}, Drop{VNum: 1000, Count: 1, Chance: 20},
	Drop{VNum: 2000, Count: 1, Chance: 20}, Drop{VNum: 3000, Count: 1, Chance: 20},
	Drop{VNum: 7000, Count: 1, Chance: 20}, Drop{VNum: 11200, Count: 1, Chance: 20},
	Drop{VNum: 11400, Count: 1, Chance: 20}, Drop{VNum: 11600, Count: 1, Chance: 20},
	Drop{VNum: 11800, Count: 1, Chance: 20})

// The materials upgrades take from +5 on: wolves leave a claw, for weapons, and boars a
// tooth, for armour, each 5 times in 100. Bears and tigers leave the first gear.
var (
	wolfDrops      = append(append([]Drop{}, potions...), Drop{VNum: 30028, Count: 1, Chance: 500})
	alphaWolfDrops = append(append([]Drop{}, gear...), Drop{VNum: 30028, Count: 1, Chance: 500})
	boarDrops      = append(append([]Drop{}, gear...), Drop{VNum: 30004, Count: 1, Chance: 500})
)

// firstGear is each class's first weapon and armour, which the stones leave one of, and
// the same at +3, which the last of them may.
var (
	firstGear   = []uint32{10, 1000, 2000, 3000, 7000, 11200, 11400, 11600, 11800}
	firstGearP3 = []uint32{13, 1003, 2003, 3003, 7003, 11203, 11403, 11603, 11803}
	potionKinds = []uint32{27001, 27004}
	materials   = []uint32{30028, 30004}
)

// stonePicks is what a stone always leaves, potions and materials, each count of one
// kind, and with a chance in 10,000 one piece of the first gear.
func stonePicks(potions, materialCount uint8, gear uint16) []Pick {
	return []Pick{{From: potionKinds, Count: potions, Chance: 10000}, {From: materials, Count: materialCount, Chance: 10000},
		{From: firstGear, Count: 1, Chance: gear}}
}

// Fightable reports whether a player can attack the mob.
func (p Proto) Fightable() bool { return p.Kind != NPC }

// Wanders reports whether the mob strolls around where it was spawned.
func (p Proto) Wanders() bool { return p.Kind == Monster }

// Lookup returns the kind of mob vnum, or false if the world has none of it.
func Lookup(vnum uint16) (Proto, bool) {
	p, ok := protos[vnum]
	return p, ok
}

// protos are the client's mob_proto values (its --dump_proto--/mob_proto.txt): speeds,
// and a monster's level, hit points, HT, DEF, ST, damage, attack range and healing. A
// stone's hit points, experience and drops are this server's (docs/mechanics/stones.md).
// The names are the Polish client's own, from its locale mob_proto, in UTF-8.
var protos = map[uint16]Proto{
	101: {VNum: 101, Name: "Dziki Pies", Kind: Monster, MoveSpeed: 100, AttackSpeed: 100,
		Level: 1, MaxHP: 126, Vitality: 5, Armour: 4, Exp: 15,
		Strength: 3, DamageMin: 20, DamageMax: 24, AttackRange: 175,
		RegenCycle: 6, RegenPercent: 7, Drops: potions, Yang: Yang{Chance: 6000, Min: 5, Max: 12}},
	102: {VNum: 102, Name: "Wilk", Kind: Monster, MoveSpeed: 100, AttackSpeed: 100,
		Level: 3, MaxHP: 162, Vitality: 7, Armour: 6, Exp: 39,
		Strength: 4, DamageMin: 23, DamageMax: 28, AttackRange: 175,
		RegenCycle: 6, RegenPercent: 7, Drops: wolfDrops, Yang: Yang{Chance: 6000, Min: 8, Max: 18}},
	103: {VNum: 103, Name: "Alfa Wilk", Kind: Monster, MoveSpeed: 130, AttackSpeed: 100,
		Level: 4, MaxHP: 182, Vitality: 10, Armour: 7, Exp: 51,
		Strength: 8, DamageMin: 25, DamageMax: 30, AttackRange: 175,
		RegenCycle: 6, RegenPercent: 7, Drops: alphaWolfDrops, Yang: Yang{Chance: 6000, Min: 10, Max: 25}},
	108: {VNum: 108, Name: "Dzik", Kind: Monster, MoveSpeed: 100, AttackSpeed: 80,
		Level: 7, MaxHP: 248, Vitality: 10, Armour: 11, Exp: 87,
		Strength: 14, DamageMin: 30, DamageMax: 37, AttackRange: 175,
		RegenCycle: 6, RegenPercent: 7, Drops: boarDrops, Yang: Yang{Chance: 6000, Min: 20, Max: 40}},
	104: {VNum: 104, Name: "Nieb. Wilk", Kind: Monster, MoveSpeed: 100, AttackSpeed: 100,
		Level: 6, MaxHP: 225, Vitality: 6, Armour: 9, Exp: 75,
		Strength: 9, DamageMin: 25, DamageMax: 38, AttackRange: 175,
		RegenCycle: 6, RegenPercent: 7, Drops: wolfDrops, Yang: Yang{Chance: 6000, Min: 15, Max: 30}},
	105: {VNum: 105, Name: "Nieb. Alfa Wilk", Kind: Monster, MoveSpeed: 100, AttackSpeed: 80,
		Level: 9, MaxHP: 297, Vitality: 16, Armour: 13, Exp: 111,
		Strength: 14, DamageMin: 33, DamageMax: 41, AttackRange: 175,
		RegenCycle: 6, RegenPercent: 7, Drops: alphaWolfDrops, Yang: Yang{Chance: 6000, Min: 20, Max: 40}},
	109: {VNum: 109, Name: "Czerw. Dzik", Kind: Monster, MoveSpeed: 100, AttackSpeed: 80,
		Level: 10, MaxHP: 389, Vitality: 13, Armour: 14, Exp: 148,
		Strength: 22, DamageMin: 35, DamageMax: 43, AttackRange: 175,
		RegenCycle: 3, RegenPercent: 7, Drops: boarDrops, Yang: Yang{Chance: 6000, Min: 30, Max: 55}},
	110: {VNum: 110, Name: "Niedźwiedź", Kind: Monster, MoveSpeed: 70, AttackSpeed: 100,
		Level: 12, MaxHP: 381, Vitality: 20, Armour: 16, Exp: 147,
		Strength: 20, DamageMin: 39, DamageMax: 47, AttackRange: 175,
		RegenCycle: 6, RegenPercent: 7, Drops: gear, Yang: Yang{Chance: 6000, Min: 25, Max: 45}},
	111: {VNum: 111, Name: "Niedźw. Grizzly", Kind: Monster, MoveSpeed: 100, AttackSpeed: 100,
		Level: 15, MaxHP: 572, Vitality: 34, Armour: 19, Exp: 220,
		Strength: 14, DamageMin: 44, DamageMax: 53, AttackRange: 175,
		RegenCycle: 6, RegenPercent: 7, Drops: gear, Yang: Yang{Chance: 6000, Min: 40, Max: 70}},
	114: {VNum: 114, Name: "Tygrys", Kind: Monster, MoveSpeed: 100, AttackSpeed: 100,
		Level: 18, MaxHP: 585, Vitality: 27, Armour: 22, Exp: 365,
		Strength: 18, DamageMin: 49, DamageMax: 60, AttackRange: 175,
		RegenCycle: 6, RegenPercent: 12, Drops: gear, Yang: Yang{Chance: 6000, Min: 35, Max: 60}},

	8001: {VNum: 8001, Name: "Metin Cierpienia", Kind: Stone, MoveSpeed: 109, AttackSpeed: 104,
		Level: 5, MaxHP: 7000, Vitality: 6, Armour: 8, Exp: 900, RegenCycle: 5, RegenPercent: 1,
		Picks: stonePicks(3, 2, 1500), Yang: Yang{Chance: 10000, Min: 500, Max: 800},
		Wave: []uint16{102, 102, 102, 103, 103}, Respawn: 10 * time.Minute, HealsAfter: 10 * time.Second},
	8002: {VNum: 8002, Name: "Metin Walki", Kind: Stone, MoveSpeed: 114, AttackSpeed: 110,
		Level: 10, MaxHP: 11500, Vitality: 12, Armour: 14, Exp: 2200, RegenCycle: 5, RegenPercent: 1,
		Picks: stonePicks(4, 3, 2500), Yang: Yang{Chance: 10000, Min: 900, Max: 1400},
		Wave: []uint16{104, 104, 104, 105, 105}, Respawn: 10 * time.Minute, HealsAfter: 10 * time.Second},
	8003: {VNum: 8003, Name: "Metin Bitwy", Kind: Stone, MoveSpeed: 121, AttackSpeed: 115,
		Level: 15, MaxHP: 17000, Vitality: 18, Armour: 19, Exp: 3300, RegenCycle: 5, RegenPercent: 1,
		Picks: append(stonePicks(5, 4, 3500), Pick{From: firstGearP3, Count: 1, Chance: 500}), Yang: Yang{Chance: 10000, Min: 1500, Max: 2200},
		Wave: []uint16{109, 109, 110, 110, 111}, Respawn: 10 * time.Minute, HealsAfter: 10 * time.Second},

	9001:  {VNum: 9001, Name: "Handlarz Bronią", Kind: NPC, MoveSpeed: 100, AttackSpeed: 100, Sells: weapons},
	9002:  {VNum: 9002, Name: "Handlarz Zbrojami", Kind: NPC, MoveSpeed: 100, AttackSpeed: 100, Sells: armours},
	9003:  {VNum: 9003, Name: "Handlarka Różności", Kind: NPC, MoveSpeed: 100, AttackSpeed: 100, Sells: sundries},
	20304: {VNum: 20304, Name: "Trener Mocy Broni", Kind: NPC, MoveSpeed: 100, AttackSpeed: 100, Teaches: "weaponry"},
	20305: {VNum: 20305, Name: "Trener Mocy Czarn. Magii", Kind: NPC, MoveSpeed: 100, AttackSpeed: 100, Teaches: "black_magic"},
	20324: {VNum: 20324, Name: "Trener Mocy Broni", Kind: NPC, MoveSpeed: 100, AttackSpeed: 100, Teaches: "weaponry"},
	20325: {VNum: 20325, Name: "Trener Mocy Czarn. Magii", Kind: NPC, MoveSpeed: 100, AttackSpeed: 100, Teaches: "black_magic"},
	20344: {VNum: 20344, Name: "Trener Mocy Broni", Kind: NPC, MoveSpeed: 100, AttackSpeed: 100, Teaches: "weaponry"},
	20345: {VNum: 20345, Name: "Trener Mocy Czarn. Magii", Kind: NPC, MoveSpeed: 100, AttackSpeed: 100, Teaches: "black_magic"},
	20300: {VNum: 20300, Name: "Trener Mocy Ciała", Kind: NPC, MoveSpeed: 100, AttackSpeed: 100, Teaches: "body"},
	20301: {VNum: 20301, Name: "Trener Mocy Duchowej", Kind: NPC, MoveSpeed: 100, AttackSpeed: 100, Teaches: "mental"},
	20320: {VNum: 20320, Name: "Trener Mocy Ciała", Kind: NPC, MoveSpeed: 100, AttackSpeed: 100, Teaches: "body"},
	20321: {VNum: 20321, Name: "Trener Mocy Duchowej", Kind: NPC, MoveSpeed: 100, AttackSpeed: 100, Teaches: "mental"},
	20340: {VNum: 20340, Name: "Trener Mocy Ciała", Kind: NPC, MoveSpeed: 100, AttackSpeed: 100, Teaches: "body"},
	20341: {VNum: 20341, Name: "Trener Mocy Duchowej", Kind: NPC, MoveSpeed: 100, AttackSpeed: 100, Teaches: "mental"},
	20302: {VNum: 20302, Name: "Trener Mocy Ostrza", Kind: NPC, MoveSpeed: 100, AttackSpeed: 100, Teaches: "dagger"},
	20303: {VNum: 20303, Name: "Trener Mocy Łuku", Kind: NPC, MoveSpeed: 100, AttackSpeed: 100, Teaches: "archery"},
	20322: {VNum: 20322, Name: "Trener Mocy Ostrza", Kind: NPC, MoveSpeed: 100, AttackSpeed: 100, Teaches: "dagger"},
	20323: {VNum: 20323, Name: "Trener Mocy Łuku", Kind: NPC, MoveSpeed: 100, AttackSpeed: 100, Teaches: "archery"},
	20342: {VNum: 20342, Name: "Trener Mocy Ostrza", Kind: NPC, MoveSpeed: 100, AttackSpeed: 100, Teaches: "dagger"},
	20343: {VNum: 20343, Name: "Trener Mocy Łuku", Kind: NPC, MoveSpeed: 100, AttackSpeed: 100, Teaches: "archery"},
	20016: {VNum: 20016, Name: "Kowal", Kind: NPC, MoveSpeed: 100, AttackSpeed: 100, Upgrades: true},
	9005:  {VNum: 9005, Name: "Dozorca", Kind: NPC, MoveSpeed: 100, AttackSpeed: 100},
	9006:  {VNum: 9006, Name: "Starsza Pani", Kind: NPC, MoveSpeed: 100, AttackSpeed: 100},
	9009:  {VNum: 9009, Name: "Rybak", Kind: NPC, MoveSpeed: 100, AttackSpeed: 100},
}
