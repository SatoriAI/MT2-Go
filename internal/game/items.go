package game

import (
	"context"

	"github.com/SatoriAI/MT2-Go/internal/account"
	"github.com/SatoriAI/MT2-Go/internal/item"
	"github.com/SatoriAI/MT2-Go/internal/protocol"
)

// Items is where the game port keeps what characters own. Each change is written as it
// happens, so what is stored is always what the player has.
type Items interface {
	// ApplyOrders carries out the orders waiting for the character, and returns what it
	// then owns and the orders carried out.
	ApplyOrders(ctx context.Context, characterID uint32) (item.Inventory, []item.Order, error)
	// Place keeps where each item now is, all or none of them.
	Place(ctx context.Context, items ...item.Item) error
	// Add keeps a new item the character owns and returns its id; SetCount keeps a stack's.
	Add(ctx context.Context, characterID uint32, it item.Item) (int64, error)
	SetCount(ctx context.Context, id int64, count uint8) error
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

// onItemMove carries out a drag: an item moved, whole, to cells where all of it fits,
// put on in its own empty place, or taken off into cells where it fits. Anything else, a
// split, a swap or a merge onto another item, another window, is refused: the client is
// told the item is where it was, so it never shows a move that did not happen.
func (s *session) onItemMove(body []byte) error {
	m, err := protocol.DecodeItemMove(body)
	if err != nil {
		return err
	}
	it, ok := s.itemAt(m.From)
	if !ok {
		s.conn.Log().Debug("item move from nothing", "from", m.From, "to", m.To)
		return nil
	}
	p, _ := item.Lookup(it.VNum)
	cell, worn, toOK := placeFromWire(m.To)
	taken := false
	if worn != "" {
		_, taken = s.inventory.WornIn(worn)
	}
	switch {
	case !toOK || (m.Count != 0 && m.Count < it.Count):
	case it.Worn == "" && worn == "" && s.inventory.Fits(p, cell, it.ID):
		moved := it
		moved.Cell = cell
		return s.place(moved)
	case it.Worn == "" && worn != "" && worn == p.Wear && !taken:
		return s.putOn(it)
	case it.Worn != "" && worn == "" && s.inventory.Fits(p, cell, it.ID):
		return s.place(takenOff(it, cell))
	}
	s.conn.Log().Debug("item move refused", "item", it.ID, "to", m.To, "count", m.Count)
	return s.conn.Send(itemSet(it))
}

// onItemUse answers a double-click: an item that is worn is put on, and one worn is
// taken off into the first cells it fits. Using anything else is not done yet.
func (s *session) onItemUse(body []byte) error {
	at, err := protocol.DecodeItemUse(body)
	if err != nil {
		return err
	}
	it, ok := s.itemAt(at)
	if !ok {
		return nil
	}
	p, _ := item.Lookup(it.VNum)
	switch {
	case it.Worn != "":
		if cell, ok := s.inventory.Free(p); ok {
			return s.place(takenOff(it, cell))
		}
		return s.conn.Send(itemSet(it))
	case p.Wear != "":
		return s.putOn(it)
	}
	s.conn.Log().Debug("item use not handled yet", "item", it.ID, "vnum", it.VNum)
	return nil
}

// putOn puts the item on, if the character's class may wear it, telling them why not
// otherwise. The class never changes in play, so it is read without the world's lock. What was worn in its place goes into the cells it leaves, or the
// first others it fits; with no room for it, nothing changes.
func (s *session) putOn(it item.Item) error {
	p, _ := item.Lookup(it.VNum)
	if err := p.CanWear(s.playing.Class); err != nil {
		return s.conn.Send(protocol.EncodeChat(protocol.ChatInfo, 0, 0, textToClient(cannotWear[err])), itemSet(it))
	}
	on := it
	on.Worn = p.Wear
	old, ok := s.inventory.WornIn(p.Wear)
	if !ok {
		return s.place(on)
	}
	oldProto, _ := item.Lookup(old.VNum)
	cell, fits := it.Cell, s.inventory.Fits(oldProto, it.Cell, it.ID)
	if !fits {
		cell, fits = s.inventory.Free(oldProto)
	}
	if !fits {
		return s.conn.Send(itemSet(it))
	}
	return s.place(on, takenOff(old, cell))
}

// cannotWear is what a player is told when they may not wear an item.
var cannotWear = map[error]string{
	item.ErrWrongClass: "Ta postać nie może tego nosić.",
}

func takenOff(it item.Item, cell uint16) item.Item {
	it.Worn, it.Cell = "", cell
	return it
}

// place moves the items to their new places: it keeps them, then tells the client each
// old place is empty and each new one holds its item, and, when what is worn changed,
// dresses the character for everyone who sees them. If they cannot be kept, the client
// is shown them where they were.
func (s *session) place(changed ...item.Item) error {
	before := make([]item.Item, len(changed))
	for i, it := range changed {
		before[i], _ = s.inventory.ByID(it.ID)
	}
	ctx, cancel := context.WithTimeout(context.Background(), saveTimeout)
	defer cancel()
	if err := s.svc.items.Place(ctx, changed...); err != nil {
		s.conn.Log().Error("items not placed", "err", err)
		packets := make([][]byte, 0, len(before))
		for _, it := range before {
			packets = append(packets, itemSet(it))
		}
		return s.conn.Send(packets...)
	}
	packets := make([][]byte, 0, 2*len(changed))
	dressed := false
	for i, it := range changed {
		s.inventory.Replace(it)
		packets = append(packets, itemCleared(before[i]))
		dressed = dressed || it.Worn != before[i].Worn
	}
	for _, it := range changed {
		packets = append(packets, itemSet(it))
	}
	if err := s.conn.Send(packets...); err != nil {
		return err
	}
	if dressed {
		s.dress()
	}
	return nil
}

// dress puts what the inventory has worn on the character, for the fights and for
// everyone who sees them.
func (s *session) dress() {
	weapon, _ := s.inventory.WornIn(item.Weapon)
	body, _ := s.inventory.WornIn(item.Body)
	s.svc.presence.dress(s.playing.vid, weapon.VNum, body.VNum)
}

// itemAt returns the item at a position the client names, in a cell or worn.
func (s *session) itemAt(at protocol.ItemPos) (item.Item, bool) {
	cell, worn, ok := placeFromWire(at)
	switch {
	case !ok:
		return item.Item{}, false
	case worn != "":
		return s.inventory.WornIn(worn)
	}
	return s.inventory.At(cell)
}

// itemPackets are the packets the client sends from its inventory window. Those other
// than a move are about what the game does not do yet, using an item, dropping it, the
// taskbar, trading and giving to an NPC, and are dropped.
var itemPackets = map[protocol.ClientHeader]bool{
	protocol.ClientItemUse: true, protocol.ClientItemDrop: true, protocol.ClientItemMove: true,
	protocol.ClientQuickslotAdd: true, protocol.ClientQuickslotDel: true, protocol.ClientQuickslotSwap: true,
	protocol.ClientItemDrop2: true, protocol.ClientExchange: true, protocol.ClientItemUseToItem: true,
	protocol.ClientGiveItem: true, protocol.ClientItemPickup: true,
}

// dress puts the weapon and armour, by vnum, 0 for none, on the player's character: their
// blows and the bites on them use them from then on, and they and everyone who sees them
// see the change. The player is told their new DEF when the armour changed.
func (w *presence) dress(vid, weapon, body uint32) {
	w.mu.Lock()
	defer w.mu.Unlock()
	p := w.all[vid]
	c := p.char
	armourChanged := c.Body != body
	c.Weapon, c.Body = weapon, body
	p.info.BodyPart, p.info.WeaponPart = bodyPart(*c), weaponPart(*c)
	update := protocol.EncodeCharacterUpdate(p.info, p.add.MoveSpeed, p.add.AttackSpeed)
	p.send(update)
	if armourChanged {
		p.send(protocol.EncodePointChange(vid, protocol.PointDefence, 0, int32(defence(c)))) //nolint:gosec // small
	}
	toViewers(p, update)
}

// defence is what a character sets against a bite, as the character window's DEF shows
// it: their level, vitality and the armour they wear.
func defence(c *account.Character) uint32 {
	armour, _ := item.Lookup(c.Body)
	return uint32(c.Level) + uint32(c.Vitality) + uint32(armour.Defence)
}

// wear puts on the character what the inventory has worn, as it enters the world.
func wear(c *account.Character, inv item.Inventory) {
	weapon, _ := inv.WornIn(item.Weapon)
	body, _ := inv.WornIn(item.Body)
	c.Weapon, c.Body = weapon.VNum, body.VNum
}
