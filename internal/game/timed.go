package game

import (
	"context"
	"log/slog"
	"slices"
	"sync"
	"time"

	"github.com/SatoriAI/MT2-Go/internal/item"
)

// wearing is an item worn for a limited time by a player in the world, the thief's gloves:
// as it was put on, with what it had been worn for then, and since when. Its time counts
// only while it is worn in the world, so it is kept here, under the world's lock.
type wearing struct {
	it    item.Item
	since time.Time
	shown int32 // the minutes left the client was last told
	spent bool  // its time ran out, and the client was told it is gone
}

// wornFor is how long the item has been worn by now.
func (w *wearing) wornFor(now time.Time) time.Duration { return w.it.WornFor + now.Sub(w.since) }

// soFar is the item as worn by now, and whether its time ran out.
func (w *wearing) soFar(now time.Time) (item.Item, bool) {
	it := w.it
	it.WornFor = w.wornFor(now)
	return it, w.spent
}

// lasting is what the inventory wears for a limited time, or an item of id 0.
func lasting(inv item.Inventory) item.Item {
	for _, place := range []item.Wear{item.Unique1, item.Unique2} {
		if it, ok := inv.WornIn(place); ok {
			if p, _ := item.Lookup(it.VNum); p.WearFor > 0 {
				return it
			}
		}
	}
	return item.Item{}
}

// wearTimed starts counting what the player now wears for a limited time, and stops
// counting what they took off, which it returns as worn so far, unless its time ran out.
// The world loop is told of the one put on, that run of it, to tell its minutes and end it.
func (w *record) wearTimed(vid uint32, it item.Item, now time.Time) (item.Item, bool) {
	l, unlock := w.lock()
	defer unlock()
	p := l.all[vid]
	old := p.timed
	if old != nil && old.it.ID == it.ID {
		return item.Item{}, false
	}
	p.timed = nil
	if it.ID != 0 {
		p.timed = &wearing{it: it, since: now, shown: socketsOf(it)[2]}
		l.news.timed = append(l.news.timed, timing{p, p.timed})
	}
	if old == nil || old.spent {
		return item.Item{}, false
	}
	off, _ := old.soFar(now)
	return off, true
}

// timing is a player the world loop counts an item down for, and that item's run.
type timing struct {
	p *present
	w *wearing
}

// keepTimed tells each player whose item worn for a limited time has a minute less left,
// and ends the item whose time ran out: the client is told it is gone, and its
// connection forgets it at its next move of an item. One taken off, or whose player left,
// is no longer counted here.
func (svc *Service) keepTimed(now time.Time) {
	if len(svc.timings) == 0 {
		return
	}
	l, unlock := svc.record.lock()
	defer unlock()
	svc.timings = slices.DeleteFunc(svc.timings, func(t timing) bool {
		if l.all[t.p.add.VID] != t.p || t.p.timed != t.w {
			return true
		}
		worn, _ := t.w.soFar(now)
		minutes := socketsOf(worn)[2]
		if minutes == 0 {
			p, _ := item.Lookup(worn.VNum)
			t.w.spent = true
			l.spent[t.p.add.VID] = append(l.spent[t.p.add.VID], worn.ID)
			t.p.send(itemCleared(worn), infoLine("Czas działania przedmiotu "+p.Name+" minął."))
			return true
		}
		if minutes != t.w.shown {
			t.w.shown = minutes
			t.p.send(itemSet(worn))
		}
		return false
	})
}

// takeSpent hands the player's connection the items whose time ran out since it last asked.
func (w *record) takeSpent(vid uint32) []int64 {
	l, unlock := w.lock()
	defer unlock()
	ids := l.spent[vid]
	delete(l.spent, vid)
	return ids
}

// forgetSpent forgets the items whose time ran out, here and in the store.
func (s *session) forgetSpent() {
	for _, id := range s.svc.record.takeSpent(s.playing.vid) {
		s.inventory = slices.DeleteFunc(s.inventory, func(it item.Item) bool { return it.ID == id })
		s.svc.removeSpent(s.playing.ID, id, s.conn.Log())
	}
}

// keepTakenOff keeps how long the item taken off was worn, in the inventory and the store,
// and shows the client its minutes left. It holds the player's save, so a periodic save
// that read the time before it cannot land after it.
func (s *session) keepTakenOff(off item.Item) error {
	held, ok := s.inventory.ByID(off.ID)
	if !ok {
		return nil
	}
	held.WornFor = off.WornFor
	s.inventory.Replace(held)
	saving := s.svc.record.savingOf(s.playing.vid)
	saving.Lock()
	s.svc.keepWorn(s.playing.ID, held, false, s.conn.Log())
	saving.Unlock()
	return s.conn.Send(itemSet(held))
}

// keepWorn keeps how long an item worn for a limited time has been worn: all its time,
// once that ran out, so the store holds it spent until the connection or the next load
// forgets it. It has its own context, as leaving runs it at shutdown too.
func (svc *Service) keepWorn(characterID uint32, it item.Item, spent bool, log *slog.Logger) {
	ctx, cancel := context.WithTimeout(context.Background(), saveTimeout)
	defer cancel()
	if spent {
		p, _ := item.Lookup(it.VNum)
		it.WornFor = p.WearFor
	}
	if err := svc.items.SetWornFor(ctx, characterID, it.ID, it.WornFor); err != nil {
		log.Error("time worn not kept", "item", it.ID, "err", err)
	}
}

// removeSpent forgets an item whose time ran out, in the store.
func (svc *Service) removeSpent(characterID uint32, id int64, log *slog.Logger) {
	ctx, cancel := context.WithTimeout(context.Background(), saveTimeout)
	defer cancel()
	if err := svc.items.Remove(ctx, characterID, id); err != nil {
		log.Error("spent item not forgotten", "item", id, "err", err)
	}
}

// savingOf is what the player's saves hold, to keep them in order.
func (w *record) savingOf(vid uint32) *sync.Mutex {
	l, unlock := w.lock()
	defer unlock()
	return &l.all[vid].saving
}

// timedOf is what the player wears for a limited time, as worn by now, and whether its
// time ran out, if they are still in the world and wear one.
func (w *record) timedOf(p *present, now time.Time) (item.Item, bool, bool) {
	l, unlock := w.lock()
	defer unlock()
	if l.all[p.add.VID] != p || p.timed == nil {
		return item.Item{}, false, false
	}
	it, spent := p.timed.soFar(now)
	return it, spent, true
}

// dropSpentOnes forgets the items of the inventory whose time has run out: worn to the end
// before the character last left.
func (s *session) dropSpentOnes(characterID uint32) {
	s.inventory = slices.DeleteFunc(s.inventory, func(it item.Item) bool {
		p, _ := item.Lookup(it.VNum)
		if p.WearFor == 0 || p.Left(it.WornFor) > 0 {
			return false
		}
		s.svc.removeSpent(characterID, it.ID, s.conn.Log())
		return true
	})
}
