package game

import (
	"math/rand/v2"
	"time"

	"github.com/SatoriAI/MT2-Go/internal/item"
	"github.com/SatoriAI/MT2-Go/internal/protocol"
)

// shootReach is how far from its victim an arrow may land: the bow's 2500 in the client's
// skilltable and a margin of 300 for positions arriving late, the debaters' reach.
const shootReach = 2800

// A bow's shot takes a motion of 1.5 s at attack speed 100, the client's bow attack.msa,
// and the next is let go no sooner than shotGapAtSpeed over the attack speed: that motion
// less a sixth, for the client's timing, the debaters' limit.
const shotGapAtSpeed = 1250 * time.Millisecond * 100

// shotGap is the shortest time between two of a player's shots at an attack speed.
func shotGap(attackSpeed uint8) time.Duration {
	return shotGapAtSpeed / time.Duration(max(attackSpeed, 1))
}

// onFlyTarget takes what the player's next arrow flies at, and shows it to everyone who
// sees them, so their clients draw the arrow flying there.
func (s *session) onFlyTarget(body []byte) error {
	f, err := protocol.DecodeFlyTarget(body)
	if err != nil {
		return err
	}
	s.flyTarget = f.Target
	s.svc.presence.aim(s.playing.vid, protocol.ServerFlyTarget, f)
	return nil
}

// onAddFlyTarget shows everyone who sees the player another target of their skill's
// arrows. Skills that shoot are not used yet, so it changes nothing else.
func (s *session) onAddFlyTarget(body []byte) error {
	f, err := protocol.DecodeFlyTarget(body)
	if err != nil {
		return err
	}
	s.svc.presence.aim(s.playing.vid, protocol.ServerAddFlyTarget, f)
	return nil
}

// onShoot lands the arrow the player let go on what it flew at, and uses one of their
// arrows up. The client decides nothing about the hit, so the server does: a shot with no
// target, sooner than the bow lets another go, without arrows worn, by a skill, or at
// nothing it can hit is dropped, and no arrow is used.
func (s *session) onShoot(body []byte) error {
	sk, err := protocol.DecodeShoot(body)
	if err != nil {
		return err
	}
	now := s.svc.now()
	target := s.flyTarget
	s.flyTarget = 0
	arrows, worn := s.inventory.WornIn(item.Arrow)
	gap := shotGap(s.svc.presence.attackSpeedOf(s.playing.vid))
	if sk != 0 || target == 0 || !worn || now.Sub(s.lastShot) < gap {
		s.conn.Log().Debug("shot dropped", "skill", sk, "target", target, "arrows", worn)
		return nil
	}
	if s.svc.presence.shoot(s.playing.vid, target, s.rand, now) == blowRefused {
		return nil
	}
	s.lastShot = now
	_, err = s.useOne(arrows)
	return err
}

// aim shows everyone who sees the player what their arrows fly at, while they live and
// hold a bow.
func (w *presence) aim(vid uint32, header protocol.ServerHeader, f protocol.FlyTarget) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if p := w.all[vid]; !p.dead() && bowInHand(p.char) {
		toViewers(p, protocol.EncodeFlyTarget(header, vid, f))
	}
}

// shoot lands the player's arrow on the monster vid as a blow, if they hold a bow, and it
// is alive, in sight and within reach. Their session has seen their arrows.
func (w *presence) shoot(playerVID, vid uint32, r *rand.Rand, now time.Time) blow {
	w.mu.Lock()
	defer w.mu.Unlock()
	p := w.all[playerVID]
	v, seen := p.sees[vid]
	if p.dead() || !seen || v.fight == nil || v.dead() || !within(p, v, shootReach) || !bowInHand(p.char) {
		return blowRefused
	}
	return w.land(p, v, r, now)
}
