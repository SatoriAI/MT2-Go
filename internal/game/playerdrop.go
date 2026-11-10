package game

import (
	"context"
	"slices"

	"github.com/SatoriAI/MT2-Go/internal/item"
	"github.com/SatoriAI/MT2-Go/internal/protocol"
)

// onItemDrop puts what the player dropped, having said yes to the client's question, on
// the ground beside them: count of the item in a bag cell, or yang. It is written first,
// then laid down for anyone. Anything else, a worn item, too many, or a dead player, is
// refused without a word, and the client still shows it where it was.
func (s *session) onItemDrop(body []byte) error {
	d, err := protocol.DecodeItemDrop2(body)
	if err != nil {
		return err
	}
	if !s.svc.record.alive(s.playing.vid) {
		return nil
	}
	if d.Gold > 0 && d.Pos == (protocol.ItemPos{}) {
		return s.dropGold(d.Gold)
	}
	cell, worn, ok := placeFromWire(d.Pos)
	it, held := s.inventory.At(cell)
	if !ok || worn != "" || !held || d.Count < 1 || d.Count > it.Count {
		s.conn.Log().Debug("drop refused", "pos", d.Pos, "count", d.Count)
		return nil
	}
	left := it
	left.Count -= d.Count
	ctx, cancel := context.WithTimeout(context.Background(), saveTimeout)
	defer cancel()
	if left.Count == 0 {
		err = s.svc.items.Remove(ctx, s.playing.ID, it.ID)
	} else {
		err = s.svc.items.SetCount(ctx, s.playing.ID, it.ID, left.Count)
	}
	if err != nil {
		s.conn.Log().Error("drop not kept", "item", it.ID, "err", err)
		return nil
	}
	shown := itemSet(left)
	if left.Count == 0 {
		s.inventory = slices.DeleteFunc(s.inventory, func(h item.Item) bool { return h.ID == it.ID })
		shown = itemCleared(it)
	} else {
		s.inventory.Replace(left)
	}
	err = s.conn.Send(append([][]byte{shown}, s.followTaskbar()...)...)
	s.layDown(&groundItem{vnum: it.VNum, count: d.Count}) // written, so laid down whatever came of telling them
	return err
}

// dropGold takes amount yang from the player and lays it on the ground beside them.
func (s *session) dropGold(amount uint32) error {
	if amount > s.gold {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), saveTimeout)
	defer cancel()
	gold, err := s.svc.items.SpendGold(ctx, s.playing.ID, amount)
	if err != nil {
		s.conn.Log().Error("yang dropped not kept", "amount", amount, "err", err)
		return nil
	}
	s.gold = gold
	err = s.conn.Send(s.yangChanged(-int64(amount)))
	s.layDown(&groundItem{gold: amount})
	return err
}

// layDown lays a dropped item on the ground near the player, now.
func (s *session) layDown(it *groundItem) {
	it.vid, it.dropped = s.svc.lastVID.Add(1), s.svc.now()
	s.svc.record.layDown(s.playing.vid, it, s.rand.Int32N(2*dropSpread+1)-dropSpread,
		s.rand.Int32N(2*dropSpread+1)-dropSpread)
}

// alive is whether the player stands in the world alive.
func (w *record) alive(vid uint32) bool {
	l, unlock := w.lock()
	defer unlock()
	return !l.all[vid].dead()
}
