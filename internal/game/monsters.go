package game

import (
	"context"
	"fmt"
	"math"
	"math/rand/v2"
	"slices"
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

// A stone's wave stands waveFrom to waveFrom+waveDepth from it.
const (
	waveFrom  = 500
	waveDepth = 300
)

// monster is one the world loop walks, if it wanders, and brings back after it dies. Only the loop moves
// it, so where it stands here is where the world record has it.
type monster struct {
	// proto is its kind, shared by the monsters of its kind: a copy in each would make the
	// monster too big for the allocator's 176-byte class, and the tick, which walks them
	// all, slower (docs/decisions/world-loop.md). The spawn is shared the same way.
	proto      *mob.Proto
	wanders    bool         // the proto's, kept here: the tick asks it of every monster ten times a second
	gone       bool         // its body has been taken away
	healing    bool         // it is on the loop's list of the hurt
	summoned   bool         // a stone called it out, so it never comes back
	spawn      *world.Spawn // nil for one summoned
	p          *present
	homeX      int32
	homeY      int32
	x, y       int32
	nextStroll time.Time
	diedAt     time.Time // zero while alive
	chase      *chase    // its fight with a player who hit it; nil when it is not fighting
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
	for i := range s.spawns {
		sp := &s.spawns[i]
		proto, ok := s.kind(sp.VNum)
		if !ok {
			return fmt.Errorf("game: spawn of %d, which has no proto", sp.VNum)
		}
		for _, vnum := range proto.Wave {
			if _, ok := s.kind(vnum); !ok {
				return fmt.Errorf("game: %d calls %d, which has no proto", sp.VNum, vnum)
			}
		}
		for range sp.Count {
			s.spawnOne(proto, sp, now)
		}
	}
	return nil
}

// kind is the proto of vnum, one shared by every monster of it.
func (s *Service) kind(vnum uint16) (*mob.Proto, bool) {
	if p, ok := s.kinds[vnum]; ok {
		return p, true
	}
	p, ok := mob.Lookup(vnum)
	if !ok {
		return nil, false
	}
	s.kinds[vnum] = &p
	return &p, true
}

func (s *Service) spawnOne(proto *mob.Proto, sp *world.Spawn, now time.Time) {
	// Whatever can be killed is the loop's to take away and bring back, wandering or not.
	if !proto.Fightable() {
		s.presence.enter(s.newMob(*proto, sp.X+s.spread(sp.Spread), sp.Y+s.spread(sp.Spread), 0))
		return
	}
	m := &monster{proto: proto, spawn: sp, wanders: proto.Wanders()}
	s.monsters = append(s.monsters, m) // before a stone's wave
	s.place(m, now)
}

// place puts the monster in the world afresh, with a new vid and all its hit points: at a
// random point of its spawn's area, or at one of its spots, picked afresh. A stone sends
// off the monsters it called before, says it has appeared to everyone on its map, and
// calls its first wave.
func (s *Service) place(m *monster, now time.Time) {
	x, y := m.spawn.X+s.spread(m.spawn.Spread), m.spawn.Y+s.spread(m.spawn.Spread)
	if spots := m.spawn.Spots; len(spots) > 0 {
		spot := spots[s.rand.IntN(len(spots))]
		x, y = spot.X, spot.Y
	}
	s.placeAt(m, x, y, now)
	if m.proto.Kind == mob.Stone {
		s.dismiss(m, now)
		s.presence.tellMap(m.p, "Pojawił się "+m.proto.Name+".")
		s.callWave(m, nil, now)
	}
}

// placeAt puts the monster in the world afresh at (x, y), its home from now on.
func (s *Service) placeAt(m *monster, x, y int32, now time.Time) {
	m.homeX, m.homeY = x, y
	m.x, m.y = m.homeX, m.homeY
	delete(s.byPresent, m.p)
	m.p = s.newMob(*m.proto, m.x, m.y, float32(s.rand.IntN(8)*45))
	m.nextStroll, m.diedAt, m.gone, m.healing = now.Add(s.strollWait()), time.Time{}, false, false
	s.byPresent[m.p] = m
	s.presence.enter(m.p)
}

// callWave calls the stone's wave out around it, as many of it as keep the stone's
// monsters alive at waveCap at most, picked at random from the wave when not all of it
// comes: on the player who wore it down, if any, as though each was hit by them.
func (s *Service) callWave(stone *monster, by *present, now time.Time) {
	called := slices.DeleteFunc(s.called[stone], func(m *monster) bool { return !m.diedAt.IsZero() || m.gone })
	wave := stone.proto.Wave
	var provoked []provocation
	for _, i := range s.rand.Perm(len(wave))[:min(len(wave), max(0, waveCap-len(called)))] {
		proto, _ := s.kind(wave[i])
		m := &monster{proto: proto, wanders: proto.Wanders(), summoned: true}
		angle := s.rand.Float64() * 2 * math.Pi
		dist := float64(waveFrom + s.rand.IntN(waveDepth+1))
		s.placeAt(m, stone.x+int32(dist*math.Cos(angle)), stone.y+int32(dist*math.Sin(angle)), now)
		s.monsters = append(s.monsters, m)
		called = append(called, m)
		if by != nil {
			provoked = append(provoked, provocation{m.p, by})
		}
	}
	s.called[stone] = called
	s.engage(provoked, now)
}

// dismiss takes away what the stone called out before and is still alive, as it comes
// back elsewhere; the tick then forgets them. One a player killed since the loop last
// took the news is left to die as any monster does, so its drops and body are not lost.
func (s *Service) dismiss(stone *monster, now time.Time) {
	for _, m := range s.called[stone] {
		if m.diedAt.IsZero() && !m.gone && s.presence.sendOff(m.p) {
			m.gone, m.diedAt, m.chase = true, now, nil // diedAt, so nothing later this tick strolls it
			s.forget = true
		}
	}
	delete(s.called, stone)
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
			damageBy: map[*present]int{}, exp: proto.Exp, stone: proto.Kind == mob.Stone,
			defender: combat.Defender{Level: proto.Level, Vitality: proto.Vitality,
				Armour: uint16(proto.Armour)}}
	}
	return p
}

// tick takes away the bodies whose time has come, brings back the monsters whose has,
// walks every living monster whose stroll is due, has those that were hit fight back, and
// heals the hurt.
func (s *Service) tick(now time.Time) {
	n := s.presence.takeNews()
	for _, t := range n.laid {
		s.vanishing = inOrder(s.vanishing, t)
	}
	for _, b := range n.buffs {
		s.buffEnds = inOrder(s.buffEnds, b)
	}
	for _, d := range n.fallen {
		m := s.byPresent[d.p]
		m.diedAt, m.chase = d.at, nil
		s.dropFrom(m, d, now)
	}
	for _, p := range n.poisoned {
		s.poisons = append(s.poisons, poisoning{p, now.Add(poisonEvery)})
	}
	for _, c := range n.calls {
		if stone := s.byPresent[c.stone]; stone != nil && stone.diedAt.IsZero() {
			s.callWave(stone, c.by, now)
		}
	}
	s.keepGround(now)
	s.keepBuffs(now)
	s.engage(n.provoked, now)
	for _, m := range s.monsters {
		switch {
		case !m.diedAt.IsZero() && !m.gone && !now.Before(m.diedAt.Add(corpseTime)):
			s.presence.leave(m.p.add.VID)
			m.gone = true
			s.forget = s.forget || m.summoned
		case m.gone && !m.summoned && !now.Before(m.diedAt.Add(m.respawnAfter())):
			s.place(m, now)
		case m.diedAt.IsZero() && m.wanders && m.chase == nil && !now.Before(m.nextStroll):
			s.stroll(m, now)
		}
	}
	if s.forget {
		s.monsters = slices.DeleteFunc(s.monsters, func(m *monster) bool {
			if m.gone && m.summoned {
				delete(s.byPresent, m.p)
				return true
			}
			return false
		})
		s.forget = false
	}
	s.fightOn(now)
	s.tickPoisons(now)
	s.healMonsters(now)
	switch {
	case s.nextPlayerRegen.IsZero():
		s.nextPlayerRegen = now.Add(playerRegenEvery)
	case !now.Before(s.nextPlayerRegen):
		s.presence.regenPlayers(now)
		s.nextPlayerRegen = now.Add(playerRegenEvery)
	}
}

// respawnAfter is how long after its death the monster comes back: a stone's own time, or
// the village fields'.
func (m *monster) respawnAfter() time.Duration {
	if m.proto.Respawn > 0 {
		return m.proto.Respawn
	}
	return respawnAfter
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
