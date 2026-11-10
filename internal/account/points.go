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
	// NextExp is the experience the next level needs; 0 at the top level.
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

// Points works out the character's derived values; false for a class the game does not
// have.
func (c Character) Points() (Points, bool) {
	g, ok := growths[c.Class]
	if !ok {
		return Points{}, false
	}
	level := uint32(c.Level)
	return Points{
		MaxHP:       g.hp + uint32(c.Vitality)*g.hpPerVitality + level*g.hpPerLevel + uint32(max(0, c.Totals.MaxHP)), //nolint:gosec // never below 0
		MaxSP:       g.sp + uint32(c.Intelligence)*g.spPerIntelligence + level*g.spPerLevel,
		MaxStamina:  1000,
		AttackSpeed: 150,
		MoveSpeed:   150,
		NextExp:     NextExp(c.Level),
	}, true
}

// Enter sets the hit points and mana the character enters the world with: all of them if
// it was never saved in play, otherwise what it was saved with, cut to its maximum now.
func (c *Character) Enter(p Points) {
	if c.HP == 0 {
		c.HP, c.SP = p.MaxHP, p.MaxSP
	}
	c.HP, c.SP = min(c.HP, p.MaxHP), min(c.SP, p.MaxSP)
}
