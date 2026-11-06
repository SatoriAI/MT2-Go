package game

import (
	"math/rand/v2"
	"sync"
	"time"

	"github.com/SatoriAI/MT2-Go/internal/account"
	"github.com/SatoriAI/MT2-Go/internal/combat"
	"github.com/SatoriAI/MT2-Go/internal/item"
	"github.com/SatoriAI/MT2-Go/internal/protocol"
	"github.com/SatoriAI/MT2-Go/internal/skill"
)

// onUseSkill uses the skill the client names by its number, once its cooldown is over.
// The world record decides the rest. The client checks the same rules itself first, so a
// skill refused here is dropped without a word.
func (s *session) onUseSkill(body []byte) error {
	u, err := protocol.DecodeUseSkill(body)
	if err != nil {
		return err
	}
	sk, known := skillByVNum(int(u.VNum))
	now := s.svc.now()
	p, _ := skill.Lookup(sk)
	if !known || !s.svc.cooldowns.over(s.playing.ID, sk, now, p.Cooldown) {
		s.conn.Log().Debug("skill not used", "vnum", u.VNum)
		return nil
	}
	if s.svc.presence.useSkill(s.playing.vid, sk, now, s.rand) {
		s.svc.cooldowns.used(s.playing.ID, sk, now)
	}
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

// useSkill uses the skill for the player, if they are alive, have learnt it, hold the
// weapon it needs and have its mana: the mana is spent and told, then an attack strikes every living monster in its
// reach, and a buff starts. A buff already running is ended instead, for nothing, as the
// client's toggle turns it off. It reports whether the skill was used.
func (w *presence) useSkill(vid uint32, sk account.Skill, now time.Time, r *rand.Rand) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	pl := w.all[vid]
	c := pl.char
	level := c.Skills[sk]
	proto, _ := skill.Lookup(sk)
	if pl.dead() || level == 0 {
		return false
	}
	if weapon, _ := item.Lookup(c.Weapon); proto.NeedsSword && !weapon.Sword {
		return false
	}
	if _, running := c.Buffs[sk]; running {
		w.endBuffLocked(pl, sk)
		return true
	}
	cost := proto.Mana.At(skill.Power(level))
	if c.SP < cost {
		return false
	}
	c.SP -= cost
	pl.send(protocol.EncodePointChange(vid, protocol.PointSP, -int32(cost), int32(c.SP))) //nolint:gosec // small
	switch {
	case proto.Reach > 0:
		w.swirl(pl, proto, level, now, r)
	case proto.Lasts > 0:
		w.startBuff(pl, sk, now.Add(proto.Lasts))
	}
	return true
}

// swirl strikes every living monster the player sees within the skill's reach, each for a
// blow's damage times the skill's, through the same defence and luck. Called under the
// lock.
func (w *presence) swirl(pl *present, proto skill.Proto, level uint8, now time.Time, r *rand.Rand) {
	a := attackerOf(pl.char, now)
	percent := proto.Damage.At(skill.Power(level))
	for _, v := range pl.sees {
		if v.fight == nil || v.dead() || !within(pl, v, proto.Reach) {
			continue
		}
		w.strike(pl, v, combat.Damage(a, v.fight.defender, r)*int(percent)/100, now)
	}
}

// startBuff starts the buff until a time, shows its effect on the player to them and
// everyone who sees them, and leaves its end for the world loop. Called under the lock.
func (w *presence) startBuff(pl *present, sk account.Skill, until time.Time) {
	c := pl.char
	if c.Buffs == nil {
		c.Buffs = map[account.Skill]time.Time{}
	}
	c.Buffs[sk] = until
	w.showAffects(pl)
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
	w.showAffects(pl)
}

// endBuffs ends every buff of a player who died. Called under the lock.
func (w *presence) endBuffs(pl *present) {
	for sk := range pl.char.Buffs {
		w.endBuffLocked(pl, sk)
	}
}

// showAffects tells the player and everyone who sees them the effects on them now.
// Called under the lock.
func (w *presence) showAffects(pl *present) {
	update := protocol.EncodeCharacterUpdate(pl.info, pl.add, affectsOf(pl.char))
	pl.send(update)
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
