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
// alight is handed to the world loop to tick. Called under the lock.
func (w *presence) burn(v, by *present, each int) {
	f := v.fight
	f.burnTurns, f.burner, f.burnEach = burnTurns, by, each
	if !f.burning {
		f.burning = true
		w.burned = append(w.burned, v)
		showMob(v)
	}
}

// burnTick takes the fire's turn off the monster for its burner, who is told it, as is
// whoever picked it; one it kills dies at their hand. It reports whether the fire burns
// on; it goes out, shown to everyone, after its last turn, or once its burner has left the
// world or died. A burn never provokes. Called by the world loop.
func (w *presence) burnTick(v *present, now time.Time) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	f := v.fight
	if v.dead() || w.all[v.add.VID] != v || !f.burning {
		return false
	}
	by := f.burner
	if w.all[by.add.VID] != by || by.dead() {
		w.putOut(v)
		return false
	}
	damage := min(f.burnEach, f.hp)
	f.hp -= damage
	f.damageBy[by] += damage
	by.send(protocol.EncodeDamage(v.add.VID, protocol.DamageNormal, int32(damage))) //nolint:gosec // at most the hit points
	for t := range f.targetedBy {
		t.send(protocol.EncodeTarget(v.add.VID, f.hpPercent()))
	}
	if f.hp == 0 {
		w.kill(by, v, now)
		return false
	}
	if f.burnTurns--; f.burnTurns == 0 {
		w.putOut(v)
		return false
	}
	return true
}

// putOut ends the monster's fire for everyone who sees it. Called under the lock.
func (w *presence) putOut(v *present) {
	v.fight.burning = false
	showMob(v)
}

// burning is a monster alight the world loop ticks, and when it ticks next.
type burning struct {
	p    *present
	next time.Time
}

// tickBurns takes the turn of each fire whose time has come, and drops those that went
// out. Only the burning are walked.
func (svc *Service) tickBurns(now time.Time) {
	kept := svc.burnings[:0]
	for _, b := range svc.burnings {
		if now.Before(b.next) {
			kept = append(kept, b)
			continue
		}
		if svc.presence.burnTick(b.p, now) {
			kept = append(kept, burning{b.p, now.Add(burnEvery)})
		}
	}
	clear(svc.burnings[len(kept):])
	svc.burnings = kept
}
