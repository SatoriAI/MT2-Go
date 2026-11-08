package game

import (
	"cmp"
	"math"
	"math/rand/v2"
	"slices"
	"sync"
	"time"

	"github.com/SatoriAI/MT2-Go/internal/account"
	"github.com/SatoriAI/MT2-Go/internal/combat"
	"github.com/SatoriAI/MT2-Go/internal/item"
	"github.com/SatoriAI/MT2-Go/internal/protocol"
	"github.com/SatoriAI/MT2-Go/internal/skill"
)

// onUseSkill uses the skill the client names by its number, on the monster it names for
// one that strikes one, once its cooldown is over. The world record decides the rest. The
// client checks the same rules itself first, so a skill refused here is dropped without a
// word.
func (s *session) onUseSkill(body []byte) error {
	u, err := protocol.DecodeUseSkill(body)
	if err != nil {
		return err
	}
	sk, known := skillByVNum(int(u.VNum))
	now := s.svc.now()
	cooled := func(wait time.Duration) bool { return s.svc.cooldowns.over(s.playing.ID, sk, now, wait) }
	if !known || !s.svc.presence.useSkill(s.playing.vid, sk, u.Target, now, s.svc.clock.Now(), s.rand, cooled) {
		s.conn.Log().Debug("skill not used", "vnum", u.VNum)
		return nil
	}
	s.svc.cooldowns.used(s.playing.ID, sk, now)
	return nil
}

// cooldowns are when each character last used each skill. They are kept by the server,
// not the connection, so changing character or logging in again does not end them.
type cooldowns struct {
	mu   sync.Mutex
	last map[cooldown]time.Time
}

type cooldown struct {
	character uint32
	skill     account.Skill
}

// over is whether the character's skill has cooled down by now.
func (c *cooldowns) over(character uint32, sk account.Skill, now time.Time, wait time.Duration) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return !now.Before(c.last[cooldown{character, sk}].Add(wait))
}

// used notes the character used the skill now.
func (c *cooldowns) used(character uint32, sk account.Skill, now time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.last == nil {
		c.last = map[cooldown]time.Time{}
	}
	c.last[cooldown{character, sk}] = now
}

// buffEnd is when a buff a player started is due to end, for the world loop.
type buffEnd struct {
	vid   uint32
	skill account.Skill
	at    time.Time
}

func (b buffEnd) due() time.Time { return b.at }

// useSkill uses the skill for the player, if they are alive, have learnt it, it has
// cooled down from its last use at its level, they hold the weapon it needs, the monster
// it strikes alone is in reach, and they have its mana: the mana is spent and told, then
// an attack strikes and a buff starts. A toggle already running is ended instead, for
// nothing, as the client turns it off. It reports whether the skill was used.
func (w *presence) useSkill(vid uint32, sk account.Skill, target uint32, now time.Time, clock uint32, r *rand.Rand,
	cooled func(time.Duration) bool,
) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	pl := w.all[vid]
	c := pl.char
	level := c.Skills[sk]
	proto, _ := skill.Lookup(sk)
	if pl.dead() || level == 0 {
		return false
	}
	if _, running := c.Buffs[sk]; running && proto.Toggle {
		w.endBuffLocked(pl, sk)
		return true
	}
	victim := aimed(pl, target, proto)
	weapon, _ := item.Lookup(c.Weapon)
	cost := proto.Mana.At(skill.Power(level))
	if !cooled(proto.CooldownAt(level)) || !proto.Wieldable(weapon) || (proto.Target || proto.Aim > 0) && victim == nil || c.SP < cost {
		return false
	}
	c.SP -= cost
	pl.send(protocol.EncodePointChange(vid, protocol.PointSP, -int32(cost), int32(c.SP))) //nolint:gosec // small
	percent := int(proto.Damage.At(skill.Power(level)))
	if proto.Dash && victim != nil {
		w.dash(pl, victim, clock)
	}
	switch {
	case victim != nil:
		w.splash(pl, victim, proto, percent, level, now, r)
	case proto.Reach > 0:
		w.splash(pl, pl, proto, percent, level, now, r)
	default:
		w.startBuff(pl, sk, now.Add(proto.LastsAt(level)))
	}
	return true
}

// aimed is the monster an attack on one lands on: with Target, the one the player picked
// if it lives within a blow's reach; with Aim, the one picked if it lives within Aim, or
// else the nearest living one in front of them within it. Nil for none, and for an attack
// on none. Called under the lock.
func aimed(pl *present, target uint32, proto skill.Proto) *present {
	if !proto.Target && proto.Aim == 0 {
		return nil
	}
	reach := int64(attackReach)
	if proto.Aim > 0 {
		reach = proto.Aim
	}
	if v, seen := pl.sees[target]; seen && v.fight != nil && !v.dead() && within(pl, v, reach) {
		return v
	}
	if proto.Target {
		return nil
	}
	var nearest *present
	for _, v := range pl.sees {
		if v.fight != nil && !v.dead() && within(pl, v, reach) && ahead(pl, v) &&
			(nearest == nil || distance(pl, v) < distance(pl, nearest)) {
			nearest = v
		}
	}
	return nearest
}

// ahead is whether o stands in the half of the world in front of p, by the way p last
// faced: the client's degrees clockwise from the world's +y.
func ahead(p, o *present) bool {
	facing := float64(p.add.Rotation) * math.Pi / 180
	return float64(o.add.X-p.add.X)*math.Sin(facing)+float64(o.add.Y-p.add.Y)*math.Cos(facing) >= 0
}

// splash strikes the attack's victims around at: the monster itself if the attack is on
// one, and every living monster the player sees within its reach of it, only those in
// front of the player if the attack strikes ahead of them, the nearest first up to its
// number, each for percent of a blow's damage through the same defence and luck, as many
// times as it hits while the victim lives. A stunning attack may stun each it leaves
// alive. Called under the lock.
func (w *presence) splash(pl, at *present, proto skill.Proto, percent int, level uint8, now time.Time, r *rand.Rand) {
	var victims []*present
	if at != pl {
		victims = append(victims, at)
	}
	if proto.Reach > 0 {
		var near []*present
		for _, v := range pl.sees {
			if v != at && v.fight != nil && !v.dead() && within(at, v, proto.Reach) && (!proto.Front || ahead(pl, v)) {
				near = append(near, v)
			}
		}
		slices.SortFunc(near, func(a, b *present) int { return cmp.Compare(distance(at, a), distance(at, b)) })
		victims = append(victims, near...)
	}
	if proto.Victims > 0 {
		victims = victims[:min(len(victims), proto.Victims)]
	}
	a := attackerOf(pl.char)
	for _, v := range victims {
		share := percent
		if proto.FromBehind > 0 && !ahead(v, pl) {
			share = share * proto.FromBehind / 100
		}
		blow := blowLanded
		for range max(proto.Hits, 1) {
			if blow = w.strike(pl, v, combat.Damage(a, v.fight.defender, r)*share/100, now); blow == blowKilled {
				break
			}
		}
		if blow != blowLanded || v.fight.stone {
			continue
		}
		if proto.Stuns && r.IntN(1000) < skill.StunChance(level) {
			w.stun(v, now)
		}
		if proto.Poisons && r.IntN(1000) < skill.PoisonChance(level) {
			w.poison(v, pl, now)
		}
	}
}

// dashStop is how far from its target a dash lands: the original client's warp
// (ProcessMotionEventWarp, ActorInstanceMotionEvent.cpp). It must stay within a blow's
// reach, attackReach, for the strike after it to land.
const dashStop = 270

// dash moves the player to dashStop from the monster, on the line between it and where
// they stood, facing it, for everyone who sees them, at the client clock's time; one already
// that near stays. The player's own client plays the dash itself. Called under the lock.
func (w *presence) dash(pl, v *present, clock uint32) {
	dx, dy := float64(pl.add.X-v.add.X), float64(pl.add.Y-v.add.Y)
	gap := math.Hypot(dx, dy)
	if gap <= dashStop {
		return
	}
	x := v.add.X + int32(dx/gap*dashStop)
	y := v.add.Y + int32(dy/gap*dashStop)
	w.moveLocked(pl, protocol.Move{Type: protocol.MoveWait, Rotation: rotationTowards(v.add.X-x, v.add.Y-y), X: x, Y: y, Time: clock})
}

// stunFor is how long a stun holds a monster: the client's two seconds.
const stunFor = 2 * time.Second

// stun holds the monster where it stands for stunFor from now, however long it had left,
// and shows everyone who sees it the stun. Called under the lock.
func (w *presence) stun(v *present, now time.Time) {
	v.fight.stunnedUntil = now.Add(stunFor)
	if !v.fight.stunned {
		v.fight.stunned = true
		toViewers(v, protocol.EncodeCharacterUpdate(v.info, v.add, mobAffects(v)))
	}
}

// Poison takes poisonShare of a monster's hit points every poisonEvery, at most the
// poisoner's mean blow on it, for poisonFor: the veteran's numbers.
const (
	poisonEvery = 2 * time.Second
	poisonFor   = 30 * time.Second
	poisonShare = 100 // in hundredths of a percent: 1%
)

// poison sets the poisoner's poison on the monster for poisonFor from now, however long
// the last had left, and shows it to everyone who sees it; one newly poisoned is handed
// to the world loop to tick. Called under the lock.
func (w *presence) poison(v, by *present, now time.Time) {
	f := v.fight
	f.poisonedUntil, f.poisoner = now.Add(poisonFor), by
	f.poison = max(1, min(f.maxHP*poisonShare/10000, int(combat.Mean(attackerOf(by.char), f.defender))))
	if !f.poisoned {
		f.poisoned = true
		w.poisoned = append(w.poisoned, v)
		toViewers(v, protocol.EncodeCharacterUpdate(v.info, v.add, mobAffects(v)))
	}
}

// poisonTick takes the poison's turn off the monster for its poisoner, who is told it, as
// is whoever picked it; one it kills dies at their hand. It reports whether the poison
// runs on; it ends, shown to everyone, once its time is up or its poisoner has left the
// world or died. A poison never provokes. Called by the world loop.
func (w *presence) poisonTick(v *present, now time.Time) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	f := v.fight
	if v.dead() || w.all[v.add.VID] != v || !f.poisoned {
		return false
	}
	by := f.poisoner
	if !now.Before(f.poisonedUntil) || w.all[by.add.VID] != by || by.dead() {
		f.poisoned = false
		toViewers(v, protocol.EncodeCharacterUpdate(v.info, v.add, mobAffects(v)))
		return false
	}
	damage := min(f.poison, f.hp)
	f.hp -= damage
	f.damageBy[by] += damage
	by.send(protocol.EncodeDamage(v.add.VID, protocol.DamagePoison, int32(damage))) //nolint:gosec // at most the hit points
	for t := range f.targetedBy {
		t.send(protocol.EncodeTarget(v.add.VID, f.hpPercent()))
	}
	if f.hp == 0 {
		w.kill(by, v, now)
		return false
	}
	return true
}

// poisoning is a poisoned monster the world loop ticks, and when it ticks next.
type poisoning struct {
	p    *present
	next time.Time
}

// tickPoisons takes the turn of each poison whose time has come, and drops those that
// ended. Only the poisoned are walked, so an idle tick costs nothing more.
func (s *Service) tickPoisons(now time.Time) {
	kept := s.poisons[:0]
	for _, ps := range s.poisons {
		if now.Before(ps.next) {
			kept = append(kept, ps)
			continue
		}
		if s.presence.poisonTick(ps.p, now) {
			kept = append(kept, poisoning{ps.p, now.Add(poisonEvery)})
		}
	}
	clear(s.poisons[len(kept):])
	s.poisons = kept
}

// wake ends the monster's stun for everyone who sees it. Called under the lock.
func (w *presence) wake(v *present) {
	v.fight.stunned = false
	toViewers(v, protocol.EncodeCharacterUpdate(v.info, v.add, mobAffects(v)))
}

// distance is how far apart two characters stand, squared.
func distance(p, o *present) int64 {
	dx, dy := int64(p.add.X)-int64(o.add.X), int64(p.add.Y)-int64(o.add.Y)
	return dx*dx + dy*dy
}

// startBuff starts the buff until a time, shows its effect on the player to them and
// everyone who sees them, and leaves its end for the world loop. Called under the lock.
func (w *presence) startBuff(pl *present, sk account.Skill, until time.Time) {
	c := pl.char
	if c.Buffs == nil {
		c.Buffs = map[account.Skill]time.Time{}
	}
	c.Buffs[sk] = until
	w.showAffects(pl, sk)
	w.buffs = append(w.buffs, buffEnd{pl.add.VID, sk, until})
}

// endBuff ends the player's buff that was due at a time, if it is still that run: one
// turned off and started again since runs on, and one whose player left is gone.
func (w *presence) endBuff(b buffEnd) {
	w.mu.Lock()
	defer w.mu.Unlock()
	pl := w.all[b.vid]
	if pl == nil || pl.char == nil || !pl.char.Buffs[b.skill].Equal(b.at) {
		return
	}
	w.endBuffLocked(pl, b.skill)
}

// endBuffLocked ends a running buff and takes its effect off the player for everyone.
// Called under the lock.
func (w *presence) endBuffLocked(pl *present, sk account.Skill) {
	delete(pl.char.Buffs, sk)
	w.showAffects(pl, sk)
}

// endBuffs ends every buff of a player who died. Called under the lock.
func (w *presence) endBuffs(pl *present) {
	for sk := range pl.char.Buffs {
		w.endBuffLocked(pl, sk)
	}
}

// showAffects tells the player and everyone who sees them the effects on them now, after
// the buff began or ended, at the speeds they now move and strike at, and the player
// their defence or speeds if it raises those. Called under the lock.
func (w *presence) showAffects(pl *present, sk account.Skill) {
	c := pl.char
	pl.add.MoveSpeed, pl.add.AttackSpeed = moveSpeed(c), attackSpeed(c)
	update := protocol.EncodeCharacterUpdate(pl.info, pl.add, affectsOf(c))
	pl.send(update)
	p, _ := skill.Lookup(sk)
	if p.Raise(skill.Defence) {
		pl.send(protocol.EncodePointChange(pl.add.VID, protocol.PointDefence, 0, int32(defence(c)))) //nolint:gosec // small
	}
	if p.Raise(skill.AttackSpeed) || p.Raise(skill.MoveSpeed) {
		pl.send(protocol.EncodePointChange(pl.add.VID, protocol.PointAttackSpeed, 0, int32(pl.add.AttackSpeed)),
			protocol.EncodePointChange(pl.add.VID, protocol.PointMoveSpeed, 0, int32(pl.add.MoveSpeed)))
	}
	toViewers(pl, update)
}

// keepBuffs ends the buffs whose time has come. They are queued in the order they end,
// so only the head is looked at.
func (s *Service) keepBuffs(now time.Time) {
	for len(s.buffEnds) > 0 && !now.Before(s.buffEnds[0].at) {
		s.presence.endBuff(s.buffEnds[0])
		s.buffEnds = s.buffEnds[1:]
	}
}
