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
	Level       uint8
	MoveSpeed   uint8 // 100 is normal; the client's animation stands still at 0
	AttackSpeed uint8
}

// Wanders reports whether the mob strolls around where it was spawned.
func (p Proto) Wanders() bool { return p.Kind == Monster }

// Lookup returns the kind of mob vnum, or false if the world has none of it.
func Lookup(vnum uint16) (Proto, bool) {
	p, ok := protos[vnum]
	return p, ok
}

// protos are the client's mob_proto values (its --dump_proto--/mob_proto.txt): level and
// speeds. The names are the Polish client's own, from its locale mob_proto
// (docs/mechanics/monsters.md says how they were read), in UTF-8.
var protos = map[uint16]Proto{
	101: {VNum: 101, Name: "Dziki Pies", Kind: Monster, Level: 1, MoveSpeed: 100, AttackSpeed: 100},
	102: {VNum: 102, Name: "Wilk", Kind: Monster, Level: 3, MoveSpeed: 100, AttackSpeed: 100},
	103: {VNum: 103, Name: "Alfa Wilk", Kind: Monster, Level: 4, MoveSpeed: 130, AttackSpeed: 100},
	108: {VNum: 108, Name: "Dzik", Kind: Monster, Level: 7, MoveSpeed: 100, AttackSpeed: 80},

	9001: {VNum: 9001, Name: "Handlarz Bronią", Kind: NPC, Level: 70, MoveSpeed: 100, AttackSpeed: 100},
	9002: {VNum: 9002, Name: "Handlarz Zbrojami", Kind: NPC, Level: 70, MoveSpeed: 100, AttackSpeed: 100},
	9003: {VNum: 9003, Name: "Handlarka Różności", Kind: NPC, Level: 70, MoveSpeed: 100, AttackSpeed: 100},
	9005: {VNum: 9005, Name: "Dozorca", Kind: NPC, Level: 70, MoveSpeed: 100, AttackSpeed: 100},
	9006: {VNum: 9006, Name: "Starsza Pani", Kind: NPC, Level: 70, MoveSpeed: 100, AttackSpeed: 100},
	9009: {VNum: 9009, Name: "Rybak", Kind: NPC, Level: 70, MoveSpeed: 100, AttackSpeed: 100},
}
