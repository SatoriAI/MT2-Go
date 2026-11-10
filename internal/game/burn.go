package game

import (
	"time"

	"github.com/SatoriAI/MT2-Go/internal/protocol"
)

// A burn takes its share of its whole every burnEvery for burnTurns turns, the designer's
// turns for the veteran's whole: the skilltable gives the whole, not how it falls.
const (
	burnEvery = 2 * time.Second
	burnTurns = 5
)

// burn sets the burner's fire on the monster for burnTurns turns, taking each a
// turn, however many the last had left, and shows it to everyone who sees it; one newly
// alight is handed to the world loop to tick.
func (l locked) burn(v, by *present, each int) {
	f := v.fight
	f.fire = dose{by: by, each: each, turns: burnTurns}
	if f.start(fire) {
		l.news.burned = append(l.news.burned, v)
		l.showMob(v)
	}
}

// burnTick takes the fire's turn off the monster for its burner, who is told it, as is
// whoever picked it; one it kills dies at their hand. It reports whether the fire burns
// on; it goes out, shown to everyone, after its last turn, or once its burner has left the
// world or died. A burn never provokes. Called by the world loop.
func (w *record) burnTick(v *present, now time.Time) bool {
	l, unlock := w.lock()
	defer unlock()
	f := v.fight
	if v.dead() || l.all[v.add.VID] != v || !f.has(fire) {
		return false
	}
	b := &f.fire
	if l.all[b.by.add.VID] != b.by || b.by.dead() {
		l.cure(v, fire)
		return false
	}
	if !l.turnDamage(v, b.by, b.each, protocol.DamageNormal, now) {
		return false
	}
	if b.turns--; b.turns == 0 {
		l.cure(v, fire)
		return false
	}
	return true
}

// turnDamage takes a turn's damage of a poison or fire off the monster for whoever set it, who
// is told it with the flag, as is whoever picked the monster; one it kills dies at their
// hand. It reports whether the monster lives on.
func (l locked) turnDamage(v, by *present, each int, flag uint8, now time.Time) bool {
	f := v.fight
	damage := min(each, f.hp)
	f.hp -= damage
	f.damageBy[by] += damage
	by.send(protocol.EncodeDamage(v.add.VID, flag, int32(damage))) //nolint:gosec // at most the hit points
	for t := range f.targetedBy {
		t.send(protocol.EncodeTarget(v.add.VID, f.hpPercent()))
	}
	if f.hp == 0 {
		l.kill(by, v, now)
		return false
	}
	return true
}
