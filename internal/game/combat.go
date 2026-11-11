package game

import (
	"math"
	"math/rand/v2"
	"time"

	"github.com/SatoriAI/MT2-Go/internal/account"
	"github.com/SatoriAI/MT2-Go/internal/combat"
	"github.com/SatoriAI/MT2-Go/internal/item"
	"github.com/SatoriAI/MT2-Go/internal/protocol"
	"github.com/SatoriAI/MT2-Go/internal/skill"
)

// attackReach is how far from its victim a blow may land: open-mt2's, a little more than
// the client's own swing reaches, since positions arrive late.
const attackReach = 300

// The shortest time between two blows of a player on one victim is blowGapAtSpeed divided
// by their attack speed, never under minBlowGap: open-mt2's 15000 ms and 80 ms.
const (
	blowGapAtSpeed = 15000 * time.Millisecond
	minBlowGap     = 80 * time.Millisecond
)

// fight is what a monster has to lose. The world record's lock guards it.
type fight struct {
	defender   combat.Defender
	hp, maxHP  int
	diedAt     time.Time // zero while alive
	targetedBy map[*present]struct{}
	// The last player to land a blow, and when: the victim's side of the blow gap, which a
	// player switching between victims cannot get round.
	lastAttacker *present
	lastBlowAt   time.Time
	// damageBy is what each player took off it, which shares its experience out.
	damageBy map[*present]int
	exp      uint32
	hitAt    time.Time // its last blow or skill, from which a stone waits to heal
	// The ailments on it; until when a stun, slow or poison holds; and what a poison and a
	// fire take a turn.
	ailing                            ailments
	stunUntil, slowUntil, poisonUntil time.Time
	poison, fire                      dose
	animal                            bool // of the animals' race, which Silny przeciwko Zwierzętom strikes harder
	// A stone's own: that it is one, the waves it has called as it was worn down, and the
	// smoke it shows.
	stone bool
	calls uint8
	smoke uint64
}

// ailment is a status a skill puts on a monster, which the client draws on it.
type ailment uint8

const (
	stun ailment = iota
	poison
	slow
	fire
	ailmentKinds
)

// ailments is a set of them, a bit each.
type ailments uint8

// dose is what a poison or fire takes off a monster a turn, for whom, and for a fire the
// turns it has left.
type dose struct {
	by    *present
	each  int
	turns int
}

// has is whether the monster suffers the ailment.
func (f *fight) has(k ailment) bool { return f.ailing&(1<<k) != 0 }

// start puts the ailment on the monster, and reports whether it is new. It shows nothing,
// since a slow changes the pace first: the caller shows a new one.
func (f *fight) start(k ailment) bool {
	if f.has(k) {
		return false
	}
	f.ailing |= 1 << k
	return true
}

// cure ends the ailment on the monster and shows it to everyone who sees it, as start
// leaves to its caller.
func (l locked) cure(v *present, k ailment) {
	v.fight.ailing &^= 1 << k
	l.showMob(v)
}

// dead is whether p lies dead: a monster since its death, a player at no hit points.
func (p *present) dead() bool {
	if p.fight != nil {
		return !p.fight.diedAt.IsZero()
	}
	return p.char != nil && p.char.HP == 0
}

// hpPercent is the share of its hit points the monster has left, as the target window
// draws it.
func (f *fight) hpPercent() uint8 {
	return uint8(math.Round(float64(f.hp) * 100 / float64(f.maxHP)))
}

// untarget stops player p targeting t, if it did.
func (l locked) untarget(p, t *present) {
	if p.target == t {
		p.target = nil
		delete(t.fight.targetedBy, p)
	}
}

// target makes the monster vid the player's target and tells them its hit points; a vid
// that is not a living monster they see, 0 included, leaves them with none, as does being
// dead.
func (w *record) target(playerVID, vid uint32) {
	l, unlock := w.lock()
	defer unlock()
	p := l.all[playerVID]
	if p.target != nil {
		l.untarget(p, p.target)
	}
	t, seen := p.sees[vid]
	if p.dead() || !seen || t.fight == nil || t.dead() {
		return
	}
	p.target = t
	t.fight.targetedBy[p] = struct{}{}
	p.send(protocol.EncodeTarget(vid, t.fight.hpPercent()))
}

// blow is what became of an attack.
type blow int

const (
	blowLanded blow = iota
	blowKilled
	blowRefused // nothing to hit there: out of sight or reach, not a monster, or dead
	blowTooSoon // the victim took a blow from the player less than their blow gap ago
)

// attack lands a blow of the player on the monster vid: the attacker is told the damage,
// whoever targets it its hit points, and, if it dies, everyone who sees it its death. A
// blow sooner than gap after the player's last on the same victim is dropped.
func (w *record) attack(playerVID, vid uint32, gap time.Duration, r *rand.Rand, now time.Time) blow {
	l, unlock := w.lock()
	defer unlock()
	p := l.all[playerVID]
	v, seen := p.sees[vid]
	if p.dead() || !seen || v.fight == nil || v.dead() || !within(p, v, attackReach) || bowInHand(p.char) {
		return blowRefused
	}
	f := v.fight
	if f.lastAttacker == p && now.Sub(f.lastBlowAt) < gap {
		return blowTooSoon
	}
	f.lastAttacker, f.lastBlowAt = p, now
	return l.land(p, v, r, now)
}

// land strikes the monster with one of the player's blows, a swing's or an arrow's, at its
// rolled damage: through its defence when a piercing chance comes up, raised by average
// damage and, on an animal, by strength against animals, and twice that when a critical
// chance comes up, from buffs and bonuses alike. A critical or piercing blow is drawn
// with its effect on the player, for them and everyone who sees them.
func (l locked) land(p, v *present, r *rand.Rand, now time.Time) blow {
	t, defender, flag := p.char.Totals, v.fight.defender, uint8(0)
	var effects [][]byte
	if t.Pierce > 0 && r.IntN(100) < t.Pierce {
		defender, flag = combat.Defender{}, protocol.DamagePiercing
		effects = append(effects, protocol.EncodeSpecialEffect(protocol.EffectPiercing, p.add.VID))
	}
	damage := raised(combat.Damage(attackerOf(p.char), defender, r), t.Average, v, t.Animals)
	if crit := buffBonus(p.char, skill.Critical) + t.Critical; crit > 0 && r.IntN(100) < crit {
		damage, flag = 2*damage, flag|protocol.DamageCritical
		effects = append(effects, protocol.EncodeSpecialEffect(protocol.EffectCritical, p.add.VID))
	}
	if flag == 0 {
		flag = protocol.DamageNormal
	}
	struck := l.strikeAs(p, v, damage, flag, now)
	for _, e := range effects {
		p.send(e)
		l.toViewers(p, e)
	}
	return struck
}

// raised is damage raised by percent, a bonus's, and on an animal by animals, the
// strength against them, in one sum so small bonuses are not rounded away. A bonus never
// takes a blow that hurts down to nothing.
func raised(damage, percent int, v *present, animals int) int {
	if !v.fight.animal {
		animals = 0
	}
	if damage <= 0 || percent == 0 && animals == 0 {
		return damage
	}
	return max(1, damage*(100+percent)*(100+animals)/10000)
}

// bowInHand is whether the character holds a bow, which only shoots.
func bowInHand(c *account.Character) bool {
	weapon, _ := item.Lookup(c.Weapon)
	return weapon.Bow
}

// attackerOf is what the player's blows are made of: their character, the weapon they
// hold with the arrows a bow shoots, and what their running buffs add to the attack. Its
// caller holds the record's lock, under which the character's gear and buffs change.
func attackerOf(c *account.Character) combat.Attacker {
	weapon, _ := item.Lookup(c.Weapon)
	least, most := weapon.AttackMin, weapon.AttackMax
	if arrows, _ := item.Lookup(c.Arrow); weapon.Bow {
		least, most = least+arrows.AttackMin, most+arrows.AttackMax
	}
	return combat.Attacker{Level: c.Level, Class: c.Class, Stats: c.Stats, WeaponMin: least,
		WeaponMax: most, Bonus: uint16(buffBonus(c, skill.Attack))} //nolint:gosec // attack buffs only add
}

// buffBonus is what the character's running buffs add to a stat. A buff runs until the
// world loop ends it, under the record's lock, which the caller holds.
func buffBonus(c *account.Character, stat skill.Stat) int {
	var bonus int
	for sk := range c.Buffs {
		p, _ := skill.Lookup(sk)
		bearer, level := skill.Bearer{Strength: c.Strength, Vitality: c.Vitality, Intelligence: c.Intelligence}, c.Skills[sk]
		if by, cast := c.BuffBy[sk]; cast {
			bearer, level = skill.Bearer{Intelligence: by.Intelligence}, by.Level
		}
		bonus += p.Bonus(stat, bearer, level)
	}
	return bonus
}

// strike takes damage, up to what it has left, from the living monster v for the player p,
// a blow's or a skill's: p is told the damage, whoever targets v its hit points, and the
// damage counts towards its drops and experience. A monster left alive turns on p; one
// killed dies for everyone who sees it, its drops and experience given.
func (l locked) strike(p, v *present, damage int, now time.Time) blow {
	return l.strikeAs(p, v, damage, protocol.DamageNormal, now)
}

// strikeAs strikes as strike does, the damage drawn as flag: a critical blow's own.
func (l locked) strikeAs(p, v *present, damage int, flag uint8, now time.Time) blow {
	f := v.fight
	vid := v.add.VID
	p.char.LastFought = now
	damage = min(damage, f.hp)
	f.hp -= damage
	f.damageBy[p] += damage
	f.hitAt = now
	p.send(protocol.EncodeDamage(vid, flag, int32(damage))) //nolint:gosec // at most the hit points
	for t := range f.targetedBy {
		t.send(protocol.EncodeTarget(vid, f.hpPercent()))
	}
	if f.hp > 0 {
		l.news.provoked = append(l.news.provoked, provocation{v, p})
		if f.stone {
			l.wear(v, p)
		}
		return blowLanded
	}
	l.kill(p, v, now)
	return blowKilled
}

// kill lays the monster dead at the player's hand: everyone who sees it sees it die, its
// drops are kept for whoever hurt it most and its experience shared.
func (l locked) kill(p, v *present, now time.Time) {
	f, vid := v.fight, v.add.VID
	f.diedAt, f.ailing = now, 0 // a body has no ailments
	winner := l.topDamager(f, p)
	l.news.fallen = append(l.news.fallen, death{v, now, owner{id: winner.char.ID, name: winner.info.Name}, wearsLuck(winner)})
	if f.stone {
		inform(winner, "Zniszczyłeś kamień Metin.")
	}
	for t := range f.targetedBy {
		l.untarget(t, v)
	}
	l.toViewers(v, protocol.EncodeDead(vid))
	l.shareExp(f, p)
}

// stoneWaves is how many waves a stone calls as it is worn down, one each time it loses
// another fifth of its hit points, and waveCap the most of its monsters alive at once.
const (
	stoneWaves = 4
	waveCap    = 6
)

// call is a wave a stone called out against the player who wore it down.
type call struct {
	stone, by *present
}

// wear calls the waves the stone owes for the hit points it has lost, at 80, 60, 40 and
// 20 in a hundred, each once however it heals, and shows its smoke.
func (l locked) wear(v *present, by *present) {
	f := v.fight
	for int(f.calls) < min(stoneWaves, (f.maxHP-f.hp)*(stoneWaves+1)/f.maxHP) {
		f.calls++
		l.news.calls = append(l.news.calls, call{v, by})
	}
	l.showSmoke(v)
}

// showSmoke tells everyone who sees the stone its smoke, if its wear changed it.
func (l locked) showSmoke(v *present) {
	if smoke := smokeOf(v.fight); smoke != v.fight.smoke {
		v.fight.smoke = smoke
		l.toViewers(v, protocol.EncodeCharacterUpdate(v.info, v.add, smoke))
	}
}

// death is a monster that died, for the world loop to take away and bring back, and
// whom what it leaves is kept for.
type death struct {
	p     *present
	at    time.Time
	owner owner
	lucky bool // the owner wore what doubles drops as it died
}

// news is what happened since the world loop last asked, kept on the world record for it.
type news struct {
	fallen   []death       // monsters killed
	provoked []provocation // blows on living monsters
	laid     []timed       // items players dropped, each with when it is to be cleared away
	buffs    []buffEnd     // buffs players started, each with when it ends
	calls    []call        // waves stones called
	poisoned []*present    // monsters newly poisoned, to tick
	drains   []buffEnd     // toggles started that drain mana
	burned   []*present    // monsters newly set alight, to tick
	lured    []*present    // players who wore a cape, to set the monsters around them on
	timed    []*present    // players who put on an item worn for a limited time, to count down
}

// takeNews hands the world loop the news, with one lock a tick.
func (w *record) takeNews() news {
	l, unlock := w.lock()
	defer unlock()
	n := l.news
	l.news = news{}
	return n
}

// blowGap is the least time between a player's blows on one victim at their attack speed.
func blowGap(attackSpeed uint8) time.Duration {
	return max(minBlowGap, blowGapAtSpeed/time.Duration(max(attackSpeed, 1)))
}

// onTarget records the character the player picked, and shows a monster's hit points.
func (s *session) onTarget(body []byte) error {
	vid, err := protocol.DecodeTarget(body)
	if err != nil {
		return err
	}
	s.svc.record.target(s.playing.vid, vid)
	return nil
}

// onAttack lands a blow the player's client saw hit; a skill's hits are the server's
// own. A blow faster than the player's attack speed, Berserk's included, allows on the
// same victim is dropped, checked on both sides, as the original does: the attacker's last
// victim here, and the victim's last attacker in the world record, which catches a player
// switching between victims.
func (s *session) onAttack(body []byte) error {
	a, err := protocol.DecodeAttack(body)
	if err != nil {
		return err
	}
	now := s.svc.now()
	p := s.playing
	gap := blowGap(s.svc.record.attackSpeedOf(p.vid))
	if a.Skill != 0 || (a.Victim == s.lastVictim && now.Sub(s.lastBlow) < gap) {
		return nil
	}
	s.lastVictim, s.lastBlow = a.Victim, now
	if s.svc.record.attack(p.vid, a.Victim, gap, s.rand, now) == blowKilled {
		s.conn.Log().Debug("killed", "character", p.Name, "victim", a.Victim)
	}
	return nil
}
