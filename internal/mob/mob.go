// Package mob is what each kind of monster and NPC is: the values the game needs to put
// one in the world. Only the kinds the world spawns are listed.
package mob

// Kind is what a mob is to the player.
type Kind string

const (
	Monster Kind = "monster"
	NPC     Kind = "npc" // a villager the player can talk to; it never moves
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
}

// Drop is an item a monster may leave when it dies: Count of item VNum, with a chance of
// Chance in 10,000.
type Drop struct {
	VNum   uint32
	Count  uint8
	Chance uint16
}

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

// Fightable reports whether a player can attack the mob.
func (p Proto) Fightable() bool { return p.Kind == Monster }

// Wanders reports whether the mob strolls around where it was spawned.
func (p Proto) Wanders() bool { return p.Kind == Monster }

// Lookup returns the kind of mob vnum, or false if the world has none of it.
func Lookup(vnum uint16) (Proto, bool) {
	p, ok := protos[vnum]
	return p, ok
}

// protos are the client's mob_proto values (its --dump_proto--/mob_proto.txt): speeds,
// and a monster's level, hit points, HT, DEF, ST, damage, attack range and healing. The
// names are the Polish client's own, from its locale mob_proto (docs/mechanics/monsters.md
// says how they were read), in UTF-8.
var protos = map[uint16]Proto{
	101: {VNum: 101, Name: "Dziki Pies", Kind: Monster, MoveSpeed: 100, AttackSpeed: 100,
		Level: 1, MaxHP: 126, Vitality: 5, Armour: 4, Exp: 15,
		Strength: 3, DamageMin: 20, DamageMax: 24, AttackRange: 175,
		RegenCycle: 6, RegenPercent: 7, Drops: potions},
	102: {VNum: 102, Name: "Wilk", Kind: Monster, MoveSpeed: 100, AttackSpeed: 100,
		Level: 3, MaxHP: 162, Vitality: 7, Armour: 6, Exp: 39,
		Strength: 4, DamageMin: 23, DamageMax: 28, AttackRange: 175,
		RegenCycle: 6, RegenPercent: 7, Drops: potions},
	103: {VNum: 103, Name: "Alfa Wilk", Kind: Monster, MoveSpeed: 130, AttackSpeed: 100,
		Level: 4, MaxHP: 182, Vitality: 10, Armour: 7, Exp: 51,
		Strength: 8, DamageMin: 25, DamageMax: 30, AttackRange: 175,
		RegenCycle: 6, RegenPercent: 7, Drops: gear},
	108: {VNum: 108, Name: "Dzik", Kind: Monster, MoveSpeed: 100, AttackSpeed: 80,
		Level: 7, MaxHP: 248, Vitality: 10, Armour: 11, Exp: 87,
		Strength: 14, DamageMin: 30, DamageMax: 37, AttackRange: 175,
		RegenCycle: 6, RegenPercent: 7, Drops: gear},

	9001: {VNum: 9001, Name: "Handlarz Bronią", Kind: NPC, MoveSpeed: 100, AttackSpeed: 100},
	9002: {VNum: 9002, Name: "Handlarz Zbrojami", Kind: NPC, MoveSpeed: 100, AttackSpeed: 100},
	9003: {VNum: 9003, Name: "Handlarka Różności", Kind: NPC, MoveSpeed: 100, AttackSpeed: 100},
	9005: {VNum: 9005, Name: "Dozorca", Kind: NPC, MoveSpeed: 100, AttackSpeed: 100},
	9006: {VNum: 9006, Name: "Starsza Pani", Kind: NPC, MoveSpeed: 100, AttackSpeed: 100},
	9009: {VNum: 9009, Name: "Rybak", Kind: NPC, MoveSpeed: 100, AttackSpeed: 100},
}
