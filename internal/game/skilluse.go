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
	cooled := func() bool { return s.svc.cooldowns.over(s.playing.ID, sk, now) }
	proto, _ := skill.Lookup(sk)
	_, arrows := s.inventory.WornIn(item.Arrow)
	if !known || proto.Shoots() && !arrows {
		s.conn.Log().Debug("skill not used", "vnum", u.VNum)
		return nil
	}
	adds := s.flyAdds
	s.flyAdds = nil // aimed for this use alone, whatever becomes of it
	happened, wait := s.svc.record.useSkill(s.playing.vid, sk, u.Target, now, s.svc.clock.Now(), s.rand, cooled)
	switch happened {
	case skillUsed:
		s.svc.cooldowns.used(s.playing.ID, sk, now.Add(wait))
		if proto.Shoots() {
			s.volley = loose(sk, proto, s.svc.record.levelOf(s.playing.vid, sk), u.Target, adds, now)
		}
	case skillRefused:
		s.conn.Log().Debug("skill not used", "vnum", u.VNum)
	case toggledOff: // its cooldown runs from when it was turned on, as the client counts it
	}
	return nil
}

// outcome is what became of a skill's use.
type outcome int

const (
	skillUsed outcome = iota
	skillRefused
	toggledOff
)

// cooldowns are when each character's skills are ready again. They are kept by the
// server, not the connection, so changing character or logging in again does not end them.
// Each is worked out when the skill is used, as the client fixes its own then, so a casting
// speed lost afterwards does not lengthen it.
type cooldowns struct {
	mu    sync.Mutex
	ready map[cooldown]time.Time
}

type cooldown struct {
	character uint32
	skill     account.Skill
}

// over is whether the character's skill has cooled down by now.
func (c *cooldowns) over(character uint32, sk account.Skill, now time.Time) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return !now.Before(c.ready[cooldown{character, sk}])
}

// used notes the character used the skill, which is ready again at a time.
func (c *cooldowns) used(character uint32, sk account.Skill, ready time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.ready == nil {
		c.ready = map[cooldown]time.Time{}
	}
	c.ready[cooldown{character, sk}] = ready
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
// nothing, as the client turns it off. It reports what became of it.
func (w *record) useSkill(vid uint32, sk account.Skill, target uint32, now time.Time, clock uint32, r *rand.Rand,
	cooled func() bool,
) (outcome, time.Duration) {
	l, unlock := w.lock()
	defer unlock()
	pl := l.all[vid]
	c := pl.char
	level := c.Skills[sk]
	proto, _ := skill.Lookup(sk)
	if pl.dead() || level == 0 {
		return skillRefused, 0
	}
	if _, running := c.Buffs[sk]; running && proto.Toggle {
		l.stopBuff(pl, sk)
		return toggledOff, 0
	}
	victim := l.aimed(pl, target, proto)
	weapon, _ := item.Lookup(c.Weapon)
	cost := proto.Mana.At(skill.Power(level))
	if !cooled() || !proto.Wieldable(weapon) ||
		proto.NeedsVictim() && victim == nil || c.SP < cost {
		return skillRefused, 0
	}
	wait := cooldownOf(proto.CooldownAt(level), castingSpeedOf(c))
	c.SP -= cost
	pl.send(pointTold(vid, protocol.PointSP, -int(cost), c.SP))
	percent := int(proto.Damage.At(skill.Power(level)))
	if proto.Dash && victim != nil {
		l.dash(pl, victim, clock)
	}
	switch {
	case proto.Shoots(): // its arrows land as the client lets them go
	case proto.Ally:
		l.help(pl, l.allyOf(pl, target, proto), sk, proto, level, now, r)
	case victim != nil:
		l.splash(pl, victim, proto, percent, level, now, r)
	case proto.Reach > 0:
		l.splash(pl, pl, proto, percent, level, now, r)
	default:
		l.startBuff(pl, sk, now.Add(proto.LastsAt(level)))
	}
	return skillUsed, wait
}

// aimed is the monster an attack on one lands on: with Target, the one the player picked
// if it lives within a blow's reach; with Aim, the one picked if it lives within Aim, or
// else the nearest living one in front of them within it. Nil for none, and for an attack
// on none.
func (l locked) aimed(pl *present, target uint32, proto skill.Proto) *present {
	if !proto.NeedsVictim() {
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
	return nearest(pl, pl.sees, func(v *present) bool {
		return v.fight != nil && !v.dead() && within(pl, v, reach) && ahead(pl, v)
	})
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
// number, each for percent of a blow's damage, or its splash share of that, through the
// same defence and luck, as many times as it hits while the victim lives. Each it leaves
// alive may be afflicted as the attack afflicts (afflictions).
func (l locked) splash(pl, at *present, proto skill.Proto, percent int, level uint8, now time.Time, r *rand.Rand) {
	victims := l.victimsOf(pl, at, proto)
	a := attackerOf(pl.char)
	for jump, v := range victims {
		share := percent
		if proto.SplashShare > 0 && v != at {
			share = share * proto.SplashShare / 100
		}
		if proto.FromBehind > 0 && !ahead(v, pl) {
			share = share * proto.FromBehind / 100
		}
		blow := blowLanded
		for range max(proto.Hits, 1) {
			var damage int
			if proto.Magic != nil {
				damage = int(proto.Magic(casterOf(pl.char, proto.DiceOf(pl.char.Intelligence), r), int32(skill.Power(level)))) //nolint:gosec // the power is at most 115
				if proto.Chain > 0 {
					damage = damage * (100 - chainLoss*jump) / 100
				}
			} else {
				damage = combat.Damage(a, v.fight.defender, r) * share / 100
			}
			if blow = l.strike(pl, v, damage, now); blow == blowKilled {
				break
			}
		}
		if blow != blowLanded || v.fight.stone {
			continue
		}
		for _, af := range afflictions {
			if chance, has := af.chance(proto, pl.char, level); has && r.IntN(1000) < chance {
				af.put(l, pl, v, level, now)
			}
		}
	}
}

// affliction is a status an attack may put on each monster it leaves alive: whether the
// attack has it and its chance, in thousandths, and what putting it on does.
type affliction struct {
	chance func(proto skill.Proto, c *account.Character, level uint8) (int, bool)
	put    func(l locked, by, v *present, level uint8, now time.Time)
}

// afflictions are rolled in this order, so a fixed seed rolls each the same.
var afflictions = []affliction{
	{ // a stun
		chance: func(p skill.Proto, _ *account.Character, level uint8) (int, bool) {
			return p.StunChance(level), p.Stuns > 0
		},
		put: func(l locked, _, v *present, _ uint8, now time.Time) { l.stun(v, now) },
	},
	{ // a poison
		chance: func(p skill.Proto, _ *account.Character, level uint8) (int, bool) {
			chance := int(p.Poisons.At(skill.Power(level)))
			return chance, chance > 0
		},
		put: func(l locked, by, v *present, _ uint8, now time.Time) { l.poison(v, by, now) },
	},
	{ // a slow
		chance: func(p skill.Proto, _ *account.Character, level uint8) (int, bool) {
			return skill.SlowChance(level), p.Slows
		},
		put: func(l locked, _, v *present, level uint8, now time.Time) {
			l.slow(v, now.Add(skill.SlowsFor(level)))
		},
	},
	{ // a burn
		chance: func(p skill.Proto, c *account.Character, level uint8) (int, bool) {
			return skill.BurnChance(c.Intelligence, level), p.Burns
		},
		put: func(l locked, by, v *present, level uint8, _ time.Time) {
			c := by.char
			l.burn(v, by, max(1, skill.BurnDamage(c.Level, c.Intelligence, level)/burnTurns))
		},
	},
}

// chainLoss is how much weaker, in hundredths, each jump of a chain strikes than the one
// before it: the skilltable's 13.
const chainLoss = 13

// victimsOf is who an attack strikes, in order: the monster aimed at if it is on one; then
// either a chain jumping from each victim to the nearest living monster not yet struck
// within its reach, or every living monster within its reach of the one aimed at, or of
// the player, only those in front of them if it strikes ahead, the nearest first; up to
// its number.
func (l locked) victimsOf(pl, at *present, proto skill.Proto) []*present {
	var victims []*present
	if at != pl {
		victims = append(victims, at)
	}
	limit := proto.Victims
	if limit == 0 {
		limit = len(pl.sees) + 1
	}
	if proto.Chain > 0 {
		for last := at; len(victims) < limit; {
			next := nearest(last, pl.sees, func(v *present) bool {
				return v.fight != nil && !v.dead() && !slices.Contains(victims, v) && within(last, v, proto.Chain)
			})
			if next == nil {
				break
			}
			victims, last = append(victims, next), next
		}
		return victims
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
	return victims[:min(len(victims), limit)]
}

// casterOf is what the player's spells are made of: their character, the weapon's magic
// attack rolled from its least to its most, the spell's dice rolled the same way, and
// their hit rate. Its caller holds the record's lock, under which the character changes.
func casterOf(c *account.Character, dice [2]int32, r *rand.Rand) skill.Caster {
	weapon, _ := item.Lookup(c.Weapon)
	return skill.Caster{Level: int32(c.Level), Strength: int32(c.Strength), Vitality: int32(c.Vitality),
		Dexterity: int32(c.Dexterity), Intelligence: int32(c.Intelligence),
		MTK: roll(int32(weapon.MagicMin), int32(weapon.MagicMax), r), Dice: roll(dice[0], dice[1], r),
		AR: skill.HitRate(c.Dexterity, c.Level)}
}

// roll is a number from least to most, both included.
func roll(least, most int32, r *rand.Rand) int32 {
	if most <= least {
		return least
	}
	return least + r.Int32N(most-least+1)
}

// slowSpeed is the share of its speed a slowed monster keeps: the veteran's 70 in a
// hundred.
const slowSpeed = 70

// slow holds the monster to slowSpeed of its pace until a time, however long it had left,
// and shows everyone who sees it the slow and its pace.
func (l locked) slow(v *present, until time.Time) {
	v.fight.slowUntil = until
	if v.fight.start(slow) {
		v.add.MoveSpeed = uint8(int(v.add.MoveSpeed) * slowSpeed / 100) //nolint:gosec // a share of a u8
		l.showMob(v)
	}
}

// quicken ends the monster's slow, giving back its pace for everyone who sees it.
func (l locked) quicken(v *present, speed uint8) {
	v.add.MoveSpeed = speed
	l.cure(v, slow)
}

// dashStop is how far from its target a dash lands: the original client's warp
// (ProcessMotionEventWarp, ActorInstanceMotionEvent.cpp). It must stay within a blow's
// reach, attackReach, for the strike after it to land.
const dashStop = 270

// dash moves the player to dashStop from the monster, on the line between it and where
// they stood, facing it, for everyone who sees them, at the client clock's time: from
// nearer too, back along that line, as the original's warp does; one standing on it, with
// no line, stays. The player's own client plays the dash itself.
func (l locked) dash(pl, v *present, clock uint32) {
	dx, dy := float64(pl.add.X-v.add.X), float64(pl.add.Y-v.add.Y)
	gap := math.Hypot(dx, dy)
	if gap == 0 {
		return
	}
	x := v.add.X + int32(dx/gap*dashStop)
	y := v.add.Y + int32(dy/gap*dashStop)
	l.shift(pl, protocol.Move{Type: protocol.MoveWait, Rotation: rotationTowards(v.add.X-x, v.add.Y-y), X: x, Y: y, Time: clock})
	pl.tether = tether{x: x, y: y}
}

// stunFor is how long a stun holds a monster: the client's two seconds.
const stunFor = 2 * time.Second

// stun holds the monster where it stands for stunFor from now, however long it had left,
// and shows everyone who sees it the stun.
func (l locked) stun(v *present, now time.Time) {
	v.fight.stunUntil = now.Add(stunFor)
	if v.fight.start(stun) {
		l.showMob(v)
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
// to the world loop to tick.
func (l locked) poison(v, by *present, now time.Time) {
	f := v.fight
	f.poisonUntil = now.Add(poisonFor)
	f.poison = dose{by: by, each: max(1, min(f.maxHP*poisonShare/10000, int(combat.Mean(attackerOf(by.char), f.defender))))}
	if f.start(poison) {
		l.news.poisoned = append(l.news.poisoned, v)
		l.showMob(v)
	}
}

// poisonTick takes the poison's turn off the monster for its poisoner, who is told it, as
// is whoever picked it; one it kills dies at their hand. It reports whether the poison
// runs on; it ends, shown to everyone, once its time is up or its poisoner has left the
// world or died. A poison never provokes. Called by the world loop.
func (w *record) poisonTick(v *present, now time.Time) bool {
	l, unlock := w.lock()
	defer unlock()
	f := v.fight
	if v.dead() || l.all[v.add.VID] != v || !f.has(poison) {
		return false
	}
	b := f.poison
	if !now.Before(f.poisonUntil) || l.all[b.by.add.VID] != b.by || b.by.dead() {
		l.cure(v, poison)
		return false
	}
	return l.turnDamage(v, b.by, b.each, protocol.DamagePoison, now)
}

// allyOf is who a skill for an ally lands on: the player picked, if they live within its
// range and are of the caster's kingdom, else the caster.
func (l locked) allyOf(pl *present, target uint32, proto skill.Proto) *present {
	if v, seen := pl.sees[target]; seen && v.isPlayer() && v.char != nil && !v.dead() &&
		v.info.Empire == pl.info.Empire && within(pl, v, proto.Aim) {
		return v
	}
	return pl
}

// help casts the skill for an ally on them: a heal of its formula's hit points, told to
// them, or its buff, running at the caster's level of it and INT however their own
// differ, renewed if it runs already.
func (l locked) help(pl, to *present, sk account.Skill, proto skill.Proto, level uint8, now time.Time, r *rand.Rand) {
	if proto.Heal != nil {
		gain := proto.Heal(casterOf(pl.char, proto.DiceOf(pl.char.Intelligence), r), int32(skill.Power(level))) //nolint:gosec // the power is at most 115
		points, _ := to.char.Points()
		to.send(restore(to, points.MaxHP, points.MaxSP, uint32(gain), 0)...) //nolint:gosec // a positive formula
		return
	}
	c := to.char
	if c.BuffBy == nil {
		c.BuffBy = map[account.Skill]account.BuffCaster{}
	}
	c.BuffBy[sk] = account.BuffCaster{Level: level, Intelligence: pl.char.Intelligence}
	l.startBuff(to, sk, now.Add(proto.LastsAt(level)))
}

// cooldownOf is a cooldown at a casting speed, shortened as the client shortens it: by
// 10000 over the speed in whole hundredths, so 7 s at 104 is 6.72 s (`PythonPlayerSkill.cpp`).
func cooldownOf(listed time.Duration, speed int) time.Duration {
	return listed * time.Duration(10000/speed) / 100
}

// castingSpeedOf is the speed the character casts at: 100, at which cooldowns run as the
// skill lists them, and what their buffs add, which shortens every cooldown as the client
// counts it (cooldownOf).
func castingSpeedOf(c *account.Character) int {
	return 100 + buffBonus(c, skill.CastingSpeed)
}

// startBuff starts the buff until a time, shows its effect on the player to them and
// everyone who sees them, and leaves its end for the world loop.
func (l locked) startBuff(pl *present, sk account.Skill, until time.Time) {
	c := pl.char
	if c.Buffs == nil {
		c.Buffs = map[account.Skill]time.Time{}
	}
	c.Buffs[sk] = until
	l.showAffects(pl, sk)
	l.news.buffs = append(l.news.buffs, buffEnd{pl.add.VID, sk, until})
	if p, _ := skill.Lookup(sk); p.Drain != (skill.Scaled{}) {
		l.news.drains = append(l.news.drains, buffEnd{pl.add.VID, sk, until})
	}
}

// drainEvery is how often a toggle that drains mana takes it.
const drainEvery = 5 * time.Second

// drain takes the toggle's mana for the five seconds to come, and tells the player, if it
// is still that run; with too little mana left, it ends instead. It reports whether it
// runs on. Called by the world loop.
func (w *record) drain(b buffEnd) bool {
	l, unlock := w.lock()
	defer unlock()
	pl := l.all[b.vid]
	if pl == nil || pl.char == nil || !pl.char.Buffs[b.skill].Equal(b.at) {
		return false
	}
	c := pl.char
	p, _ := skill.Lookup(b.skill)
	cost := p.Drain.At(skill.Power(c.Skills[b.skill]))
	if c.SP < cost {
		l.stopBuff(pl, b.skill)
		return false
	}
	c.SP -= cost
	pl.send(pointTold(b.vid, protocol.PointSP, -int(cost), c.SP))
	return true
}

// endBuff ends the player's buff that was due at a time, if it is still that run: one
// turned off and started again since runs on, and one whose player left is gone.
func (w *record) endBuff(b buffEnd) {
	l, unlock := w.lock()
	defer unlock()
	pl := l.all[b.vid]
	if pl == nil || pl.char == nil || !pl.char.Buffs[b.skill].Equal(b.at) {
		return
	}
	l.stopBuff(pl, b.skill)
}

// stopBuff ends a running buff and takes its effect off the player for everyone.
func (l locked) stopBuff(pl *present, sk account.Skill) {
	delete(pl.char.Buffs, sk)
	delete(pl.char.BuffBy, sk)
	l.showAffects(pl, sk)
}

// stopBuffs ends every buff of a player who died.
func (l locked) stopBuffs(pl *present) {
	for sk := range pl.char.Buffs {
		l.stopBuff(pl, sk)
	}
}

// showAffects tells the player and everyone who sees them the effects on them now, after
// the buff began or ended, at the speeds they now move and strike at, and the player
// their defence or speeds if it raises those.
func (l locked) showAffects(pl *present, sk account.Skill) {
	c := pl.char
	pl.add.MoveSpeed, pl.add.AttackSpeed = moveSpeed(c), attackSpeed(c)
	update := protocol.EncodeCharacterUpdate(pl.info, pl.add, affectsOf(c))
	pl.send(update)
	p, _ := skill.Lookup(sk)
	if p.Raise(skill.Defence) {
		pl.send(pointTold(pl.add.VID, protocol.PointDefence, 0, defence(c)))
	}
	if p.Raise(skill.AttackSpeed) || p.Raise(skill.MoveSpeed) {
		pl.send(pointTold(pl.add.VID, protocol.PointAttackSpeed, 0, pl.add.AttackSpeed),
			pointTold(pl.add.VID, protocol.PointMoveSpeed, 0, pl.add.MoveSpeed))
	}
	if p.Raise(skill.CastingSpeed) {
		pl.send(pointTold(pl.add.VID, protocol.PointCastingSpeed, 0, castingSpeedOf(c)))
	}
	l.toViewers(pl, update)
}

// keepBuffs ends the buffs whose time has come. They are queued in the order they end,
// so only the head is looked at.
func (svc *Service) keepBuffs(now time.Time) {
	for len(svc.buffEnds) > 0 && !now.Before(svc.buffEnds[0].at) {
		svc.record.endBuff(svc.buffEnds[0])
		svc.buffEnds = svc.buffEnds[1:]
	}
}
