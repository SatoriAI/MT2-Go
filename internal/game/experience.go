package game

import (
	"github.com/SatoriAI/MT2-Go/internal/account"
	"github.com/SatoriAI/MT2-Go/internal/protocol"
	"github.com/SatoriAI/MT2-Go/internal/skill"
)

// shareExp gives out a dead monster's experience, multiplied by the running event: each
// player who hurt it and is still in the world, alive, gets the share of it that matches
// the share of its hit points they took, less or more by the gap between its level and
// theirs. What rounding leaves goes to the killer.
func (l locked) shareExp(f *fight, killer *present) {
	exp := uint64(f.exp)
	if l.event != nil {
		exp = l.event.Exp(exp)
	}
	total := uint64(f.maxHP) //nolint:gosec // hit points are positive
	gains := map[*present]uint64{}
	var given uint64
	for p, damage := range f.damageBy {
		share := exp * uint64(damage) / total //nolint:gosec // damage is positive
		given += share
		if l.all[p.add.VID] == p && !p.dead() {
			gains[p] += share
		}
	}
	gains[killer] += exp - given
	for p, share := range gains {
		gain := share * uint64(account.GapPercent(f.defender.Level, p.char.Level)) / 100
		if gain > 0 {
			l.gainExp(p, uint32(gain)) //nolint:gosec // at most 18 times a monster's experience, far under 2^32
		}
	}
}

// gainExp adds experience to a player and tells them; on a level up, tells them their new
// level, needs, stat and skill points and maximums, and everyone who sees them the new
// level.
func (l locked) gainExp(p *present, gain uint32) {
	c := p.char
	levels := c.GainExp(gain)
	vid := p.add.VID
	p.send(pointTold(vid, protocol.PointExp, gain, c.Exp))
	if levels == 0 {
		return
	}
	p.info.Level = uint32(c.Level)
	gained := int32(levels) //nolint:gosec // at most the 98 levels there are
	points, _ := c.Points()
	c.HP, c.SP = points.MaxHP, points.MaxSP
	p.send(
		pointTold(vid, protocol.PointLevel, gained, c.Level),
		pointTold(vid, protocol.PointNextExp, 0, points.NextExp),
		pointTold(vid, protocol.PointStatPoints, gained*account.StatPointsPerLevel, c.StatPoints),
		pointTold(vid, protocol.PointMaxHP, 0, points.MaxHP),
		pointTold(vid, protocol.PointHP, 0, points.MaxHP),
		pointTold(vid, protocol.PointMaxSP, 0, points.MaxSP),
		pointTold(vid, protocol.PointSP, 0, points.MaxSP),
		pointTold(vid, protocol.PointDefence, 0, defence(c)),
	)
	if c.SkillGroup != "" {
		p.send(pointTold(vid, protocol.PointSkillPoints, 0, skillPoints(c)))
	}
	l.toViewers(p, pointTold(vid, protocol.PointLevel, gained, c.Level))
}

// statsByName are the stats a player may spend a point on, as the character window's
// buttons name them, with the point the client shows each at.
var statsByName = map[string]struct {
	stat  func(*account.Character) *uint8
	point protocol.Point
}{
	"st": {func(c *account.Character) *uint8 { return &c.Strength }, protocol.PointST},
	"ht": {func(c *account.Character) *uint8 { return &c.Vitality }, protocol.PointHT},
	"dx": {func(c *account.Character) *uint8 { return &c.Dexterity }, protocol.PointDX},
	"iq": {func(c *account.Character) *uint8 { return &c.Intelligence }, protocol.PointIQ},
}

// spendStatPoint moves a stat point into the named stat, if the player has one and the
// stat is below its cap, and tells them the stat, their points left and, for vitality or
// intelligence, their new maximum hit points or mana. Anything else is ignored.
func (w *record) spendStatPoint(vid uint32, name string) {
	s, known := statsByName[name]
	if !known {
		return
	}
	l, unlock := w.lock()
	defer unlock()
	p := l.all[vid]
	c := p.char
	stat := s.stat(c)
	if !c.SpendStatPoint(stat) {
		return
	}
	points, _ := c.Points()
	packets := [][]byte{
		pointTold(vid, s.point, 1, *stat),
		pointTold(vid, protocol.PointStatPoints, -1, c.StatPoints),
	}
	switch name {
	case "ht":
		packets = append(packets, pointTold(vid, protocol.PointMaxHP, 0, points.MaxHP),
			pointTold(vid, protocol.PointDefence, 0, defence(c)))
	case "iq":
		packets = append(packets, pointTold(vid, protocol.PointMaxSP, 0, points.MaxSP))
	}
	// a defence buff grows with intelligence or strength
	if (name == "iq" || name == "st") && buffBonus(c, skill.Defence) > 0 {
		packets = append(packets, pointTold(vid, protocol.PointDefence, 0, defence(c)))
	}
	p.send(packets...)
}
