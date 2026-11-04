package game

import (
	"math"
	"math/rand/v2"
	"time"

	"github.com/SatoriAI/MT2-Go/internal/combat"
	"github.com/SatoriAI/MT2-Go/internal/protocol"
)

// attackReach is how far from its victim a blow may land: open-mt2's, a little more than
// the client's own swing reaches, since positions arrive late.
const attackReach = 300

// The shortest time between two blows of a player on one victim is blowGapAtSpeed divided
// by their attack speed, never under minBlowGap: open-mt2's 15000 ms and 80 ms.
const (
	blowGapAtSpeed = 15000 * time.Millisecond
	minBlowGap     = 80 * time.Millisecond
)

// fight is what a monster has to lose. The world record's lock guards it.
type fight struct {
	defender   combat.Defender
	hp, maxHP  int
	diedAt     time.Time // zero while alive
	targetedBy map[*present]struct{}
	// The last player to land a blow, and when: the victim's side of the blow gap, which a
	// player switching between victims cannot get round.
	lastAttacker *present
	lastBlowAt   time.Time
	// damageBy is what each player took off it, which shares its experience out.
	damageBy map[*present]int
	exp      uint32
}

func (p *present) dead() bool { return p.fight != nil && !p.fight.diedAt.IsZero() }

// hpPercent is the share of its hit points the monster has left, as the target window
// draws it.
func (f *fight) hpPercent() uint8 {
	return uint8(math.Round(float64(f.hp) * 100 / float64(f.maxHP)))
}

// untarget stops player p targeting t, if it did.
func untarget(p, t *present) {
	if p.target == t {
		p.target = nil
		delete(t.fight.targetedBy, p)
	}
}

// target makes the monster vid the player's target and tells them its hit points; a vid
// that is not a living monster they see, 0 included, leaves them with none.
func (w *presence) target(playerVID, vid uint32) {
	w.mu.Lock()
	defer w.mu.Unlock()
	p := w.all[playerVID]
	if p.target != nil {
		untarget(p, p.target)
	}
	t, seen := p.sees[vid]
	if !seen || t.fight == nil || t.dead() {
		return
	}
	p.target = t
	t.fight.targetedBy[p] = struct{}{}
	p.send(protocol.EncodeTarget(vid, t.fight.hpPercent()))
}

// blow is what became of an attack.
type blow int

const (
	blowLanded blow = iota
	blowKilled
	blowRefused // nothing to hit there: out of sight or reach, not a monster, or dead
	blowTooSoon // the victim took a blow from the player less than their blow gap ago
)

// attack lands a blow of the player on the monster vid: the attacker is told the damage,
// whoever targets it its hit points, and, if it dies, everyone who sees it its death. A
// blow sooner than gap after the player's last on the same victim is dropped.
func (w *presence) attack(playerVID, vid uint32, gap time.Duration, r *rand.Rand, now time.Time) blow {
	w.mu.Lock()
	defer w.mu.Unlock()
	p := w.all[playerVID]
	v, seen := p.sees[vid]
	if !seen || v.fight == nil || v.dead() || !within(p, v, attackReach) {
		return blowRefused
	}
	f := v.fight
	if f.lastAttacker == p && now.Sub(f.lastBlowAt) < gap {
		return blowTooSoon
	}
	f.lastAttacker, f.lastBlowAt = p, now
	a := combat.Attacker{Level: p.char.Level, Class: p.char.Class, Stats: p.char.Stats}
	damage := min(combat.Damage(a, f.defender, r), f.hp)
	f.hp -= damage
	f.damageBy[p] += damage
	p.send(protocol.EncodeDamage(vid, protocol.DamageNormal, int32(damage))) //nolint:gosec // at most the hit points
	for t := range f.targetedBy {
		t.send(protocol.EncodeTarget(vid, f.hpPercent()))
	}
	if f.hp > 0 {
		return blowLanded
	}
	f.diedAt = now
	w.fallen = append(w.fallen, death{v, now})
	for t := range f.targetedBy {
		untarget(t, v)
	}
	toViewers(v, protocol.EncodeDead(vid))
	w.shareExp(f, p)
	return blowKilled
}

// death is a monster that died, for the world loop to take away and bring back.
type death struct {
	p  *present
	at time.Time
}

// takeFallen hands the world loop the monsters that died since it last asked, so the loop
// learns of deaths without looking at every monster under the lock.
func (w *presence) takeFallen() []death {
	w.mu.Lock()
	defer w.mu.Unlock()
	fallen := w.fallen
	w.fallen = nil
	return fallen
}

func within(p, o *present, reach int64) bool {
	dx, dy := int64(p.add.X)-int64(o.add.X), int64(p.add.Y)-int64(o.add.Y)
	return dx*dx+dy*dy <= reach*reach
}

// blowGap is the least time between a player's blows on one victim at their attack speed.
func blowGap(attackSpeed uint8) time.Duration {
	return max(minBlowGap, blowGapAtSpeed/time.Duration(max(attackSpeed, 1)))
}

// onTarget records the character the player picked, and shows a monster's hit points.
func (s *session) onTarget(body []byte) error {
	vid, err := protocol.DecodeTarget(body)
	if err != nil {
		return err
	}
	s.svc.presence.target(s.playing.vid, vid)
	return nil
}

// onAttack lands a blow the player's client saw hit. Skills are not handled yet. A blow
// faster than the player's attack speed allows on the same victim is dropped, checked on
// both sides, as the original does: the attacker's last victim here, and the victim's last
// attacker in the world record, which catches a player switching between victims.
func (s *session) onAttack(body []byte) error {
	a, err := protocol.DecodeAttack(body)
	if err != nil {
		return err
	}
	now := time.Now()
	p := s.playing
	if a.Skill != 0 || (a.Victim == s.lastVictim && now.Sub(s.lastBlow) < blowGap(p.points.AttackSpeed)) {
		return nil
	}
	s.lastVictim, s.lastBlow = a.Victim, now
	if s.svc.presence.attack(p.vid, a.Victim, blowGap(p.points.AttackSpeed), s.rand, now) == blowKilled {
		s.conn.Log().Debug("killed", "character", p.Name, "victim", a.Victim)
	}
	return nil
}
