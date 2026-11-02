package account

// A character's derived values: what follows from its class, level and stats rather than
// being stored. The formula and constants are open-mt2's (PlayerPoints.ts, jobs.json); no
// source closer to the original game has been checked yet.

// Points are a character's derived values.
type Points struct {
	MaxHP       uint32
	MaxSP       uint32
	MaxStamina  uint32
	AttackSpeed uint8
	MoveSpeed   uint8
	// NextExp is the experience the next level needs; 0 where it is not known yet.
	NextExp uint32
}

// growth is how a class's hit points and mana grow with its stats and level.
type growth struct {
	hp, hpPerVitality, hpPerLevel     uint32
	sp, spPerIntelligence, spPerLevel uint32
}

var growths = map[Class]growth{
	Warrior: {hp: 700, hpPerVitality: 50, hpPerLevel: 100, sp: 500, spPerIntelligence: 20, spPerLevel: 50},
	Ninja:   {hp: 700, hpPerVitality: 50, hpPerLevel: 100, sp: 500, spPerIntelligence: 20, spPerLevel: 50},
	Sura:    {hp: 700, hpPerVitality: 50, hpPerLevel: 100, sp: 700, spPerIntelligence: 20, spPerLevel: 50},
	Shaman:  {hp: 700, hpPerVitality: 50, hpPerLevel: 100, sp: 700, spPerIntelligence: 20, spPerLevel: 50},
}

// nextExp is the experience each level needs to reach the next, from open-mt2's exp
// table; only the levels a character can have so far are listed.
var nextExp = map[uint8]uint32{1: 300}

// Points works out the character's derived values; false for a class the game does not
// have.
func (c Character) Points() (Points, bool) {
	g, ok := growths[c.Class]
	if !ok {
		return Points{}, false
	}
	level := uint32(c.Level)
	return Points{
		MaxHP:       g.hp + uint32(c.Vitality)*g.hpPerVitality + level*g.hpPerLevel,
		MaxSP:       g.sp + uint32(c.Intelligence)*g.spPerIntelligence + level*g.spPerLevel,
		MaxStamina:  1000,
		AttackSpeed: 150,
		MoveSpeed:   150,
		NextExp:     nextExp[c.Level],
	}, true
}
