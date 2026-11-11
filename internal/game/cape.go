package game

import (
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
// those fighting no one, whatever their level, and those on them already. Each counts its
// fight from when it will have walked to the player, so none gives up on the way.
func (svc *Service) lure(lured []*present, now time.Time) {
	for _, p := range lured {
		x, y, around := svc.record.around(p, lureReach)
		pulled := 0
		for _, o := range around {
			m := svc.byPresent[o]
			if pulled == lureCap {
				break
			}
			if m == nil || !m.diedAt.IsZero() || m.gone || m.chase != nil && m.chase.target != p {
				continue
			}
			if m.chase == nil {
				svc.chaseOn(m, p, now)
			}
			m.chase.lastExchange = now.Add(walkTime(m, x-m.x, y-m.y))
			pulled++
		}
	}
}

// around is where the player stands and the living monsters in their view within reach,
// stones left out, the nearest first; none for a player who left the world or died.
func (w *record) around(p *present, reach int64) (int32, int32, []*present) {
	l, unlock := w.lock()
	defer unlock()
	if l.all[p.add.VID] != p || p.dead() {
		return 0, 0, nil
	}
	var near []*present
	for _, o := range p.sees {
		if o.fight != nil && !o.fight.stone && !o.dead() && within(p, o, reach) {
			near = append(near, o)
		}
	}
	slices.SortFunc(near, func(a, b *present) int { return int(distance(p, a) - distance(p, b)) })
	return p.add.X, p.add.Y, near
}

// walkTime is how long the monster takes to walk (dx, dy) at its own speed.
func walkTime(m *monster, dx, dy int32) time.Duration {
	return time.Duration(math.Hypot(float64(dx), float64(dy)) /
		(walkPerSecond * float64(max(m.proto.MoveSpeed, 1)) / 100) * float64(time.Second))
}
