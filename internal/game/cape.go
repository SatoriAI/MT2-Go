package game

import (
	"cmp"
	"math"
	"slices"
	"time"

	"github.com/SatoriAI/MT2-Go/internal/item"
)

// What Peleryna Męstwa pulls, as the veteran and designer settled it
// (docs/mechanics/cape.md): the monsters within lureReach, the nearest lureCap of them, and
// one cape at most every capeEvery.
const (
	lureReach = 2500
	lureCap   = 50
	capeEvery = 2 * time.Second
)

// wearCape uses a cape of the stack up to call the monsters around the player onto them. A
// cape is used up whether any monster is near or not, as the original's were; one worn
// too soon after the last, or by the dead, is not.
func (s *session) wearCape(it item.Item) error {
	now := s.svc.now()
	if now.Before(s.lastCape.Add(capeEvery)) || !s.svc.record.alive(s.playing.vid) {
		return nil
	}
	if used, err := s.useOne(it); !used || err != nil {
		return err
	}
	s.lastCape = now
	s.svc.record.lure(s.playing.vid)
	return nil
}

// lure leaves the player for the world loop to call the monsters around them onto, unless
// they died since the cape was used up.
func (w *record) lure(vid uint32) {
	l, unlock := w.lock()
	defer unlock()
	if p := l.all[vid]; !p.dead() {
		l.news.lured = append(l.news.lured, p)
	}
}

// lure sets on each player who wore a cape the monsters around them, the nearest first:
// those fighting no one, whatever their level, one that just gave up among them, and
// those on them already. Each counts its fight from when it will have reached the player,
// so none gives up on the way.
func (svc *Service) lure(lured []*present, now time.Time) {
	for _, p := range lured {
		pulled := 0
		for _, c := range svc.record.around(p, lureReach, now) {
			m := svc.byPresent[c.o]
			if pulled == lureCap {
				break
			}
			if m == nil || !m.diedAt.IsZero() || m.gone || m.chase != nil && m.chase.target != p {
				continue
			}
			if m.chase == nil {
				svc.chaseOn(m, p, now)
			}
			m.chase.lastExchange = now.Add(c.arrives)
			pulled++
		}
	}
}

// pull is a monster a cape may call, and how long it would take to reach the player: its
// stun, then its walk at the speed it moves at now, a slow one slower.
type pull struct {
	o       *present
	arrives time.Duration
}

// around is the living monsters in the player's view within reach, stones left out, the
// nearest first; none for a player who died, nor one who left the world, who sees no one.
func (w *record) around(p *present, reach int64, now time.Time) []pull {
	_, unlock := w.lock()
	defer unlock()
	if p.dead() {
		return nil
	}
	var near []pull
	for _, o := range p.sees {
		if o.fight != nil && !o.fight.stone && !o.dead() && within(p, o, reach) {
			walk := walkTime(o.add.MoveSpeed, p.add.X-o.add.X, p.add.Y-o.add.Y)
			near = append(near, pull{o, max(0, o.fight.stunUntil.Sub(now)) + walk})
		}
	}
	slices.SortFunc(near, func(a, b pull) int { return cmp.Compare(distance(p, a.o), distance(p, b.o)) })
	return near
}

// walkTime is how long a walk of (dx, dy) takes at move speed speed.
func walkTime(speed uint8, dx, dy int32) time.Duration {
	return time.Duration(math.Hypot(float64(dx), float64(dy)) /
		(walkPerSecond * float64(max(speed, 1)) / 100) * float64(time.Second))
}
