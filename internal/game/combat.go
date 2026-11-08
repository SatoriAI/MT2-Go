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
	// stunnedUntil is when a stun on it ends, shown while stunned is set.
	stunnedUntil time.Time
	stunned      bool
	// A stone's own: that it is one, the waves it has called as it was worn down, and the
	// smoke it shows.
	stone bool
	calls uint8
	smoke uint64
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
func untarget(p, t *present) {
	if p.target == t {
		p.target = nil
		delete(t.fight.targetedBy, p)
	}
}

// target makes the monster vid the player's target and tells them its hit points; a vid
// that is not a living monster they see, 0 included, leaves them with none, as does being
// dead.
func (w *presence) target(playerVID, vid uint32) {
	w.mu.Lock()
	defer w.mu.Unlock()
	p := w.all[playerVID]
	if p.target != nil {
		untarget(p, p.target)
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
func (w *presence) attack(playerVID, vid uint32, gap time.Duration, r *rand.Rand, now time.Time) blow {
	w.mu.Lock()
	defer w.mu.Unlock()
	p := w.all[playerVID]
	v, seen := p.sees[vid]
	if p.dead() || !seen || v.fight == nil || v.dead() || !within(p, v, attackReach) {
		return blowRefused
	}
	f := v.fight
	if f.lastAttacker == p && now.Sub(f.lastBlowAt) < gap {
		return blowTooSoon
	}
	f.lastAttacker, f.lastBlowAt = p, now
	return w.strike(p, v, combat.Damage(attackerOf(p.char), f.defender, r), now)
}

// attackerOf is what the player's blows are made of: their character, the weapon they
// hold, and what their running buffs add to the attack. Called under the lock.
func attackerOf(c *account.Character) combat.Attacker {
	weapon, _ := item.Lookup(c.Weapon)
	return combat.Attacker{Level: c.Level, Class: c.Class, Stats: c.Stats, WeaponMin: weapon.AttackMin,
		WeaponMax: weapon.AttackMax, Bonus: uint16(buffBonus(c, skill.Attack))} //nolint:gosec // attack buffs only add
}

// buffBonus is what the character's running buffs add to a stat. A buff runs until the
// world loop ends it. Called under the lock.
func buffBonus(c *account.Character, stat skill.Stat) int {
	var bonus int
	for sk := range c.Buffs {
		p, _ := skill.Lookup(sk)
		bonus += p.Bonus(stat, skill.Bearer{Strength: c.Strength, Vitality: c.Vitality, Intelligence: c.Intelligence}, c.Skills[sk])
	}
	return bonus
}

// strike takes damage, up to what it has left, from the living monster v for the player p,
// a blow's or a skill's: p is told the damage, whoever targets v its hit points, and the
// damage counts towards its drops and experience. A monster left alive turns on p; one
// killed dies for everyone who sees it, its drops and experience given. Called under the
// lock.
func (w *presence) strike(p, v *present, damage int, now time.Time) blow {
	f := v.fight
	vid := v.add.VID
	p.char.LastFought = now
	damage = min(damage, f.hp)
	f.hp -= damage
	f.damageBy[p] += damage
	f.hitAt = now
	p.send(protocol.EncodeDamage(vid, protocol.DamageNormal, int32(damage))) //nolint:gosec // at most the hit points
	for t := range f.targetedBy {
		t.send(protocol.EncodeTarget(vid, f.hpPercent()))
	}
	if f.hp > 0 {
		w.provoked = append(w.provoked, provocation{v, p})
		if f.stone {
			w.wear(v, p)
		}
		return blowLanded
	}
	f.diedAt, f.stunned = now, false // a body is not stunned
	winner := w.topDamager(f, p)
	w.fallen = append(w.fallen, death{v, now, owner{id: winner.char.ID, name: winner.info.Name}})
	if f.stone {
		winner.send(protocol.EncodeChat(protocol.ChatInfo, 0, 0, textToClient("Zniszczyłeś kamień Metin.")))
	}
	for t := range f.targetedBy {
		untarget(t, v)
	}
	toViewers(v, protocol.EncodeDead(vid))
	w.shareExp(f, p)
	return blowKilled
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
// 20 in a hundred, each once however it heals, and shows its smoke. Called under the
// lock.
func (w *presence) wear(v *present, by *present) {
	f := v.fight
	for int(f.calls) < min(stoneWaves, (f.maxHP-f.hp)*(stoneWaves+1)/f.maxHP) {
		f.calls++
		w.calls = append(w.calls, call{v, by})
	}
	w.showSmoke(v)
}

// showSmoke tells everyone who sees the stone its smoke, if its wear changed it. Called
// under the lock.
func (w *presence) showSmoke(v *present) {
	if smoke := smokeOf(v.fight); smoke != v.fight.smoke {
		v.fight.smoke = smoke
		toViewers(v, protocol.EncodeCharacterUpdate(v.info, v.add, smoke))
	}
}

// death is a monster that died, for the world loop to take away and bring back, and
// whom what it leaves is kept for.
type death struct {
	p     *present
	at    time.Time
	owner owner
}

// news is what happened since the world loop last asked: the monsters that died, the
// blows that landed on living ones, the items players dropped, each with when it is to be
// cleared away, and the buffs they started, with when each ends.
type news struct {
	fallen   []death
	provoked []provocation
	laid     []timed
	buffs    []buffEnd
	calls    []call
}

// takeNews hands the world loop the news, with one lock a tick.
func (w *presence) takeNews() news {
	w.mu.Lock()
	defer w.mu.Unlock()
	n := news{w.fallen, w.provoked, w.laid, w.buffs, w.calls}
	w.fallen, w.provoked, w.laid, w.buffs, w.calls = nil, nil, nil, nil, nil
	return n
}

func within(p, o *present, reach int64) bool {
	dx, dy := int64(p.add.X)-int64(o.add.X), int64(p.add.Y)-int64(o.add.Y)
	return dx*dx+dy*dy <= reach*reach
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
	s.svc.presence.target(s.playing.vid, vid)
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
	now := time.Now()
	p := s.playing
	gap := blowGap(s.svc.presence.attackSpeedOf(p.vid))
	if a.Skill != 0 || (a.Victim == s.lastVictim && now.Sub(s.lastBlow) < gap) {
		return nil
	}
	s.lastVictim, s.lastBlow = a.Victim, now
	if s.svc.presence.attack(p.vid, a.Victim, gap, s.rand, now) == blowKilled {
		s.conn.Log().Debug("killed", "character", p.Name, "victim", a.Victim)
	}
	return nil
}
