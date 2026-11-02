package account

// What a new character starts with. The stats are the client's START_STAT table in
// introcreate.py, which is what the create screen shows; the positions are each kingdom's
// first village, inside that map's area in the client's atlasinfo.txt.

// Stats are a character's four base stats.
type Stats struct {
	Vitality     uint8 // the client's CON: hit points and defence
	Intelligence uint8 // INT: mana and magic
	Strength     uint8 // STR: physical attack
	Dexterity    uint8 // DEX: hit rate, evasion and bows
}

// StartingStats are the stats a new character of the class has, for either sex.
func (c Class) StartingStats() Stats {
	switch c {
	case Warrior:
		return Stats{Vitality: 4, Intelligence: 3, Strength: 6, Dexterity: 3}
	case Ninja:
		return Stats{Vitality: 3, Intelligence: 3, Strength: 4, Dexterity: 6}
	case Sura:
		return Stats{Vitality: 3, Intelligence: 5, Strength: 5, Dexterity: 3}
	default:
		return Stats{Vitality: 4, Intelligence: 6, Strength: 3, Dexterity: 3}
	}
}

// StartPosition is where the kingdom's new characters appear, in world coordinates.
func (e Empire) StartPosition() (x, y int32) {
	switch e {
	case EmpireShinsoo:
		return 475000, 966100
	case EmpireChunjo:
		return 60000, 156000
	default:
		return 963400, 278200
	}
}

// Name limits for characters, this server's own rule: the create screen takes at most 12
// characters, and the client refuses only an empty name, so the minimum of 2 and the ASCII
// letters and digits are ours, chosen so every name reads the same to every player.
const (
	MinCharacterName = 2
	MaxCharacterName = 12
)

// ValidCharacterName reports whether a new character may be called name.
func ValidCharacterName(name string) bool {
	if len(name) < MinCharacterName || len(name) > MaxCharacterName {
		return false
	}
	for _, r := range name {
		if !isASCIIAlphanumeric(r) {
			return false
		}
	}
	return true
}

func isASCIIAlphanumeric(r rune) bool {
	return 'a' <= r && r <= 'z' || 'A' <= r && r <= 'Z' || '0' <= r && r <= '9'
}
