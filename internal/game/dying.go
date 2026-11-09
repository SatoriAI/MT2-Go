package game

import (
	"time"

	"github.com/SatoriAI/MT2-Go/internal/protocol"
)

// die lays the player down at no hit points: they lose the death penalty from their
// experience, and they and everyone who sees them see them fall; the client then opens its
// restart dialog. Monsters chasing them give up at their next step. Called under the lock.
func (w *presence) die(p *present) {
	vid := p.add.VID
	if p.target != nil {
		untarget(p, p.target)
	}
	w.endBuffs(p)
	if lost := p.char.LoseExpOnDeath(); lost > 0 {
		p.send(protocol.EncodePointChange(vid, protocol.PointExp, -int32(lost), int32(p.char.Exp))) //nolint:gosec // under 2^31
	}
	p.send(protocol.EncodeDead(vid))
	toViewers(p, protocol.EncodeDead(vid))
}

// revive brings a dead player back, where they fell or in town at (x, y): with half their
// hit points and mana here, all of them in town. The client has no revive packet, so the
// player is taken out of the world and put back: those who see them get them afresh, and
// they get their own character, alive, then everyone in view. It reports whether the
// player was dead; a restart while alive changes nothing.
func (w *presence) revive(vid uint32, town bool, x, y int32) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	p := w.all[vid]
	if !p.dead() {
		return false
	}
	c := p.char
	points, _ := c.Points()
	c.HP, c.SP = max(points.MaxHP/2, 1), points.MaxSP/2
	c.LastFought = time.Time{} // the fight ended with the death; healing goes on as usual
	if town {
		c.HP, c.SP = points.MaxHP, points.MaxSP
	}
	w.leaveLocked(p)
	if town {
		p.add.X, p.add.Y = x, y
	}
	p.tether = tether{x: p.add.X, y: p.add.Y} // walking on from where they rose
	p.send(protocol.EncodeChat(protocol.ChatCommand, vid, p.info.Empire, "CloseRestartWindow"))
	p.send(p.appearance()...)
	p.send(protocol.EncodePointChange(vid, protocol.PointHP, 0, int32(c.HP)), //nolint:gosec // small
		protocol.EncodePointChange(vid, protocol.PointSP, 0, int32(c.SP))) //nolint:gosec // small
	w.enterLocked(p)
	return true
}
