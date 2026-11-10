package game

import (
	"context"
	"slices"

	"github.com/SatoriAI/MT2-Go/internal/item"
	"github.com/SatoriAI/MT2-Go/internal/mob"
	"github.com/SatoriAI/MT2-Go/internal/protocol"
)

// npcReach is how near an NPC a click reaches them from: the client walks to within 500
// before it sends the click, and the rest is room for moves still on their way.
const npcReach = 1000

// onClick answers a click on an NPC near the player: a shopkeeper opens their shop, and a
// class teacher talks of the skill group they teach. Any other click is not answered yet.
func (s *session) onClick(body []byte) error {
	vid, err := protocol.DecodeOnClick(body)
	if err != nil {
		return err
	}
	npc, ok := s.svc.record.npcInReach(s.playing.vid, vid)
	switch {
	case !ok:
		return nil
	case len(npc.Sells) > 0:
		return s.openShop(vid, npc)
	case npc.Teaches != "":
		return s.teach(vid, npc)
	}
	return nil
}

// openShop opens the shop of the shopkeeper vid.
func (s *session) openShop(vid uint32, keeper mob.Proto) error {
	s.shop = keeper.VNum
	var cells [protocol.ShopCells]protocol.ShopItem
	for _, w := range keeper.Sells {
		kind, _ := item.Lookup(w.VNum)
		cells[w.Cell] = protocol.ShopItem{VNum: w.VNum, Price: kind.PriceOf(w.Count), Count: w.Count}
	}
	return s.conn.Send(protocol.EncodeShopStart(vid, cells))
}

// npcInReach returns the kind of the character vid, if it stands within reach of the
// player, alive, on their map. Only an NPC's kind sells or teaches, so a monster or a
// player found here is answered with nothing.
func (w *record) npcInReach(playerVID, vid uint32) (mob.Proto, bool) {
	l, unlock := w.lock()
	defer unlock()
	p, npc := l.all[playerVID], l.all[vid]
	if npc == nil || p.dead() || npc.world != p.world || !near(p.add.X, p.add.Y, npc.add.X, npc.add.Y, npcReach+1) {
		return mob.Proto{}, false
	}
	return mob.Lookup(npc.add.Race)
}

// onShop carries out what the player does in the shop they opened: closing it, buying a
// cell's ware or selling from the bag. The client's other sale, which its scripts never
// send, and anything with no shop open, are dropped.
func (s *session) onShop(body []byte) error {
	q, err := protocol.DecodeShop(body)
	if err != nil {
		return err
	}
	switch {
	case q.Sub == protocol.ShopEnd:
		s.shop = 0
		return nil
	case s.shop == 0 || q.Sub == protocol.ShopSell:
		s.conn.Log().Debug("shop request dropped", "sub", q.Sub, "shop", s.shop)
		return nil
	case q.Sub == protocol.ShopBuy:
		return s.buy(q.Pos)
	}
	return s.sell(q.Cell, q.Count)
}

// buy sells the player the ware at the shop's cell pos, for its price, into the first room
// their bag has for it: a stack of its kind with room for all of it, or free cells.
func (s *session) buy(pos uint8) error {
	keeper, _ := mob.Lookup(s.shop)
	i := slices.IndexFunc(keeper.Sells, func(w mob.Ware) bool { return w.Cell == pos })
	if i < 0 {
		return s.conn.Send(protocol.EncodeShopError(protocol.ShopInvalid))
	}
	ware := keeper.Sells[i]
	kind, _ := item.Lookup(ware.VNum)
	price := kind.PriceOf(ware.Count)
	if s.gold < price {
		return s.conn.Send(protocol.EncodeShopError(protocol.ShopNotEnoughYang))
	}
	into, ok := s.inventory.RoomFor(kind, ware.Count)
	if !ok {
		return s.conn.Send(protocol.EncodeShopError(protocol.ShopBagFull))
	}
	ctx, cancel := context.WithTimeout(context.Background(), saveTimeout)
	defer cancel()
	bought, gold, err := s.svc.items.Buy(ctx, s.playing.ID, price, into)
	if err != nil {
		s.conn.Log().Error("buy not kept", "vnum", ware.VNum, "err", err)
		return nil
	}
	if _, held := s.inventory.ByID(bought.ID); held {
		s.inventory.Replace(bought)
	} else {
		s.inventory = append(s.inventory, bought)
	}
	s.gold = gold
	return s.conn.Send(itemSet(bought), s.yangChanged(-int64(price)))
}

// sell buys count of the item at the bag's cell back from the player, at what the client's
// tooltip says it is worth, 0 included. A worn item has no cell, so it cannot be sold.
func (s *session) sell(cell, count uint8) error {
	it, ok := s.inventory.At(uint16(cell))
	if !ok || count < 1 || count > it.Count {
		return s.conn.Send(protocol.EncodeShopError(protocol.ShopInvalid))
	}
	kind, _ := item.Lookup(it.VNum)
	pay := kind.SellPrice * uint32(count)
	if uint64(s.gold)+uint64(pay) > maxGold {
		return s.conn.Send(protocol.EncodeShopError(protocol.ShopInvalid))
	}
	left := it
	left.Count -= count
	ctx, cancel := context.WithTimeout(context.Background(), saveTimeout)
	defer cancel()
	gold, err := s.svc.items.Sell(ctx, s.playing.ID, pay, left)
	if err != nil {
		s.conn.Log().Error("sale not kept", "item", it.ID, "err", err)
		return nil
	}
	shown := itemSet(left)
	if left.Count == 0 {
		s.inventory = slices.DeleteFunc(s.inventory, func(held item.Item) bool { return held.ID == it.ID })
		shown = itemCleared(it)
	} else {
		s.inventory.Replace(left)
	}
	s.gold = gold
	return s.conn.Send(append([][]byte{shown, s.yangChanged(int64(pay))}, s.followTaskbar()...)...)
}

// yangChanged tells the player their yang changed by amount, to what they have now. The
// client says how much they got when it is more, and nothing when it is less.
func (s *session) yangChanged(amount int64) []byte {
	return pointTold(s.playing.vid, protocol.PointGold, amount, s.gold)
}
