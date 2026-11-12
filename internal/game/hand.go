package game

import (
	"context"
	"slices"
	"time"

	"github.com/SatoriAI/MT2-Go/internal/item"
	"github.com/SatoriAI/MT2-Go/internal/protocol"
)

// handOf is when the third hand the inventory wears ends, or zero for none.
func handOf(inv item.Inventory) time.Time {
	for _, place := range []item.Wear{item.Unique1, item.Unique2} {
		if it, ok := inv.WornIn(place); ok {
			if p, _ := item.Lookup(it.VNum); p.PicksYang {
				return it.EndsAt
			}
		}
	}
	return time.Time{}
}

// wearHand keeps when the third hand the player wears ends, and shows them its icon while
// it works, or takes the icon away.
func (w *record) wearHand(vid uint32, ends, now time.Time) {
	l, unlock := w.lock()
	defer unlock()
	p := l.all[vid]
	was := now.Before(p.hand)
	p.hand = ends
	switch is := now.Before(ends); {
	case is && !was:
		p.send(handShown())
	case was && !is:
		p.send(handGone())
	}
}

// collector is who a dead monster's yang goes straight to: the player its drops are kept
// for, wearing a third hand that still works, on its map. Anyone else's yang falls.
func collector(winner, body *present, now time.Time) *present {
	if winner.world == body.world && now.Before(winner.hand) {
		return winner
	}
	return nil
}

// creditYang hands each player the yang their third hand picked up this tick, one write and
// one POINT_CHANGE each, however many monsters fell. The write is the store's, so it runs
// off the world loop, which waits for those under way as it stops; the player's connection
// reads its yang again before it next uses it.
func (svc *Service) creditYang() {
	for p, amount := range svc.picked {
		svc.crediting.Go(func() { svc.credit(p, amount) })
	}
	clear(svc.picked)
}

// credit keeps the yang picked up for the player and tells them their balance. One player's
// credits take turns, so the balances they are told only rise. Yang the store refuses, past
// what they may hold, falls at their feet instead, theirs alone, if they are still there.
func (svc *Service) credit(p *present, amount uint32) {
	p.creditTurn.Lock()
	defer p.creditTurn.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), saveTimeout)
	defer cancel()
	gold, err := svc.items.AddGold(ctx, p.char.ID, amount)
	if err != nil {
		laid := svc.record.layYang(p, &groundItem{vid: svc.lastVID.Add(1), gold: amount,
			owner: owner{id: p.char.ID, name: p.info.Name}, dropped: svc.now()})
		svc.log.Warn("yang picked up by the third hand not kept", "character", p.char.ID, "amount", amount,
			"laid at their feet", laid, "err", err)
		return
	}
	p.credited.Store(true)
	p.send(pointTold(p.add.VID, protocol.PointGold, int64(amount), gold))
}

// layYang lays yang the third hand could not keep at the player's feet, if they are still
// in the world, and reports whether it did.
func (w *record) layYang(p *present, it *groundItem) bool {
	l, unlock := w.lock()
	defer unlock()
	if l.all[p.add.VID] != p {
		return false
	}
	it.world, it.x, it.y = p.world, p.add.X, p.add.Y
	l.lay(it)
	l.toNear(it, shown(it)...)
	l.news.laid = append(l.news.laid, timed{it.vid, it.dropped.Add(lieFor)})
	return true
}

// lastingIn is what the inventory holds that lasts in real time.
func lastingIn(inv item.Inventory) []item.Item {
	var lasting []item.Item
	for _, it := range inv {
		if p, _ := item.Lookup(it.VNum); p.Lasts > 0 {
			lasting = append(lasting, it)
		}
	}
	return lasting
}

// keepLasting tells the world record what the player holds that lasts in real time, where
// it now is, for the world loop to end it on time; not what has ended, which the loop has
// already, until the connection forgets it. Such an item is bound and leaves only at its
// end, which the loop itself takes off its list, so a player holding none tells nothing.
func (s *session) keepLasting() {
	lasting := lastingIn(s.inventory)
	if len(lasting) == 0 {
		return
	}
	now := s.svc.now()
	lasting = slices.DeleteFunc(lasting, func(it item.Item) bool { return !now.Before(it.EndsAt) })
	if len(lasting) > 0 {
		s.svc.record.holdLasting(s.playing.vid, lasting)
	}
}

// holdLasting keeps what the player holds that lasts in real time, and has the world loop
// watch them while they hold any.
func (w *record) holdLasting(vid uint32, lasting []item.Item) {
	l, unlock := w.lock()
	defer unlock()
	p := l.all[vid]
	p.lasting = lasting
	if len(lasting) > 0 && !p.watched {
		p.watched = true
		l.news.lasting = append(l.news.lasting, p)
	}
}

// endLasting ends what has reached its end of what the watched players hold: the client is
// told it is gone, and a third hand that ends stops picking up yang; the connection forgets
// it before its next packet that touches what it holds, as it does a spent pair of gloves.
func (svc *Service) endLasting(now time.Time) {
	if len(svc.lastings) == 0 {
		return
	}
	l, unlock := svc.record.lock()
	defer unlock()
	svc.lastings = slices.DeleteFunc(svc.lastings, func(p *present) bool {
		if l.all[p.add.VID] != p {
			return true
		}
		p.lasting = slices.DeleteFunc(p.lasting, func(it item.Item) bool {
			if now.Before(it.EndsAt) {
				return false
			}
			kind, _ := item.Lookup(it.VNum)
			l.spent[p.add.VID] = append(l.spent[p.add.VID], it.ID)
			p.send(itemCleared(it), infoLine("Czas działania przedmiotu "+kind.Name+" minął."))
			if it.Worn != "" && kind.PicksYang {
				p.hand = time.Time{}
				p.send(handGone())
			}
			return true
		})
		p.watched = len(p.lasting) > 0
		return !p.watched
	})
}

// handShown is the third hand's icon, if the player enters the world wearing one at work.
func (s *session) handShown() [][]byte {
	if s.svc.now().Before(handOf(s.inventory)) {
		return [][]byte{handShown()}
	}
	return nil
}

// catchUpYang reads the player's yang again once their third hand has picked some up, so
// what the connection holds is the store's before it is used, and shows the client it, in
// case a credit's balance reached it after one of its own.
func (s *session) catchUpYang(ctx context.Context) error {
	if s.me == nil || !s.me.credited.Swap(false) {
		return nil
	}
	gold, err := s.svc.items.Gold(ctx, s.playing.ID)
	if err != nil {
		return err
	}
	s.gold = gold
	return s.conn.Send(s.yangChanged(0))
}
