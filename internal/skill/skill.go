// Package skill holds the skills the game has: whose they are, how far they rise, and how
// many skill points a character has to raise them with.
package skill

import "github.com/SatoriAI/MT2-Go/internal/account"

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

// Proto is what one skill is: the class and skill group whose it is.
type Proto struct {
	Class account.Class
	Group account.SkillGroup
}

var protos = map[account.Skill]Proto{
	DragonSwirl:    {Class: account.Sura, Group: "weaponry"},
	EnchantedBlade: {Class: account.Sura, Group: "weaponry"},
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
