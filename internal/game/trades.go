package game

import (
	"fmt"
	"slices"
	"strconv"
	"time"

	"github.com/SatoriAI/MT2-Go/internal/account"
	"github.com/SatoriAI/MT2-Go/internal/item"
	"github.com/SatoriAI/MT2-Go/internal/protocol"
)

const (
	// tradeReach is how near another player must be to trade, as near as the client's
	// target board offers it.
	tradeReach = 3000
	// walkAway is how far, along either axis, a side may walk from where the trade began
	// before it ends, as the client ends it too.
	walkAway = 1000
	// acceptAfter is how long after the last change to either offer an accept counts, so
	// a last-moment change cannot be accepted unseen.
	acceptAfter = 2 * time.Second
	// The trade window is 4 cells wide and 3 high for each side; an item stands in its
	// column from its top cell down.
	tradeCells = 12
	tradeRow   = 4
)

// trade is two players trading, which the world record keeps under both their vids.
type trade struct {
	sides   [2]*trader
	changed time.Time // when either offer last changed
	// done is made once both have accepted and the trade is being kept, and closed once it
	// is: from then on nothing changes it but the side keeping it.
	done chan struct{}
}

// trader is one side of a trade: what it offers, and once it accepts, where each item the
// other side offers goes in its inventory and which of its taskbar slots hold what it
// offers.
type trader struct {
	p            *present
	fromX, fromY int32 // where it stood as the trade began
	offer        []offered
	gold         uint32
	accepted     bool
	places       []uint16
	slots        []uint8
}

// offered is an item offered, at its cell in the inventory, and the trade window cell it
// is shown at.
type offered struct {
	it    item.Item
	shown uint8
}

// settlement is what a trade kept changes for one side, for its connection to apply to
// what it holds then: the items it gave go, those it got come at their cells, its yang
// moves by gold, and the taskbar slots that held what it gave empty. A connection may
// have changed what it holds since it accepted, as shooting uses arrows up, so it is a
// change rather than what the side held. A trade that could not be kept leaves reload,
// for the connection to read again what it holds, as the database may have kept it after
// all.
type settlement struct {
	gave, got []item.Item
	gold      int64
	slots     []uint8
	reload    bool
}

func (t *trade) side(vid uint32) (me, other *trader) {
	if t.sides[0].p.add.VID == vid {
		return t.sides[0], t.sides[1]
	}
	return t.sides[1], t.sides[0]
}

// tell sends the news to both sides, each as its own or the other's.
func (t *trade) tell(from *trader, n protocol.ExchangeNews) {
	for _, s := range t.sides {
		n.Mine = s == from
		s.p.send(protocol.EncodeExchange(n))
	}
}

// changedAt clears both accepts, telling both sides of each taken back, as an offer
// changes at now.
func (t *trade) changedAt(now time.Time) {
	t.changed = now
	for _, s := range t.sides {
		if s.accepted {
			s.accepted, s.places, s.slots = false, nil, nil
			t.tell(s, protocol.ExchangeNews{Sub: protocol.ExchangeAccepted})
		}
	}
}

func line(p *present, text string) { p.send(infoLine(text)) }

func infoLine(text string) []byte {
	return protocol.EncodeChat(protocol.ChatInfo, 0, 0, textToClient(text))
}

// startTrade opens a trade between the player vid and the player with: both alive, of one
// kingdom, on one map within tradeReach, neither trading already and with not refusing
// trades. Anything else is refused, with a line where the player could not know why;
// asking again to trade with the one already traded with changes nothing, as dragging an
// item onto them does.
func (w *presence) startTrade(vid, with uint32) {
	w.mu.Lock()
	defer w.mu.Unlock()
	p, o := w.all[vid], w.all[with]
	if o == nil || o == p || !o.isPlayer() || o.world != p.world || p.dead() || o.dead() || !within(p, o, tradeReach) {
		return
	}
	if t := w.trades[vid]; t != nil {
		if _, other := t.side(vid); other.p != o {
			line(p, "Już handlujesz.")
		}
		return
	}
	switch {
	case o.info.Empire != p.info.Empire:
		line(p, "Nie możesz handlować z graczem z innego królestwa.")
		return
	case w.trades[with] != nil:
		line(p, "Ten gracz już handluje.")
		return
	case o.blocks.Has(account.BlockExchange):
		line(p, "Ten gracz nie przyjmuje propozycji handlu.")
		return
	}
	t := &trade{sides: [2]*trader{{p: p, fromX: p.add.X, fromY: p.add.Y}, {p: o, fromX: o.add.X, fromY: o.add.Y}}}
	w.trades[vid], w.trades[with] = t, t
	p.send(protocol.EncodeExchange(protocol.ExchangeNews{Sub: protocol.ExchangeStarted, Arg: with}))
	o.send(protocol.EncodeExchange(protocol.ExchangeNews{Sub: protocol.ExchangeStarted, Arg: vid}))
}

// offerItem adds an item the player carries to their offer: at the trade window cell they
// dropped it on if it fits there, or the first it fits, once.
func (w *presence) offerItem(vid uint32, it item.Item, shown uint8, now time.Time) {
	w.mu.Lock()
	defer w.mu.Unlock()
	t := w.trades[vid]
	if t == nil || t.done != nil {
		return
	}
	me, _ := t.side(vid)
	if slices.ContainsFunc(me.offer, func(o offered) bool { return o.it.ID == it.ID }) {
		return
	}
	p, _ := item.Lookup(it.VNum)
	if !me.fits(p, shown) {
		var ok bool
		if shown, ok = me.free(p); !ok {
			line(me.p, "Nie ma już miejsca w oknie handlu.")
			return
		}
	}
	me.offer = append(me.offer, offered{it, shown})
	t.changedAt(now)
	t.tell(me, protocol.ExchangeNews{Sub: protocol.ExchangeAdded, Arg: it.VNum, Shown: shown, Count: it.Count})
}

// fits reports whether an item of kind p shown from cell down stays in the trade window
// and on no cell another item offered stands in.
func (s *trader) fits(p item.Proto, cell uint8) bool {
	if int(cell)+(int(p.Size)-1)*tradeRow >= tradeCells {
		return false
	}
	for _, o := range s.offer {
		other, _ := item.Lookup(o.it.VNum)
		if o.shown%tradeRow == cell%tradeRow && o.shown <= cell+(p.Size-1)*tradeRow && cell <= o.shown+(other.Size-1)*tradeRow {
			return false
		}
	}
	return true
}

func (s *trader) free(p item.Proto) (uint8, bool) {
	for cell := range uint8(tradeCells) {
		if s.fits(p, cell) {
			return cell, true
		}
	}
	return 0, false
}

// offerYang sets the yang the player offers, once, to no more than they hold.
func (w *presence) offerYang(vid, amount, held uint32, now time.Time) {
	w.mu.Lock()
	defer w.mu.Unlock()
	t := w.trades[vid]
	if t == nil || t.done != nil || amount == 0 {
		return
	}
	me, other := t.side(vid)
	switch {
	case me.gold != 0:
		line(me.p, "Nie możesz teraz zmienić kwoty.")
		return
	case amount > held:
		me.p.send(protocol.EncodeExchange(protocol.ExchangeNews{Sub: protocol.ExchangeLessYang, Mine: true}))
		line(me.p, "Nie masz tyle yang.")
		return
	}
	me.gold = amount
	t.changedAt(now)
	t.tell(me, protocol.ExchangeNews{Sub: protocol.ExchangeYangSet, Arg: amount})
	line(other.p, fmt.Sprintf("%s oferuje %s yang.", me.p.info.Name, inFull(amount)))
}

// inFull writes yang with a dot between each three digits, as the trade window does.
func inFull(n uint32) string {
	s := strconv.FormatUint(uint64(n), 10)
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "." + s[i:]
	}
	return s
}

// plan is where each item the other side offers would go in a side's inventory as it
// accepts, and which of its taskbar slots hold what it offers, or the line telling both
// sides why it cannot take them. It runs under the world's lock, which is safe as only the
// side's own connection touches what it holds.
type plan func(mine []offered, gave uint32, theirs []offered, gets uint32) ([]uint16, []uint8, string)

// accept takes the player's accept, at least acceptAfter since the last change and when
// what the other side offers fits them. An accept refused is taken back on their screen.
// Once both have accepted, accept returns the trade to keep, which then changes no more.
func (w *presence) accept(vid uint32, now time.Time, planned plan) *trade {
	w.mu.Lock()
	defer w.mu.Unlock()
	t := w.trades[vid]
	if t == nil || t.done != nil {
		return nil
	}
	me, other := t.side(vid)
	if me.accepted {
		return nil
	}
	refused := func() {
		me.p.send(protocol.EncodeExchange(protocol.ExchangeNews{Sub: protocol.ExchangeAccepted, Mine: true}))
	}
	if now.Sub(t.changed) < acceptAfter {
		line(me.p, "Oferta właśnie się zmieniła. Poczekaj chwilę.")
		refused()
		return nil
	}
	places, slots, why := planned(me.offer, me.gold, other.offer, other.gold)
	if why != "" {
		line(me.p, why)
		line(other.p, fmt.Sprintf("%s nie może przyjąć tej oferty.", me.p.info.Name))
		refused()
		return nil
	}
	me.accepted, me.places, me.slots = true, places, slots
	t.tell(me, protocol.ExchangeNews{Sub: protocol.ExchangeAccepted, Arg: 1})
	if !other.accepted {
		return nil
	}
	t.done = make(chan struct{})
	return t
}

// cancelTrade ends the player's trade, the other side told so.
func (w *presence) cancelTrade(vid uint32) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if t := w.trades[vid]; t != nil && t.done == nil {
		_, other := t.side(vid)
		w.endTrade(t)
		line(other.p, "Handel został anulowany.")
	}
}

// breakTrade ends p's trade, unless it is being kept, both sides told it broke off: as p
// died or walked away from it. Called under the lock.
func (w *presence) breakTrade(p *present) {
	if t := w.trades[p.add.VID]; t != nil && t.done == nil {
		w.endTrade(t)
		for _, s := range t.sides {
			line(s.p, "Handel został przerwany.")
		}
	}
}

// walkedAway breaks p's trade once p is further than walkAway along either axis from
// where it began. Called under the lock, after every move of a player.
func (w *presence) walkedAway(p *present) {
	t := w.trades[p.add.VID]
	if t == nil {
		return
	}
	me, _ := t.side(p.add.VID)
	if max(p.add.X-me.fromX, me.fromX-p.add.X, p.add.Y-me.fromY, me.fromY-p.add.Y) > walkAway {
		w.breakTrade(p)
	}
}

// endTrade takes the trade out of the world record and closes both windows. Called under
// the lock.
func (w *presence) endTrade(t *trade) {
	for _, s := range t.sides {
		delete(w.trades, s.p.add.VID)
		s.p.send(protocol.EncodeExchange(protocol.ExchangeNews{Sub: protocol.ExchangeEnded}))
	}
}

// leaveTrade ends the trade of a player leaving the world, the other side told so; for a
// trade being kept it returns the channel that closes once it is, which the player must
// wait for, so they never leave before what they then hold is stored.
func (w *presence) leaveTrade(vid uint32) <-chan struct{} {
	w.mu.Lock()
	defer w.mu.Unlock()
	t := w.trades[vid]
	if t == nil {
		return nil
	}
	if t.done != nil {
		return t.done
	}
	_, other := t.side(vid)
	w.endTrade(t)
	line(other.p, "Handel został anulowany.")
	return nil
}

// catchUp hands the player what a trade the other side kept changed for them, if one did
// since they last asked, and reports whether they are trading.
func (w *presence) catchUp(vid uint32) (settlement, bool, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	h, kept := w.settled[vid]
	delete(w.settled, vid)
	return h, kept, w.trades[vid] != nil
}

// tradeKept closes the trade kept, with gold the yang each side then has, or failed. Each
// side gets its settlement and is told what changed and that the window closed: the side
// keeping it gets its settlement back, and the other has it left in the world record, for
// its connection to take.
func (w *presence) tradeKept(t *trade, keeper uint32, gold [2]uint32, failed bool) settlement {
	w.mu.Lock()
	defer w.mu.Unlock()
	defer close(t.done)
	ended := protocol.EncodeExchange(protocol.ExchangeNews{Sub: protocol.ExchangeEnded})
	var mine settlement
	for i, s := range t.sides {
		delete(w.trades, s.p.add.VID)
		st, told := settlement{reload: true}, [][]byte{ended, infoLine("Handel nie powiódł się.")}
		if !failed {
			st, told = s.settled(t.sides[1-i], gold[i])
			told = append(told, ended)
		}
		s.p.send(told...)
		if s.p.add.VID == keeper {
			mine = st
		} else {
			w.settled[s.p.add.VID] = st
		}
	}
	return mine
}

// settled is what the trade kept changes for the side, now holding gold yang, and what its
// client is told of it: each item it gave gone from its cell, each it got in the cell
// planned, its yang, and the taskbar slots that held what it gave emptied, as the
// database emptied them.
func (s *trader) settled(other *trader, gold uint32) (settlement, [][]byte) {
	st := settlement{gold: int64(other.gold) - int64(s.gold), slots: s.slots}
	var told [][]byte
	for _, o := range s.offer {
		st.gave = append(st.gave, o.it)
		told = append(told, itemCleared(o.it))
	}
	for i, o := range other.offer {
		got := o.it
		got.Cell = s.places[i]
		st.got = append(st.got, got)
		told = append(told, itemSet(got))
	}
	if st.gold != 0 {
		told = append(told, pointTold(s.p.add.VID, protocol.PointGold, st.gold, gold))
	}
	for _, slot := range s.slots {
		told = append(told, protocol.EncodeQuickslotDel(slot))
	}
	return st, told
}

// given is the trade as the store keeps it: each side's items, each with the cell planned
// for it on the other side, and its yang.
func (t *trade) given() item.Trade {
	var g item.Trade
	for i, s := range t.sides {
		g[i] = item.TradeSide{Character: s.p.char.ID, Gold: s.gold}
		for j, o := range s.offer {
			it := o.it
			it.Cell = t.sides[1-i].places[j]
			g[i].Items = append(g[i].Items, it)
		}
	}
	return g
}
