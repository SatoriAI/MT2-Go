package game

import (
	"context"
	"slices"

	"github.com/SatoriAI/MT2-Go/internal/item"
	"github.com/SatoriAI/MT2-Go/internal/protocol"
)

// drink uses one potion of the item's stack: the stack is kept one fewer, or forgotten
// with its last, then the player is healed. A potion that would do nothing, to the dead
// or to a player already full of all it restores, is not used up.
func (s *session) drink(it item.Item) error {
	p, _ := item.Lookup(it.VNum)
	if !s.svc.presence.wouldDrink(s.playing.vid, p) {
		return nil
	}
	if used, err := s.useOne(it); !used || err != nil {
		return err
	}
	s.svc.presence.drink(s.playing.vid, p)
	return nil
}

// useOne uses one of the item's stack up: the stack is kept one fewer, or forgotten with its
// last, and the client shown it. Arrows worn out of their place undress the character of
// them. If it cannot be kept, the client is shown the stack as it was, and nothing is used.
func (s *session) useOne(it item.Item) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), saveTimeout)
	defer cancel()
	left := it
	left.Count--
	var err error
	if left.Count == 0 {
		err = s.svc.items.Remove(ctx, it.ID)
	} else {
		err = s.svc.items.SetCount(ctx, it.ID, left.Count)
	}
	if err != nil {
		s.conn.Log().Error("item not used up", "item", it.ID, "err", err)
		return false, s.conn.Send(itemSet(it))
	}
	shown := itemSet(left)
	if left.Count == 0 {
		s.inventory = slices.DeleteFunc(s.inventory, func(held item.Item) bool { return held.ID == it.ID })
		shown = itemCleared(it)
	} else {
		s.inventory.Replace(left)
	}
	if err := s.conn.Send(append([][]byte{shown}, s.followTaskbar()...)...); err != nil {
		return true, err
	}
	if left.Count == 0 && it.Worn != "" {
		s.dress()
	}
	return true, nil
}

// wouldDrink reports whether the potion would do the player any good: they are alive and
// short of something it restores.
func (w *presence) wouldDrink(vid uint32, potion item.Proto) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	p := w.all[vid]
	if p.dead() {
		return false
	}
	c := p.char
	points, _ := c.Points()
	return potion.HP > 0 && c.HP < points.MaxHP || potion.SP > 0 && c.SP < points.MaxSP
}

// drink heals the player by what the potion restores, up to their maximum, and draws its
// effect for them and everyone who sees them. One who died since it was used up is not
// healed, and one healed to full meanwhile sees no effect of nothing.
func (w *presence) drink(vid uint32, potion item.Proto) {
	w.mu.Lock()
	defer w.mu.Unlock()
	p := w.all[vid]
	if p.dead() {
		return
	}
	points, _ := p.char.Points()
	effect := protocol.EffectBluePotion
	if potion.HP > 0 {
		effect = protocol.EffectRedPotion
	}
	told := restore(p, points.MaxHP, points.MaxSP, uint32(potion.HP), uint32(potion.SP))
	if len(told) == 0 {
		return
	}
	drawn := protocol.EncodeSpecialEffect(effect, vid)
	p.send(append(told, drawn)...)
	toViewers(p, drawn)
}

// restore gives the player hp and sp more, each up to its maximum, and returns what to
// tell them of the change.
func restore(p *present, maxHP, maxSP, hp, sp uint32) [][]byte {
	c := p.char
	newHP, newSP := regained(c.HP, maxHP, hp), regained(c.SP, maxSP, sp)
	vid := p.add.VID
	var told [][]byte
	if newHP != c.HP {
		told = append(told, protocol.EncodePointChange(vid, protocol.PointHP, int32(newHP-c.HP), int32(newHP))) //nolint:gosec // small
	}
	if newSP != c.SP {
		told = append(told, protocol.EncodePointChange(vid, protocol.PointSP, int32(newSP-c.SP), int32(newSP))) //nolint:gosec // small
	}
	c.HP, c.SP = newHP, newSP
	return told
}
