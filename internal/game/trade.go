package game

import (
	"context"
	"maps"
	"slices"

	"github.com/SatoriAI/MT2-Go/internal/item"
	"github.com/SatoriAI/MT2-Go/internal/protocol"
)

// onExchange carries out what the player does in a trade: asks another player to trade,
// offers an item they carry or yang, accepts, or cancels.
func (s *session) onExchange(ctx context.Context, body []byte) error {
	e, err := protocol.DecodeExchange(body)
	if err != nil {
		return err
	}
	w, vid, now := &s.svc.presence, s.playing.vid, s.svc.now()
	switch e.Sub {
	case protocol.ExchangeStart:
		if s.shop != 0 || s.upgrading != nil {
			return s.conn.Send(infoLine("Zamknij najpierw otwarte okno."))
		}
		w.startTrade(vid, e.Arg)
	case protocol.ExchangeAdd:
		cell, worn, ok := placeFromWire(e.Pos)
		it, held := s.inventory.At(cell)
		if !ok || worn != "" || !held {
			s.conn.Log().Debug("trade offer refused", "pos", e.Pos)
			return nil
		}
		w.offerItem(vid, it, e.Shown, now)
	case protocol.ExchangeYang:
		w.offerYang(vid, e.Arg, s.gold, now)
	case protocol.ExchangeAccept:
		if t := w.accept(vid, now, s.plan); t != nil {
			return s.keepTrade(ctx, t)
		}
	case protocol.ExchangeCancel:
		w.cancelTrade(vid)
	default:
		s.conn.Log().Debug("trade subheader ignored", "sub", e.Sub)
	}
	return nil
}

// plan places what the other side offers in the player's inventory as it would be with
// what they offer gone, each item in the first cells it fits, and checks their yang
// stays within what they may hold. It names the taskbar slots holding what they offer,
// which the trade empties.
func (s *session) plan(mine []offered, gave uint32, theirs []offered, gets uint32) ([]uint16, []uint8, string) {
	if uint64(s.gold)-uint64(gave)+uint64(gets) > maxGold {
		return nil, nil, "Miałbyś za dużo yang."
	}
	offering := func(id int64) bool { return slices.ContainsFunc(mine, func(o offered) bool { return o.it.ID == id }) }
	inv := slices.DeleteFunc(slices.Clone(s.inventory), func(it item.Item) bool { return offering(it.ID) })
	places := make([]uint16, 0, len(theirs))
	for _, o := range theirs {
		p, _ := item.Lookup(o.it.VNum)
		cell, ok := inv.Free(p)
		if !ok {
			return nil, nil, "Nie masz miejsca w ekwipunku."
		}
		got := o.it
		got.Cell = cell
		inv = append(inv, got)
		places = append(places, cell)
	}
	var slots []uint8
	for _, slot := range slices.Sorted(maps.Keys(s.taskbar)) {
		if offering(s.taskbar[slot].held.Item) {
			slots = append(slots, slot)
		}
	}
	return places, slots, ""
}

// keepTrade stores the trade both sides accepted, and settles it for the player.
func (s *session) keepTrade(ctx context.Context, t *trade) error {
	written, cancel := context.WithTimeout(context.Background(), saveTimeout)
	defer cancel()
	gold, err := s.svc.items.Trade(written, t.given())
	if err != nil {
		s.conn.Log().Error("trade not kept", "err", err)
	} else {
		s.conn.Log().Info("trade kept", "first", t.sides[0].p.info.Name, "second", t.sides[1].p.info.Name, "trade", t.given())
	}
	return s.settle(ctx, s.svc.presence.tradeKept(t, s.playing.vid, gold, err != nil))
}

// settle applies to what the player holds what a trade changed for them, or reads it all
// again after a trade that could not be kept.
func (s *session) settle(ctx context.Context, st settlement) error {
	if st.reload {
		if err := s.loadInventory(ctx, s.playing.ID); err != nil {
			return err
		}
		return s.loadTaskbar(ctx, s.playing.ID)
	}
	s.inventory = slices.DeleteFunc(s.inventory, func(it item.Item) bool {
		return slices.ContainsFunc(st.gave, func(g item.Item) bool { return g.ID == it.ID })
	})
	s.inventory = append(s.inventory, st.got...)
	s.gold = uint32(int64(s.gold) + st.gold) //nolint:gosec // the database kept the balance within its cap
	for _, slot := range st.slots {
		delete(s.taskbar, slot)
	}
	return nil
}

// catchUp applies what a trade the other side kept changed for the player, and reports
// whether they are trading.
func (s *session) catchUp(ctx context.Context) (bool, error) {
	st, kept, trading := s.svc.presence.catchUp(s.playing.vid)
	if kept {
		return trading, s.settle(ctx, st)
	}
	return trading, nil
}

// leaveTrade ends the player's trade as they leave the world, first waiting for one being
// kept, so a new login reads what they hold after it. What it changed for them is not
// applied: the next login reads it all.
func (s *session) leaveTrade() {
	if done := s.svc.presence.leaveTrade(s.playing.vid); done != nil {
		<-done
	}
	s.svc.presence.catchUp(s.playing.vid)
}

// touchesWhatIsHeld are the packets that read or change what the player carries, their
// yang or their taskbar. Each first takes what a trade kept by the other side left them;
// while they trade, all but the trade's own are refused, so what each side accepted for
// stays what it holds.
var touchesWhatIsHeld = func() map[protocol.ClientHeader]bool {
	m := maps.Clone(itemPackets)
	m[protocol.ClientShop], m[protocol.ClientOnClick], m[protocol.ClientRefine] = true, true, true
	return m
}()

// closesAWindow reports whether the packet only closes a shop or the blacksmith's dialog,
// which a trade started by someone else may have found open.
func closesAWindow(f protocol.Frame) bool {
	switch f.Header {
	case protocol.ClientShop:
		return len(f.Body) > 0 && f.Body[0] == protocol.ShopEnd
	case protocol.ClientRefine:
		q, err := protocol.DecodeRefine(f.Body)
		return err == nil && q.Cell == protocol.RefineCancel
	}
	return false
}
