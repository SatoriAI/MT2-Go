package game

import (
	"context"

	"github.com/SatoriAI/MT2-Go/internal/item"
	"github.com/SatoriAI/MT2-Go/internal/protocol"
)

// Items is where the game port keeps what characters own. Each change is written as it
// happens, so what is stored is always what the player has.
type Items interface {
	// ApplyOrders carries out the orders waiting for the character, and returns what it
	// then owns and the orders carried out.
	ApplyOrders(ctx context.Context, characterID uint32) (item.Inventory, []item.Order, error)
	Move(ctx context.Context, id int64, cell uint16) error
}

// loadInventory carries out the orders waiting for the character, and keeps what it then
// owns. Only the connection's goroutine touches the inventory from then on.
func (s *session) loadInventory(ctx context.Context, characterID uint32) error {
	inv, done, err := s.svc.items.ApplyOrders(ctx, characterID)
	if err != nil {
		return err
	}
	for _, o := range done {
		s.conn.Log().Info("item order carried out", "order", o.ID, "kind", o.Kind, "vnum", o.VNum, "count", o.Count,
			"item", o.ItemID)
	}
	s.inventory = inv
	return nil
}

// onItemMove moves an item, whole, to cells where all of it fits. Anything else, a split,
// a swap or a merge onto another item, or another window, is refused: the client is told
// the item is where it was, so it never shows a move that did not happen.
func (s *session) onItemMove(body []byte) error {
	m, err := protocol.DecodeItemMove(body)
	if err != nil {
		return err
	}
	from, fromOK := cellFromWire(m.From)
	it, ok := s.inventory.At(from)
	if !fromOK || !ok {
		s.conn.Log().Debug("item move from nothing", "from", m.From, "to", m.To)
		return nil
	}
	p, _ := item.Lookup(it.VNum)
	to, toOK := cellFromWire(m.To)
	if !toOK || (m.Count != 0 && m.Count < it.Count) || !s.inventory.Fits(p, to, it.ID) {
		s.conn.Log().Debug("item move refused", "item", it.ID, "to", m.To, "count", m.Count)
		return s.conn.Send(itemSet(it))
	}
	ctx, cancel := context.WithTimeout(context.Background(), saveTimeout)
	defer cancel()
	if err := s.svc.items.Move(ctx, it.ID, to); err != nil {
		s.conn.Log().Error("item move not saved", "item", it.ID, "err", err)
		return s.conn.Send(itemSet(it))
	}
	for i := range s.inventory {
		if s.inventory[i].ID == it.ID {
			s.inventory[i].Cell = to
		}
	}
	moved := it
	moved.Cell = to
	return s.conn.Send(itemCleared(from), itemSet(moved))
}

// itemPackets are the packets the client sends from its inventory window. Those other
// than a move are about what the game does not do yet, using an item, dropping it, the
// taskbar, trading and giving to an NPC, and are dropped.
var itemPackets = map[protocol.ClientHeader]bool{
	protocol.ClientItemUse: true, protocol.ClientItemDrop: true, protocol.ClientItemMove: true,
	protocol.ClientQuickslotAdd: true, protocol.ClientQuickslotDel: true, protocol.ClientQuickslotSwap: true,
	protocol.ClientItemDrop2: true, protocol.ClientExchange: true, protocol.ClientItemUseToItem: true,
	protocol.ClientGiveItem: true,
}
