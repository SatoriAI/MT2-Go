package game

import (
	"github.com/SatoriAI/MT2-Go/internal/account"
	"github.com/SatoriAI/MT2-Go/internal/protocol"
)

// shareExp gives out a dead monster's experience: each player who hurt it and is still in
// the world gets the share of it that matches the share of its hit points they took, less
// or more by the gap between its level and theirs. What rounding leaves goes to the
// killer. Called under the lock.
func (w *presence) shareExp(f *fight, killer *present) {
	total := uint64(f.maxHP) //nolint:gosec // hit points are positive
	gains := map[*present]uint64{}
	var given uint64
	for p, damage := range f.damageBy {
		share := uint64(f.exp) * uint64(damage) / total //nolint:gosec // damage is positive
		given += share
		if w.all[p.add.VID] == p {
			gains[p] += share
		}
	}
	gains[killer] += uint64(f.exp) - given
	for p, share := range gains {
		gain := share * uint64(account.GapPercent(f.defender.Level, p.char.Level)) / 100
		if gain > 0 {
			w.gainExp(p, uint32(gain)) //nolint:gosec // at most a monster's experience
		}
	}
}

// gainExp adds experience to a player and tells them; on a level up, tells them their new
// level, needs, stat points and maximums, and everyone who sees them the new level.
func (w *presence) gainExp(p *present, gain uint32) {
	c := p.char
	levels := c.GainExp(gain)
	vid := p.add.VID
	p.send(protocol.EncodePointChange(vid, protocol.PointExp, int32(gain), int32(c.Exp))) //nolint:gosec // under 2^31
	if levels == 0 {
		return
	}
	p.info.Level = uint32(c.Level)
	gained := int32(levels) //nolint:gosec // at most the 98 levels there are
	points, _ := c.Points()
	p.send(
		protocol.EncodePointChange(vid, protocol.PointLevel, gained, int32(c.Level)),
		protocol.EncodePointChange(vid, protocol.PointNextExp, 0, int32(points.NextExp)), //nolint:gosec // under 2^31
		protocol.EncodePointChange(vid, protocol.PointStatPoints, gained*account.StatPointsPerLevel, int32(c.StatPoints)),
		protocol.EncodePointChange(vid, protocol.PointMaxHP, 0, int32(points.MaxHP)), //nolint:gosec // small
		protocol.EncodePointChange(vid, protocol.PointHP, 0, int32(points.MaxHP)),    //nolint:gosec // small
		protocol.EncodePointChange(vid, protocol.PointMaxSP, 0, int32(points.MaxSP)), //nolint:gosec // small
		protocol.EncodePointChange(vid, protocol.PointSP, 0, int32(points.MaxSP)),    //nolint:gosec // small
	)
	toViewers(p, protocol.EncodePointChange(vid, protocol.PointLevel, gained, int32(c.Level)))
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
func (w *presence) spendStatPoint(vid uint32, name string) {
	s, known := statsByName[name]
	if !known {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	p := w.all[vid]
	c := p.char
	stat := s.stat(c)
	if !c.SpendStatPoint(stat) {
		return
	}
	points, _ := c.Points()
	packets := [][]byte{
		protocol.EncodePointChange(vid, s.point, 1, int32(*stat)),
		protocol.EncodePointChange(vid, protocol.PointStatPoints, -1, int32(c.StatPoints)),
	}
	switch name {
	case "ht":
		packets = append(packets, protocol.EncodePointChange(vid, protocol.PointMaxHP, 0, int32(points.MaxHP))) //nolint:gosec // small
	case "iq":
		packets = append(packets, protocol.EncodePointChange(vid, protocol.PointMaxSP, 0, int32(points.MaxSP))) //nolint:gosec // small
	}
	p.send(packets...)
}
