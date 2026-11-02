package game

import (
	"github.com/SatoriAI/MT2-Go/internal/account"
	"github.com/SatoriAI/MT2-Go/internal/protocol"
)

// The client numbers what the game names. These are the only places the numbers appear.

var empireNumbers = map[account.Empire]uint8{
	account.EmpireShinsoo: 1,
	account.EmpireChunjo:  2,
	account.EmpireJinno:   3,
}

// empireFromNumber returns the kingdom the client's number stands for, or false.
func empireFromNumber(n uint8) (account.Empire, bool) {
	for e, number := range empireNumbers {
		if number == n {
			return e, true
		}
	}
	return account.EmpireNone, false
}

var classNumbers = map[account.Class]uint8{
	account.Warrior: 0,
	account.Ninja:   1,
	account.Sura:    2,
	account.Shaman:  3,
}

// job is the client's single number for class and sex: 0 to 3 are the male warrior,
// ninja, sura and shaman, 4 to 7 the female ones.
func job(c account.Class, s account.Sex) uint8 {
	if s == account.Female {
		return classNumbers[c] + 4
	}
	return classNumbers[c]
}

// skillGroupNumber is 1 or 2 for the class's first or second path, 0 for none chosen.
func skillGroupNumber(c account.Class, g account.SkillGroup) uint8 {
	for i, group := range c.SkillGroups() {
		if group == g {
			return uint8(i + 1)
		}
	}
	return 0
}

// slots lays the characters out as the selection screen shows them. The body part stays
// 0, the bare body, until characters wear armour.
func slots(characters []account.Character) [protocol.CharacterSlots]protocol.CharacterSummary {
	var out [protocol.CharacterSlots]protocol.CharacterSummary
	for _, c := range characters {
		out[c.Slot] = protocol.CharacterSummary{
			ID:          c.ID,
			Name:        c.Name,
			Job:         job(c.Class, c.Sex),
			Level:       c.Level,
			PlayMinutes: c.PlayMinutes,
			ST:          c.ST,
			HT:          c.HT,
			DX:          c.DX,
			IQ:          c.IQ,
			HairPart:    c.Hair,
			X:           c.X,
			Y:           c.Y,
			SkillGroup:  skillGroupNumber(c.Class, c.SkillGroup),
		}
	}
	return out
}
