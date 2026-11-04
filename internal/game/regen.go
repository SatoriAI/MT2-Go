package game

import (
	"time"

	"github.com/SatoriAI/MT2-Go/internal/protocol"
)

// Players regain hit points and mana every playerRegenEvery while alive and out of a
// fight: playerRegenFlat hit points and playerRegenPercent of their maximum, and that
// share of their mana. open-mt2's numbers, without its 1% in the first step after moving.
// A player is in a fight until outOfFight after their last blow or bite: healing during
// one would outpace a wild dog, and nobody could die to it.
const (
	playerRegenEvery   = 3 * time.Second
	playerRegenFlat    = 15
	playerRegenPercent = 5
	outOfFight         = 5 * time.Second
)

// regenPlayers heals every living player out of a fight who is short of hit points or
// mana, and tells them what they have now.
func (w *presence) regenPlayers(now time.Time) {
	w.mu.Lock()
	defer w.mu.Unlock()
	for _, p := range w.byName {
		c := p.char
		if c == nil || p.dead() || now.Sub(c.LastFought) < outOfFight {
			continue
		}
		points, _ := c.Points()
		hp := regained(c.HP, points.MaxHP, playerRegenFlat+points.MaxHP*playerRegenPercent/100)
		sp := regained(c.SP, points.MaxSP, points.MaxSP*playerRegenPercent/100)
		vid := p.add.VID
		var told [][]byte
		if hp != c.HP {
			told = append(told, protocol.EncodePointChange(vid, protocol.PointHP, int32(hp-c.HP), int32(hp))) //nolint:gosec // small
		}
		if sp != c.SP {
			told = append(told, protocol.EncodePointChange(vid, protocol.PointSP, int32(sp-c.SP), int32(sp))) //nolint:gosec // small
		}
		p.send(told...)
		c.HP, c.SP = hp, sp
	}
}

// regained is have with gain added, up to most; it never takes any away.
func regained(have, most, gain uint32) uint32 {
	if have >= most {
		return have
	}
	return min(most, have+gain)
}

// healing is a hurt monster the world loop heals, as it was when hurt, and when it heals
// next. A monster that died and came back is another present, which this does not heal.
type healing struct {
	m    *monster
	p    *present
	next time.Time
}

// hurt puts the monster on the loop's list of the hurt, once, to heal from a cycle on.
func (s *Service) hurt(m *monster, now time.Time) {
	if m.healing || m.proto.RegenCycle == 0 {
		return
	}
	m.healing = true
	s.healing = append(s.healing, healing{m, m.p, now.Add(time.Duration(m.proto.RegenCycle) * time.Second)})
}

// healMonsters heals each hurt monster whose cycle has come, and drops from the list those
// that are whole again or dead. Only the hurt are walked, so an idle tick costs nothing
// more.
func (s *Service) healMonsters(now time.Time) {
	kept := s.healing[:0]
	for _, h := range s.healing {
		if now.Before(h.next) {
			kept = append(kept, h)
			continue
		}
		if h.m.p == h.p && s.presence.heal(h.p, h.m.proto.RegenPercent) {
			h.next = now.Add(time.Duration(h.m.proto.RegenCycle) * time.Second)
			kept = append(kept, h)
			continue
		}
		if h.m.p == h.p {
			h.m.healing = false
		}
	}
	clear(s.healing[len(kept):])
	s.healing = kept
}

// heal gives the monster back percent of its hit points, at least one, and tells whoever
// picked it. It reports whether the monster is still short of them; a dead one is not
// healed.
func (w *presence) heal(p *present, percent uint8) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	f := p.fight
	if p.dead() || f.hp >= f.maxHP {
		return false
	}
	f.hp = min(f.maxHP, f.hp+max(1, f.maxHP*int(percent)/100))
	for t := range f.targetedBy {
		t.send(protocol.EncodeTarget(p.add.VID, f.hpPercent()))
	}
	return f.hp < f.maxHP
}
