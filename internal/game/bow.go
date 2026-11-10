package game

import (
	"math/rand/v2"
	"slices"
	"time"

	"github.com/SatoriAI/MT2-Go/internal/account"
	"github.com/SatoriAI/MT2-Go/internal/item"
	"github.com/SatoriAI/MT2-Go/internal/protocol"
	"github.com/SatoriAI/MT2-Go/internal/skill"
)

// shootReach is how far from its victim an arrow may land, a blow's or a skill's.
const shootReach = skill.ShootReach

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
	s.svc.record.aim(s.playing.vid, protocol.ServerFlyTarget, f)
	return nil
}

// onAddFlyTarget keeps another target the client aimed the arrows of the skill it is about
// to use at, up to as many as a skill shoots, and shows it to everyone who sees the
// player.
func (s *session) onAddFlyTarget(body []byte) error {
	f, err := protocol.DecodeFlyTarget(body)
	if err != nil {
		return err
	}
	if f.Target != 0 && len(s.flyAdds) < maxFlyAdds {
		s.flyAdds = append(s.flyAdds, f.Target)
	}
	s.svc.record.aim(s.playing.vid, protocol.ServerAddFlyTarget, f)
	return nil
}

// maxFlyAdds is the most further targets the client queues for a skill's arrows: the most
// arrows a skill lets go, 2 + 6k at its 0.5, less the one picked.
const maxFlyAdds = 4

// volleyFor is how long after an arrow skill is used its arrows may still be let go: its
// longest motion, Repetitive Shot's five arrows, takes about three seconds.
const volleyFor = 4 * time.Second

// volley is what an arrow skill has left to shoot: each arrow at the next of its targets,
// or all at its one, for percent of a blow, until a time.
type volley struct {
	skill   account.Skill
	level   uint8
	targets []uint32
	left    int
	percent int
	until   time.Time
}

// loose is the volley of an arrow skill used at level on the monster picked, and for a fan
// on the further ones the client aimed at too. Its arrows share its damage, each
// arrow's part of it added to it.
func loose(sk account.Skill, proto skill.Proto, level uint8, picked uint32, adds []uint32, now time.Time) volley {
	power := skill.Power(level)
	arrows := int(proto.Arrows.At(power))
	targets := []uint32{picked}
	percent := int(proto.Damage.At(power))
	if proto.Fan {
		for _, t := range adds {
			if !slices.Contains(targets, t) {
				targets = append(targets, t)
			}
		}
		arrows = min(arrows, len(targets))
	} else {
		percent = (percent + int(proto.EachArrow)*arrows) / arrows
	}
	return volley{skill: sk, level: level, targets: targets, left: arrows, percent: percent, until: now.Add(volleyFor)}
}

// next is the target of the volley's next arrow, and false once it has none left.
func (v *volley) next(sk account.Skill, now time.Time) (uint32, bool) {
	if v.left == 0 || v.skill != sk || now.After(v.until) {
		return 0, false
	}
	at := v.targets[0]
	if len(v.targets) > 1 {
		at, v.targets = v.targets[0], v.targets[1:]
	}
	v.left--
	return at, true
}

// onShoot lands the arrow the player let go on what it flew at, and uses one of their
// arrows up. The client decides nothing about the hit, so the server does: a plain shot
// with no target, or sooner than the bow lets another go, a skill's past what the skill
// last used has left to shoot, one without arrows worn, or at nothing it can hit is
// dropped, and no arrow is used.
func (s *session) onShoot(body []byte) error {
	vnum, err := protocol.DecodeShoot(body)
	if err != nil {
		return err
	}
	now := s.svc.now()
	arrows, worn := s.inventory.WornIn(item.Arrow)
	if vnum != 0 {
		sk, _ := skillByVNum(int(vnum))
		target, left := s.volley.next(sk, now)
		if !worn || !left || s.svc.record.shootSkill(s.playing.vid, target, s.volley, s.rand, now) == blowRefused {
			s.conn.Log().Debug("skill shot dropped", "skill", vnum, "target", target, "arrows", worn)
			return nil
		}
		s.lastShot = now // a plain shot waits on it as on another
		_, err = s.useOne(arrows)
		return err
	}
	target := s.flyTarget
	s.flyTarget, s.flyAdds = 0, nil
	gap := shotGap(s.svc.record.attackSpeedOf(s.playing.vid))
	if target == 0 || !worn || now.Sub(s.lastShot) < gap {
		s.conn.Log().Debug("shot dropped", "target", target, "arrows", worn)
		return nil
	}
	if s.svc.record.shoot(s.playing.vid, target, s.rand, now) == blowRefused {
		return nil
	}
	s.lastShot = now
	_, err = s.useOne(arrows)
	return err
}

// aim shows everyone who sees the player what their arrows fly at, while they live and
// hold a bow.
func (w *record) aim(vid uint32, header protocol.ServerHeader, f protocol.FlyTarget) {
	l, unlock := w.lock()
	defer unlock()
	if p := l.all[vid]; !p.dead() && bowInHand(p.char) {
		l.toViewers(p, protocol.EncodeFlyTarget(header, vid, f))
	}
}

// shoot lands the player's arrow on the monster vid as a blow, if they hold a bow, and it
// is alive, in sight and within reach. Their session has seen their arrows.
func (w *record) shoot(playerVID, vid uint32, r *rand.Rand, now time.Time) blow {
	l, unlock := w.lock()
	defer unlock()
	p := l.all[playerVID]
	v, seen := p.sees[vid]
	if p.dead() || !seen || v.fight == nil || v.dead() || !within(p, v, shootReach) || !bowInHand(p.char) {
		return blowRefused
	}
	return l.land(p, v, r, now)
}

// shootSkill lands an arrow of the volley's skill on the monster vid, with its splash and
// poison, if the player holds a bow, and it is alive, in sight and within reach.
func (w *record) shootSkill(playerVID, vid uint32, v volley, r *rand.Rand, now time.Time) blow {
	l, unlock := w.lock()
	defer unlock()
	p := l.all[playerVID]
	at, seen := p.sees[vid]
	if p.dead() || !seen || at.fight == nil || at.dead() || !within(p, at, shootReach) || !bowInHand(p.char) {
		return blowRefused
	}
	proto, _ := skill.Lookup(v.skill)
	l.splash(p, at, proto, v.percent, v.level, now, r)
	return blowLanded
}

// levelOf is the player's level of the skill.
func (w *record) levelOf(vid uint32, sk account.Skill) uint8 {
	l, unlock := w.lock()
	defer unlock()
	return l.all[vid].char.Skills[sk]
}
