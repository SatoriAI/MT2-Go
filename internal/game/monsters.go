package game

import (
	"context"
	"fmt"
	"math"
	"math/rand/v2"
	"time"

	"github.com/SatoriAI/MT2-Go/internal/combat"
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

// A dead monster's body lies corpseTime, as open-mt2's does; a new one takes its place
// in its spawn's area respawnAfter its death, this server's choice for the village
// fields.
const (
	corpseTime   = 2 * time.Second
	respawnAfter = 20 * time.Second
)

// monster is one the world loop walks, if it wanders, and brings back after it dies. Only the loop moves
// it, so where it stands here is where the world record has it.
type monster struct {
	proto      mob.Proto
	spawn      world.Spawn
	p          *present
	homeX      int32
	homeY      int32
	x, y       int32
	nextStroll time.Time
	diedAt     time.Time // zero while alive
	gone       bool      // its body has been taken away
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
	// Whatever can be killed is the loop's to take away and bring back, wandering or not.
	if !proto.Fightable() {
		s.presence.enter(s.newMob(proto, sp.X+s.spread(sp.Spread), sp.Y+s.spread(sp.Spread), 0))
		return
	}
	m := &monster{proto: proto, spawn: sp}
	s.place(m, now)
	s.monsters = append(s.monsters, m)
}

// place puts the monster in the world afresh, at a random point of its spawn's area, with
// a new vid and all its hit points.
func (s *Service) place(m *monster, now time.Time) {
	m.homeX, m.homeY = m.spawn.X+s.spread(m.spawn.Spread), m.spawn.Y+s.spread(m.spawn.Spread)
	m.x, m.y = m.homeX, m.homeY
	delete(s.byPresent, m.p)
	m.p = s.newMob(m.proto, m.x, m.y, float32(s.rand.IntN(8)*45))
	m.nextStroll, m.diedAt, m.gone = now.Add(s.strollWait()), time.Time{}, false
	s.byPresent[m.p] = m
	s.presence.enter(m.p)
}

func (s *Service) newMob(proto mob.Proto, x, y int32, facing float32) *present {
	vid := s.lastVID.Add(1)
	p := &present{
		add: protocol.CharacterAdd{VID: vid, Rotation: facing, X: x, Y: y, Type: entityType(proto.Kind),
			Race: proto.VNum, MoveSpeed: proto.MoveSpeed, AttackSpeed: proto.AttackSpeed},
		info: protocol.CharacterInfo{VID: vid, Name: textToClient(proto.Name)},
	}
	if proto.Fightable() {
		p.fight = &fight{hp: int(proto.MaxHP), maxHP: int(proto.MaxHP), targetedBy: map[*present]struct{}{},
			damageBy: map[*present]int{}, exp: proto.Exp,
			defender: combat.Defender{Level: proto.Level, Vitality: proto.Vitality,
				Armour: proto.Armour}}
	}
	return p
}

// tick takes away the bodies whose time has come, brings back the monsters whose has, and
// walks every living monster whose stroll is due.
func (s *Service) tick(now time.Time) {
	for _, d := range s.presence.takeFallen() {
		s.byPresent[d.p].diedAt = d.at
	}
	for _, m := range s.monsters {
		switch {
		case !m.diedAt.IsZero() && !m.gone && !now.Before(m.diedAt.Add(corpseTime)):
			s.presence.leave(m.p.add.VID)
			m.gone = true
		case m.gone && !now.Before(m.diedAt.Add(respawnAfter)):
			s.place(m, now)
		case m.diedAt.IsZero() && m.proto.Wanders() && !now.Before(m.nextStroll):
			s.stroll(m, now)
		}
	}
}

func (s *Service) stroll(m *monster, now time.Time) {
	x, y := m.homeX+s.spread(wanderRange), m.homeY+s.spread(wanderRange)
	move := protocol.Move{Type: protocol.MoveWait, Rotation: rotationTowards(x-m.x, y-m.y), X: x, Y: y, Time: s.clock.Now()}
	s.presence.move(m.p.add.VID, move)
	m.x, m.y = x, y
	m.nextStroll = now.Add(s.strollWait())
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
