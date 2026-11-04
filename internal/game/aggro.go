package game

import (
	"math"
	"math/rand/v2"
	"time"

	"github.com/SatoriAI/MT2-Go/internal/combat"
	"github.com/SatoriAI/MT2-Go/internal/protocol"
)

// A monster that is hit fights back, with open-mt2's numbers: it takes a step every
// chaseStep, bites within its attack range and 15% more, and walks to nine tenths of
// its range when it is further. It gives up after giveUpAfter without a bite or a blow
// from its target, or once it is leash from where it last bit.
const (
	chaseStep   = 500 * time.Millisecond
	giveUpAfter = 10 * time.Second
	leash       = 5000
	// walkPerSecond is how far a monster of move speed 100 gets in a second: this server's
	// estimate of the client's walk, which the client animates at its own pace.
	walkPerSecond = 400
)

// provocation is a blow a player landed on a living monster, for the world loop to answer.
type provocation struct {
	monster, by *present
}

// chase is a monster's fight with one player. Only the world loop touches it.
type chase struct {
	target       *present
	nextStep     time.Time
	nextBite     time.Time
	lastFought   time.Time // when it last bit, or its target last hit it
	fromX, fromY int32     // where it last bit, or was first hit
}

// engage answers the blows landed since the last tick: the monster is hurt, so it heals
// from then on, and one that is not fighting takes the player who hit it as its target;
// one fighting them fights on, however long since its last bite. Blows from others are
// not answered while it fights.
func (s *Service) engage(provoked []provocation, now time.Time) {
	for _, pr := range provoked {
		m := s.byPresent[pr.monster]
		if m == nil || !m.diedAt.IsZero() {
			continue // gone since
		}
		s.hurt(m, now)
		if m.chase != nil {
			if m.chase.target == pr.by {
				m.chase.lastFought = now
			}
			continue
		}
		m.chase = &chase{target: pr.by, nextStep: now, nextBite: now, lastFought: now, fromX: m.x, fromY: m.y}
		s.fighting = append(s.fighting, m)
	}
}

// fightOn takes the step of every fighting monster whose step is due, and sends home those
// that gave up. Only fighting monsters are walked, so an idle tick costs nothing more.
func (s *Service) fightOn(now time.Time) {
	kept := s.fighting[:0]
	for _, m := range s.fighting {
		switch {
		case m.chase == nil: // it died
		case now.Before(m.chase.nextStep) || s.presence.fightStep(m, now, s.clock.Now(), s.rand):
			kept = append(kept, m)
		default:
			s.goHome(m, now)
		}
	}
	clear(s.fighting[len(kept):])
	s.fighting = kept
}

// goHome ends the monster's fight and walks it back to where it was spawned, where it
// strolls again. The world record has it home at once, while the client walks it there.
func (s *Service) goHome(m *monster, now time.Time) {
	m.chase = nil
	s.presence.move(m.p.add.VID, protocol.Move{Type: protocol.MoveWait, Rotation: rotationTowards(m.homeX-m.x, m.homeY-m.y),
		X: m.homeX, Y: m.homeY, Time: s.clock.Now()})
	walk := time.Duration(math.Hypot(float64(m.homeX-m.x), float64(m.homeY-m.y)) /
		(walkPerSecond * float64(max(m.proto.MoveSpeed, 1)) / 100) * float64(time.Second))
	m.x, m.y = m.homeX, m.homeY
	m.nextStroll = now.Add(walk + s.strollWait()) // not before the client has walked it home
}

// fightStep takes one step of the monster's fight: a bite if its target is in reach and
// its bite is due, otherwise a walk towards it. It reports false when the monster stops:
// it died, its target left the world or died, it has not fought for too long, or it
// strayed too far. Called from the world loop, which owns m.
func (w *presence) fightStep(m *monster, now time.Time, clock uint32, r *rand.Rand) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	c, me := m.chase, m.p
	t := c.target
	if me.dead() || w.all[t.add.VID] != t || t.dead() || now.Sub(c.lastFought) >= giveUpAfter ||
		!near(me.add.X, me.add.Y, c.fromX, c.fromY, leash) {
		return false
	}
	c.nextStep = now.Add(chaseStep)
	if within(me, t, int64(m.proto.AttackRange)*115/100) {
		if !now.Before(c.nextBite) {
			w.bite(m, t, now, clock, r)
		}
		return true
	}
	x, y := approach(me.add.X, me.add.Y, t.add.X, t.add.Y, float64(m.proto.AttackRange)*0.9,
		walkPerSecond*float64(m.proto.MoveSpeed)/100*chaseStep.Seconds())
	w.moveLocked(me, protocol.Move{Type: protocol.MoveWait, Rotation: rotationTowards(x-me.add.X, y-me.add.Y),
		X: x, Y: y, Time: clock})
	m.x, m.y = x, y
	return true
}

// bite has the monster attack its target where it stands: everyone who sees it sees the
// attack, and the target is told the damage and their hit points left, and dies at none.
func (w *presence) bite(m *monster, t *present, now time.Time, clock uint32, r *rand.Rand) {
	me, c, ch := m.p, m.chase, t.char
	rotation := rotationTowards(t.add.X-me.add.X, t.add.Y-me.add.Y)
	me.add.Rotation = float32(rotation) * 5
	toViewers(me, protocol.EncodeMove(me.add.VID, protocol.Move{Type: protocol.MoveAttack, Rotation: rotation,
		X: me.add.X, Y: me.add.Y, Time: clock}))
	b := combat.Biter{Level: m.proto.Level, Strength: m.proto.Strength, DamageMin: m.proto.DamageMin,
		DamageMax: m.proto.DamageMax}
	damage := min(uint32(combat.Bite(b, combat.Defender{Level: ch.Level, Vitality: ch.Vitality}, r)), ch.HP) //nolint:gosec // at least 1
	ch.HP -= damage
	ch.LastFought = now
	vid := t.add.VID
	t.send(protocol.EncodeDamage(vid, protocol.DamageNormal, int32(damage)), //nolint:gosec // at most the hit points
		protocol.EncodePointChange(vid, protocol.PointHP, -int32(damage), int32(ch.HP))) //nolint:gosec // small
	c.lastFought, c.fromX, c.fromY, c.nextBite = now, me.add.X, me.add.Y, now.Add(biteGap(m.proto.AttackSpeed))
	if ch.HP == 0 {
		w.die(t)
	}
}

// biteGap is the time between a monster's bites at its attack speed: two seconds at 100,
// never under one, as open-mt2 has it.
func biteGap(attackSpeed uint8) time.Duration {
	return max(time.Second, 2*time.Second*100/time.Duration(max(attackSpeed, 1)))
}

// approach is the point a walk from (x, y) towards (tx, ty) reaches, stopping short at
// distance stop from it, and covering at most step.
func approach(x, y, tx, ty int32, stop, step float64) (int32, int32) {
	dx, dy := float64(tx-x), float64(ty-y)
	dist := math.Hypot(dx, dy)
	walk := min(dist-stop, step)
	if walk <= 0 {
		return x, y
	}
	return x + int32(math.Round(dx/dist*walk)), y + int32(math.Round(dy/dist*walk))
}

// near is whether (x, y) is less than reach from (fx, fy).
func near(x, y, fx, fy int32, reach int64) bool {
	dx, dy := int64(x)-int64(fx), int64(y)-int64(fy)
	return dx*dx+dy*dy < reach*reach
}
