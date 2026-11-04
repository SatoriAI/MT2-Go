package game

import (
	"context"
	"fmt"
	"math"
	"math/rand/v2"
	"time"

	"github.com/SatoriAI/MT2-Go/internal/mob"
	"github.com/SatoriAI/MT2-Go/internal/protocol"
	"github.com/SatoriAI/MT2-Go/internal/world"
)

// A monster strolls to a point within wanderRange of where it was spawned, every
// wanderMin to wanderMax: open-mt2's numbers for an idle monster.
const (
	wanderRange = 600
	wanderMin   = 5 * time.Second
	wanderMax   = 25 * time.Second
	// tick is how often the world loop looks for something to do; a stroll starts at most
	// this late.
	tick = 100 * time.Millisecond
)

// wanderer is a monster the world loop walks. Only the loop moves it, so where it stands
// here is where the world record has it.
type wanderer struct {
	vid        uint32
	homeX      int32
	homeY      int32
	x, y       int32
	nextStroll time.Time
}

// Run puts the world's mobs in it, then walks the monsters until ctx ends.
func (s *Service) Run(ctx context.Context) error {
	if err := s.spawn(time.Now()); err != nil {
		return err
	}
	t := time.NewTicker(tick)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case now := <-t.C:
			s.tick(now)
		}
	}
}

// spawn puts every spawn's mobs in the world, each at a random point of its area.
func (s *Service) spawn(now time.Time) error {
	for _, sp := range s.spawns {
		proto, ok := mob.Lookup(sp.VNum)
		if !ok {
			return fmt.Errorf("game: spawn of %d, which has no proto", sp.VNum)
		}
		for range sp.Count {
			s.spawnOne(proto, sp, now)
		}
	}
	return nil
}

func (s *Service) spawnOne(proto mob.Proto, sp world.Spawn, now time.Time) {
	x, y := sp.X+s.spread(sp.Spread), sp.Y+s.spread(sp.Spread)
	vid := s.lastVID.Add(1)
	var facing float32 // an NPC faces the way its model was made to
	if proto.Wanders() {
		facing = float32(s.rand.IntN(8) * 45)
	}
	s.presence.enter(&present{
		add: protocol.CharacterAdd{VID: vid, Rotation: facing, X: x, Y: y, Type: entityType(proto.Kind),
			Race: proto.VNum, MoveSpeed: proto.MoveSpeed, AttackSpeed: proto.AttackSpeed},
		info: protocol.CharacterInfo{VID: vid, Name: textToClient(proto.Name)},
	})
	if proto.Wanders() {
		s.wanderers = append(s.wanderers, &wanderer{vid: vid, homeX: x, homeY: y, x: x, y: y,
			nextStroll: now.Add(s.strollWait())})
	}
}

// tick walks every monster whose stroll is due.
func (s *Service) tick(now time.Time) {
	for _, w := range s.wanderers {
		if now.Before(w.nextStroll) {
			continue
		}
		x, y := w.homeX+s.spread(wanderRange), w.homeY+s.spread(wanderRange)
		m := protocol.Move{Type: protocol.MoveWait, Rotation: rotationTowards(x-w.x, y-w.y), X: x, Y: y, Time: s.clock.Now()}
		s.presence.move(w.vid, m)
		w.x, w.y = x, y
		w.nextStroll = now.Add(s.strollWait())
	}
}

// spread is a random offset of at most r either way.
func (s *Service) spread(r int32) int32 {
	if r <= 0 {
		return 0
	}
	return s.rand.Int32N(2*r+1) - r
}

func (s *Service) strollWait() time.Duration {
	return wanderMin + time.Duration(s.rand.Int64N(int64(wanderMax-wanderMin)))
}

// rotationTowards is the client's facing for a step of dx, dy: degrees clockwise from the
// world's +y, as its GetDegreeFromPosition reckons them, in its steps of 5.
func rotationTowards(dx, dy int32) uint8 {
	if dx == 0 && dy == 0 {
		return 0
	}
	degrees := math.Acos(float64(dy)/math.Hypot(float64(dx), float64(dy))) * 180 / math.Pi
	if dx < 0 {
		degrees = 360 - degrees
	}
	return uint8(math.Round(degrees/5)) % 72
}

// newRand is the world's randomness, seeded afresh each start. Where a monster strolls is
// no secret, so a fast generator does.
func newRand() *rand.Rand {
	return rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64())) //nolint:gosec // game dice, not security
}
