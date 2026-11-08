package game

import (
	"cmp"
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
	if !known || !s.svc.presence.useSkill(s.playing.vid, sk, u.Target, now, s.rand, cooled) {
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
func (w *presence) useSkill(vid uint32, sk account.Skill, target uint32, now time.Time, r *rand.Rand,
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
	victim, inReach := pl.sees[target]
	inReach = inReach && victim.fight != nil && !victim.dead() && within(pl, victim, attackReach)
	weapon, _ := item.Lookup(c.Weapon)
	cost := proto.Mana.At(skill.Power(level))
	if !cooled(proto.CooldownAt(level)) || !proto.Wieldable(weapon) || proto.Target && !inReach || c.SP < cost {
		return false
	}
	c.SP -= cost
	pl.send(protocol.EncodePointChange(vid, protocol.PointSP, -int32(cost), int32(c.SP))) //nolint:gosec // small
	percent := int(proto.Damage.At(skill.Power(level)))
	switch {
	case proto.Target:
		w.splash(pl, victim, proto, percent, now, r)
	case proto.Reach > 0:
		w.splash(pl, pl, proto, percent, now, r)
	default:
		w.startBuff(pl, sk, now.Add(proto.LastsAt(level)))
	}
	return true
}

// splash strikes the attack's victims around at: the monster itself if the attack is on
// one, and every living monster the player sees within its reach of it, the nearest first
// up to its number, each for percent of a blow's damage through the same defence and luck,
// as many times as it hits while the victim lives. Called under the lock.
func (w *presence) splash(pl, at *present, proto skill.Proto, percent int, now time.Time, r *rand.Rand) {
	var victims []*present
	if at != pl {
		victims = append(victims, at)
	}
	if proto.Reach > 0 {
		var near []*present
		for _, v := range pl.sees {
			if v != at && v.fight != nil && !v.dead() && within(at, v, proto.Reach) {
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
		for range max(proto.Hits, 1) {
			if w.strike(pl, v, combat.Damage(a, v.fight.defender, r)*percent/100, now) == blowKilled {
				break
			}
		}
	}
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
