package account

// How a character grows: the experience each level needs, how a monster's experience
// shrinks or grows with the gap in levels, and the stat points levels bring. The numbers
// are the original game's, as open-mt2's data records them (exp.ts, expDeltaLevel.ts);
// docs/mechanics/experience.md has the rules.

const (
	MaxLevel           = 99
	StatPointsPerLevel = 3
	// MaxStat is the most points a stat can hold; spending stops there.
	MaxStat = 90
)

// levelNeeds is the experience each level needs to reach the next, from level 1.
var levelNeeds = [MaxLevel - 1]uint32{
	300, 800, 1500, 2500, 4300, 7200, 11000, 17000, 24000, 33000,
	43000, 58000, 76000, 100000, 130000, 169000, 219000, 283000, 365000, 472000,
	610000, 705000, 813000, 937000, 1077000, 1237000, 1418000, 1624000, 1857000, 2122000,
	2421000, 2761000, 3145000, 3580000, 4073000, 4632000, 5194000, 5717000, 6264000, 6837000,
	7600000, 8274000, 8990000, 9753000, 10560000, 11410000, 12320000, 13270000, 14280000, 15340000,
	16870000, 18960000, 19980000, 21420000, 22930000, 24530000, 26200000, 27960000, 29800000, 32780000,
	36060000, 39670000, 43640000, 48000000, 52800000, 58080000, 63890000, 70280000, 77310000, 85040000,
	93540000, 102900000, 113200000, 124500000, 137000000, 150700000, 165700000, 236990000, 260650000, 286780000,
	315380000, 346970000, 381680000, 419770000, 461760000, 508040000, 558740000, 614640000, 676130000, 743730000,
	1041222000, 1145344200, 1259878620, 1385866482, 1524453130, 1676898443, 1844588288, 2029047116,
}

// NextExp is the experience level needs to reach the next; 0 at the top level, which
// gains none.
func NextExp(level uint8) uint32 {
	if level < 1 || level >= MaxLevel {
		return 0
	}
	return levelNeeds[level-1]
}

// DeathPenaltyPercent is the share of the current level's need a character loses on
// dying: this server's choice, since the original's rule is not recorded.
const DeathPenaltyPercent = 2

// LoseExpOnDeath takes the death penalty from the character's experience, never below 0,
// so a level is never lost, and returns how much went.
func (c *Character) LoseExpOnDeath() uint32 {
	lost := min(c.Exp, NextExp(c.Level)*DeathPenaltyPercent/100)
	c.Exp -= lost
	return lost
}

// gapPercents is the share of a monster's experience a player gets, by the monster's level
// less the player's, from 15 or more below to 15 or more above.
var gapPercents = [31]uint32{
	1, 5, 10, 20, 30, 50, 70, 80, 85, 90, 92, 94, 96, 98, 100, // -15 to -1
	100,                                                                       // level with the player
	105, 110, 115, 120, 125, 130, 135, 140, 145, 150, 155, 160, 165, 170, 180, // +1 to +15
}

// GapPercent is the share, in percent, of a monster's experience a player of playerLevel
// gets from one of monsterLevel. Gaps past 15 either way count as 15.
func GapPercent(monsterLevel, playerLevel uint8) uint32 {
	gap := min(max(int(monsterLevel)-int(playerLevel), -15), 15)
	return gapPercents[gap+15]
}

// GainExp adds experience, raising the level as often as it fills, with what is left over
// carried into the next; each level brings StatPointsPerLevel stat points. It returns how
// many levels were gained. At the top level experience stays 0.
func (c *Character) GainExp(n uint32) (levels int) {
	exp := uint64(c.Exp) + uint64(n)
	for c.Level < MaxLevel && exp >= uint64(NextExp(c.Level)) {
		exp -= uint64(NextExp(c.Level))
		c.Level++
		c.StatPoints += StatPointsPerLevel
		levels++
	}
	if c.Level >= MaxLevel {
		exp = 0
	}
	c.Exp = uint32(exp)
	return levels
}

// SpendStatPoint moves one stat point into the stat, if there is one to spend and the
// stat is below MaxStat; false otherwise.
func (c *Character) SpendStatPoint(stat *uint8) bool {
	if c.StatPoints == 0 || *stat >= MaxStat {
		return false
	}
	*stat++
	c.StatPoints--
	return true
}
