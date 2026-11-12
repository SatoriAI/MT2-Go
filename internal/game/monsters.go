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
// in its spawn's area after its spawn's time, or respawnAfter for a spawn that sets none.
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
	pack       *pack     // the monsters at its spawn's centre, itself among them; nil for a stone or one summoned
}

// pack is the monsters standing at one spawn centre: a pack of a kind, and its leader with
// it, or a family's pack. They fight together and come back together.
type pack struct {
	members   []*monster
	calmUntil time.Time    // until its monsters, walking home from a fight, attack no one on sight
	family    *world.Spawn // a family's spawn, which rolls the pack afresh each time it comes back; nil for one of a kind
}

// packReach is how far from a packmate a monster may stand and still join its fight: the
// veteran's, about a pack's spread and a stroll either way.
const packReach = 3000

// down is whether every monster of the pack is dead, and when the last of them died.
func (k *pack) down() (bool, time.Time) {
	var last time.Time
	for _, m := range k.members {
		if m.diedAt.IsZero() {
			return false, time.Time{}
		}
		if m.diedAt.After(last) {
			last = m.diedAt
		}
	}
	return true, last
}

// Run puts the world's mobs in it, then walks the monsters until ctx ends.
func (svc *Service) Run(ctx context.Context) error {
	if err := svc.spawn(svc.now()); err != nil {
		return err
	}
	t := time.NewTicker(tick)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case now := <-t.C:
			svc.tick(now)
		}
	}
}

// spawn puts every spawn's mobs in the world, each at a random point of its area.
func (svc *Service) spawn(now time.Time) error {
	packs := map[[2]int32]*pack{}
	for i := range svc.spawns {
		sp := &svc.spawns[i]
		if len(sp.Kinds) > 0 {
			if err := svc.spawnFamily(sp, now); err != nil {
				return err
			}
			continue
		}
		proto, ok := svc.kind(sp.VNum)
		if !ok {
			return fmt.Errorf("game: spawn of %d, which has no proto", sp.VNum)
		}
		for _, vnum := range proto.Wave {
			if _, ok := svc.kind(vnum); !ok {
				return fmt.Errorf("game: %d calls %d, which has no proto", sp.VNum, vnum)
			}
		}
		for range sp.Count {
			m := svc.spawnOne(proto, sp, now)
			if m == nil || len(sp.Spots) > 0 || proto.Kind == mob.Stone {
				continue
			}
			centre := [2]int32{sp.X, sp.Y}
			if packs[centre] == nil {
				packs[centre] = &pack{}
			}
			m.pack = packs[centre]
			m.pack.members = append(m.pack.members, m)
		}
	}
	return nil
}

// spawnFamily puts a family's pack in the world: room for as many as it may hold, rolled
// as it comes. Its kinds are monsters that wait to be hit: one that attacks on sight would
// have to join the loop's watchers each time the pack is rolled.
func (svc *Service) spawnFamily(sp *world.Spawn, now time.Time) error {
	for _, k := range sp.Kinds {
		proto, ok := svc.kind(k.VNum)
		if !ok {
			return fmt.Errorf("game: pack of %d, which has no proto", k.VNum)
		}
		if !proto.Fightable() || proto.Aggressive || proto.Kind == mob.Stone {
			return fmt.Errorf("game: %d in a family's pack, which takes only monsters that wait to be hit", k.VNum)
		}
	}
	k := &pack{family: sp}
	for range sp.Count {
		m := &monster{spawn: sp, pack: k}
		k.members = append(k.members, m)
		svc.monsters = append(svc.monsters, m)
	}
	svc.placeFamily(k, now)
	return nil
}

// placeFamily puts a family's pack in the world afresh: MinCount to Count of it, each of a
// kind picked by the kinds' weights. Those it leaves out stay away until it comes back.
func (svc *Service) placeFamily(k *pack, now time.Time) {
	sp := k.family
	n := sp.MinCount + svc.rand.IntN(sp.Count-sp.MinCount+1)
	for i, m := range k.members {
		m.proto, _ = svc.kind(pickKind(sp.Kinds, svc.rand)) // one left out still has a kind, whose time it waits
		m.wanders = m.proto.Wanders()
		if i >= n {
			m.diedAt, m.gone = now, true
			continue
		}
		svc.place(m, now)
	}
}

// pickKind picks one of the kinds, each as likely as its weight against the others'.
func pickKind(kinds []world.Kind, r *rand.Rand) uint16 {
	total := 0
	for _, k := range kinds {
		total += k.Weight
	}
	roll := r.IntN(total)
	for _, k := range kinds[:len(kinds)-1] {
		if roll < k.Weight {
			return k.VNum
		}
		roll -= k.Weight
	}
	return kinds[len(kinds)-1].VNum
}

// kind is the proto of vnum, one shared by every monster of it.
func (svc *Service) kind(vnum uint16) (*mob.Proto, bool) {
	if p, ok := svc.kinds[vnum]; ok {
		return p, true
	}
	p, ok := mob.Lookup(vnum)
	if !ok {
		return nil, false
	}
	svc.kinds[vnum] = &p
	return &p, true
}

func (svc *Service) spawnOne(proto *mob.Proto, sp *world.Spawn, now time.Time) *monster {
	// Whatever can be killed is the loop's to take away and bring back, wandering or not.
	if !proto.Fightable() {
		svc.record.enter(svc.newMob(*proto, sp.X+svc.spread(sp.Spread), sp.Y+svc.spread(sp.Spread), sp.Facing))
		return nil
	}
	m := &monster{proto: proto, spawn: sp, wanders: proto.Wanders()}
	svc.monsters = append(svc.monsters, m) // before a stone's wave
	if proto.Aggressive {
		svc.watchers = append(svc.watchers, m)
	}
	svc.place(m, now)
	return m
}

// place puts the monster in the world afresh, with a new vid and all its hit points: at a
// random point of its spawn's area, or at one of its spots, picked afresh. A stone sends
// off the monsters it called before, says it has appeared to everyone on its map, and
// calls its first wave.
func (svc *Service) place(m *monster, now time.Time) {
	x, y := m.spawn.X+svc.spread(m.spawn.Spread), m.spawn.Y+svc.spread(m.spawn.Spread)
	if spots := m.spawn.Spots; len(spots) > 0 {
		spot := spots[svc.rand.IntN(len(spots))]
		x, y = spot.X, spot.Y
	}
	svc.placeAt(m, x, y, now)
	if m.proto.Kind == mob.Stone {
		svc.dismiss(m)
		svc.record.tellMap(m.p, "Pojawił się "+m.proto.Name+".")
		svc.callWave(m, nil, now)
	}
}

// placeAt puts the monster in the world afresh at (x, y), its home from now on.
func (svc *Service) placeAt(m *monster, x, y int32, now time.Time) {
	m.homeX, m.homeY = x, y
	m.x, m.y = m.homeX, m.homeY
	delete(svc.byPresent, m.p)
	m.p = svc.newMob(*m.proto, m.x, m.y, float32(svc.rand.IntN(8)*45))
	m.nextStroll, m.diedAt, m.gone, m.healing = now.Add(svc.strollWait()), time.Time{}, false, false
	svc.byPresent[m.p] = m
	svc.record.enter(m.p)
}

// callWave calls the stone's wave out around it, as many of it as keep the stone's
// monsters alive at waveCap at most, picked at random from the wave when not all of it
// comes: on the player who wore it down, if any, as though each was hit by them.
func (svc *Service) callWave(stone *monster, by *present, now time.Time) {
	called := slices.DeleteFunc(svc.called[stone], func(m *monster) bool { return !m.diedAt.IsZero() || m.gone })
	wave := stone.proto.Wave
	var provoked []provocation
	for _, i := range svc.rand.Perm(len(wave))[:min(len(wave), max(0, waveCap-len(called)))] {
		proto, _ := svc.kind(wave[i])
		m := &monster{proto: proto, wanders: proto.Wanders(), summoned: true}
		angle := svc.rand.Float64() * 2 * math.Pi
		dist := float64(waveFrom + svc.rand.IntN(waveDepth+1))
		svc.placeAt(m, stone.x+int32(dist*math.Cos(angle)), stone.y+int32(dist*math.Sin(angle)), now)
		svc.monsters = append(svc.monsters, m)
		called = append(called, m)
		if by != nil {
			provoked = append(provoked, provocation{m.p, by})
		}
	}
	svc.called[stone] = called
	svc.engage(provoked, now)
}

// dismiss takes away what the stone called out before and is still alive, as it comes
// back elsewhere; the tick then forgets them. One a player killed since the loop last
// took the news is left to die as any monster does, so its drops and body are not lost.
func (svc *Service) dismiss(stone *monster) {
	for _, m := range svc.called[stone] {
		if m.diedAt.IsZero() && !m.gone && svc.record.sendOff(m.p) {
			m.gone, m.chase = true, nil
			svc.forget = true
		}
	}
	delete(svc.called, stone)
}

func (svc *Service) newMob(proto mob.Proto, x, y int32, facing float32) *present {
	vid := svc.lastVID.Add(1)
	p := &present{
		add: protocol.CharacterAdd{VID: vid, Rotation: facing, X: x, Y: y, Type: entityType(proto.Kind),
			Race: proto.VNum, MoveSpeed: proto.MoveSpeed, AttackSpeed: proto.AttackSpeed},
		info: protocol.CharacterInfo{VID: vid, Name: textToClient(proto.Name)},
	}
	if proto.Fightable() {
		p.fight = &fight{hp: int(proto.MaxHP), maxHP: int(proto.MaxHP), targetedBy: map[*present]struct{}{},
			damageBy: map[*present]int{}, exp: proto.Exp, stone: proto.Kind == mob.Stone, animal: proto.Animal,
			defender: combat.Defender{Level: proto.Level, Vitality: proto.Vitality,
				Armour: uint16(proto.Armour)}}
	}
	return p
}

// tick takes away the bodies whose time has come, brings back the monsters whose has,
// walks every living monster whose stroll is due, has those that were hit fight back, and
// heals the hurt.
func (svc *Service) tick(now time.Time) {
	n := svc.record.takeNews()
	for _, t := range n.laid {
		svc.vanishing = inOrder(svc.vanishing, t)
	}
	for _, b := range n.buffs {
		svc.buffEnds = inOrder(svc.buffEnds, b)
	}
	for _, d := range n.fallen {
		m := svc.byPresent[d.p]
		m.diedAt, m.chase = d.at, nil
		svc.dropFrom(m, d, now)
	}
	for _, p := range n.poisoned {
		svc.poisons.add(p, now.Add(poisonEvery))
	}
	for _, p := range n.burned {
		svc.burnings.add(p, now.Add(burnEvery))
	}
	for _, d := range n.drains {
		svc.drainings.add(d, now.Add(drainEvery))
	}
	for _, c := range n.calls {
		if stone := svc.byPresent[c.stone]; stone != nil && stone.diedAt.IsZero() {
			svc.callWave(stone, c.by, now)
		}
	}
	svc.keepGround(now)
	svc.keepBuffs(now)
	svc.engage(n.provoked, now)
	svc.lure(n.lured, now)
	svc.timings = append(svc.timings, n.timed...)
	svc.keepTimed(now)
	svc.lookOut(now)
	for _, m := range svc.monsters {
		switch {
		case m.gone: // a body taken away, or one a stone dismissed while still alive
			if !m.summoned {
				svc.bringBack(m, now)
			}
		case !m.diedAt.IsZero():
			if !now.Before(m.diedAt.Add(corpseTime)) {
				svc.record.leave(m.p.add.VID)
				m.gone = true
				svc.forget = svc.forget || m.summoned
			}
		case m.wanders && m.chase == nil && !now.Before(m.nextStroll):
			svc.stroll(m, now)
		}
	}
	if svc.forget {
		svc.monsters = slices.DeleteFunc(svc.monsters, func(m *monster) bool {
			if m.gone && m.summoned {
				delete(svc.byPresent, m.p)
				return true
			}
			return false
		})
		svc.forget = false
	}
	svc.fightOn(now)
	svc.poisons.run(now, poisonEvery, func(p *present) bool { return svc.record.poisonTick(p, now) })
	svc.burnings.run(now, burnEvery, func(p *present) bool { return svc.record.burnTick(p, now) })
	svc.drainings.run(now, drainEvery, svc.record.drain)
	svc.healMonsters(now)
	switch {
	case svc.nextPlayerRegen.IsZero():
		svc.nextPlayerRegen = now.Add(playerRegenEvery)
	case !now.Before(svc.nextPlayerRegen):
		svc.record.regenPlayers(now)
		svc.nextPlayerRegen = now.Add(playerRegenEvery)
	}
}

// bringBack puts a monster whose body is gone back in the world once its time has come:
// its own time after its death, or, for one of a pack, its time after the last of the pack
// died, the whole pack together.
func (svc *Service) bringBack(m *monster, now time.Time) {
	if m.pack == nil {
		if !now.Before(m.diedAt.Add(m.respawnAfter())) {
			svc.place(m, now)
		}
		return
	}
	if down, last := m.pack.down(); down && !now.Before(last.Add(m.respawnAfter())) {
		if m.pack.family != nil {
			svc.placeFamily(m.pack, now)
			return
		}
		for _, mate := range m.pack.members {
			svc.place(mate, now)
		}
	}
}

// respawnAfter is how long after its death the monster comes back: a stone's own time, its
// spawn's, set by how far its band lies from the village, or else the fields' default.
func (m *monster) respawnAfter() time.Duration {
	switch {
	case m.proto.Respawn > 0:
		return m.proto.Respawn
	case m.spawn.Respawn > 0:
		return m.spawn.Respawn
	}
	return respawnAfter
}

func (svc *Service) stroll(m *monster, now time.Time) {
	x, y := m.homeX+svc.spread(wanderRange), m.homeY+svc.spread(wanderRange)
	move := protocol.Move{Type: protocol.MoveWait, Rotation: rotationTowards(x-m.x, y-m.y), X: x, Y: y, Time: svc.clock.Now()}
	svc.record.move(m.p.add.VID, move)
	m.x, m.y = x, y
	m.nextStroll = now.Add(svc.strollWait())
}

// spread is a random offset of at most r either way.
func (svc *Service) spread(r int32) int32 {
	if r <= 0 {
		return 0
	}
	return svc.rand.Int32N(2*r+1) - r
}

func (svc *Service) strollWait() time.Duration {
	return wanderMin + time.Duration(svc.rand.Int64N(int64(wanderMax-wanderMin)))
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
