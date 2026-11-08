package game

import (
	"context"

	"github.com/SatoriAI/MT2-Go/internal/account"
	"github.com/SatoriAI/MT2-Go/internal/item"
	"github.com/SatoriAI/MT2-Go/internal/protocol"
	"github.com/SatoriAI/MT2-Go/internal/skill"
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
	// Remove forgets an item the character no longer has.
	Remove(ctx context.Context, id int64) error
	// Gold returns the character's yang; AddGold keeps more of it and returns the balance.
	Gold(ctx context.Context, characterID uint32) (uint32, error)
	AddGold(ctx context.Context, characterID, amount uint32) (uint32, error)
	// SpendGold takes yang from the character and returns the balance.
	SpendGold(ctx context.Context, characterID, amount uint32) (uint32, error)
	// Buy takes price yang and keeps what was bought; Sell pays and keeps what is left.
	// Each returns the yang the character then has.
	Buy(ctx context.Context, characterID, price uint32, into item.Item) (item.Item, uint32, error)
	Sell(ctx context.Context, characterID, pay uint32, left item.Item) (uint32, error)
	// Upgrade takes cost yang, keeps the item's new kind and what is left of each stack of
	// material spent, and returns the yang the character then has.
	Upgrade(ctx context.Context, characterID, cost uint32, upgraded item.Item, spent []item.Item) (uint32, error)
}

// loadInventory carries out the orders waiting for the character, and keeps what it then
// owns, items and yang. Only the connection's goroutine touches either from then on.
func (s *session) loadInventory(ctx context.Context, characterID uint32) error {
	inv, done, err := s.svc.items.ApplyOrders(ctx, characterID)
	if err != nil {
		return err
	}
	for _, o := range done {
		s.conn.Log().Info("item order carried out", "order", o.ID, "kind", o.Kind, "vnum", o.VNum, "count", o.Count,
			"item", o.ItemID)
	}
	gold, err := s.svc.items.Gold(ctx, characterID)
	if err != nil {
		return err
	}
	s.inventory, s.gold = inv, gold
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

// onItemUse answers a double-click: an item that is worn is put on, one worn is taken
// off into the first cells it fits, and a potion is drunk. Using anything else is not
// done yet.
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
	case p.HP > 0 || p.SP > 0:
		return s.drink(it)
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
	if err := s.conn.Send(append(packets, s.followTaskbar()...)...); err != nil {
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
	s.svc.presence.dress(s.playing.vid, outfitOf(s.inventory))
}

// outfit is what a character wears, each by vnum, 0 for nothing.
type outfit struct {
	weapon, body, head, feet, shield uint32
}

func outfitOf(inv item.Inventory) outfit {
	worn := func(place item.Wear) uint32 {
		it, _ := inv.WornIn(place)
		return it.VNum
	}
	return outfit{weapon: worn(item.Weapon), body: worn(item.Body), head: worn(item.Head), feet: worn(item.Feet),
		shield: worn(item.Shield)}
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

// itemPackets are the packets the client sends from its inventory window and taskbar.
// Those the game does not do yet, the old drop its scripts never send and trading, are
// dropped, and so is any of them still in flight after Change character.
var itemPackets = map[protocol.ClientHeader]bool{
	protocol.ClientItemUse: true, protocol.ClientItemDrop: true, protocol.ClientItemMove: true,
	protocol.ClientQuickslotAdd: true, protocol.ClientQuickslotDel: true, protocol.ClientQuickslotSwap: true,
	protocol.ClientItemDrop2: true, protocol.ClientExchange: true, protocol.ClientItemUseToItem: true,
	protocol.ClientGiveItem: true, protocol.ClientItemPickup: true,
}

// dress puts the outfit on the player's character: their blows and the bites on them use
// it from then on, and they and everyone who sees them see the change, at the speed their
// shoes and shield give. A buff that needs a weapon, as Enchanted Blade and Sword Aura
// do, ends with it taken out of hand. The player is told their new DEF when what they
// wear against bites changed, and their speed when it did.
func (w *presence) dress(vid uint32, o outfit) {
	w.mu.Lock()
	defer w.mu.Unlock()
	p := w.all[vid]
	c := p.char
	armourChanged := c.Body != o.body || c.Head != o.head || c.Feet != o.feet || c.Shield != o.shield
	speed := p.add.MoveSpeed
	c.Weapon, c.Body, c.Head, c.Feet, c.Shield = o.weapon, o.body, o.head, o.feet, o.shield
	held, _ := item.Lookup(o.weapon)
	for sk := range c.Buffs {
		if buff, _ := skill.Lookup(sk); !buff.Wieldable(held) {
			delete(c.Buffs, sk) // it runs only with its weapon in hand
		}
	}
	p.add.MoveSpeed, p.add.AttackSpeed = moveSpeed(c), attackSpeed(c)
	p.info.BodyPart, p.info.WeaponPart = bodyPart(*c), weaponPart(*c)
	update := protocol.EncodeCharacterUpdate(p.info, p.add, affectsOf(c))
	p.send(update)
	if armourChanged {
		p.send(protocol.EncodePointChange(vid, protocol.PointDefence, 0, int32(defence(c)))) //nolint:gosec // small
	}
	if p.add.MoveSpeed != speed {
		p.send(protocol.EncodePointChange(vid, protocol.PointMoveSpeed, 0, int32(p.add.MoveSpeed)))
	}
	toViewers(p, update)
}

// defence is what a character sets against a bite, as the character window's DEF shows
// it: their level, vitality, the armour they wear and what their buffs add.
func defence(c *account.Character) uint32 {
	return uint32(c.Level) + uint32(c.Vitality) + uint32(armourOf(c))
}

// armourOf is the armour a character sets against a bite: their body armour's, helmet's,
// shoes' and shield's, and their buffs'.
func armourOf(c *account.Character) uint16 {
	defence := buffBonus(c, skill.Defence)
	for _, vnum := range []uint32{c.Body, c.Head, c.Feet, c.Shield} {
		worn, _ := item.Lookup(vnum)
		defence += worn.Defence
	}
	return defence
}

// moveSpeed is the speed the character moves at: everyone's, what their shoes and buffs
// add and what their shield takes off.
func moveSpeed(c *account.Character) uint8 {
	points, _ := c.Points()
	shoes, _ := item.Lookup(c.Feet)
	shield, _ := item.Lookup(c.Shield)
	return uint8(int(points.MoveSpeed) + int(shoes.MoveSpeed) + int(shield.MoveSpeed) + int(buffBonus(c, skill.MoveSpeed))) //nolint:gosec // 150 give or take 25
}

// attackSpeed is the speed the character strikes at: everyone's, and what their buffs add.
func attackSpeed(c *account.Character) uint8 {
	points, _ := c.Points()
	return points.AttackSpeed + uint8(buffBonus(c, skill.AttackSpeed)) //nolint:gosec // at most 25 more
}

// attackSpeedOf is the speed the player strikes at now, for the time their blows must
// leave between them.
func (w *presence) attackSpeedOf(vid uint32) uint8 {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.all[vid].add.AttackSpeed
}

// wear puts on the character what the inventory has worn, as it enters the world.
func wear(c *account.Character, inv item.Inventory) {
	o := outfitOf(inv)
	c.Weapon, c.Body, c.Head, c.Feet, c.Shield = o.weapon, o.body, o.head, o.feet, o.shield
}
