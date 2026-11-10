package game

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"net"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/SatoriAI/MT2-Go/internal/account"
	"github.com/SatoriAI/MT2-Go/internal/item"
	"github.com/SatoriAI/MT2-Go/internal/network"
	"github.com/SatoriAI/MT2-Go/internal/protocol"
)

// tradeStore stands in for the store's items in a trade: it keeps the last trade asked
// for, and refuses it with fail set. What it holds for each character, by id, is what
// reading them again gives.
type tradeStore struct {
	Items // the rest is never called
	kept  item.Trade
	gold  [2]uint32 // what each side holds before the trade
	fail  error
	held  map[uint32]item.Inventory
	yang  map[uint32]uint32
}

func (k *tradeStore) ApplyOrders(_ context.Context, id uint32) (item.Inventory, []item.Order, error) {
	return slices.Clone(k.held[id]), nil, nil
}

func (k *tradeStore) Gold(_ context.Context, id uint32) (uint32, error) { return k.yang[id], nil }

func (k *tradeStore) Trade(_ context.Context, t item.Trade) ([2]uint32, error) {
	if k.fail != nil {
		return [2]uint32{}, k.fail
	}
	k.kept = t
	return [2]uint32{k.gold[0] - t[0].Gold + t[1].Gold, k.gold[1] - t[1].Gold + t[0].Gold}, nil
}

// side is one player of a trade test: their session, client and world record.
type side struct {
	s      *session
	client net.Conn
	p      *present
}

// traders are Hendra and Kira, of one kingdom, 500 apart in a world of no monsters, at
// noon: Hendra with a sword, two cells tall, at cell 0 on her taskbar's slot 0, and 1000
// yang; Kira with 30 potions at cell 3 and 50 yang.
func traders(t *testing.T) (*tradeStore, side, side) {
	t.Helper()
	svc := worldWith(t)
	store := &tradeStore{gold: [2]uint32{1000, 50}, held: map[uint32]item.Inventory{}, yang: map[uint32]uint32{}}
	svc.items, svc.now = store, func() time.Time { return noon }
	svc.taskbar = &keptTaskbar{slots: map[uint8]account.Shortcut{}}
	player := func(vid, id uint32, name string, x int32, inv item.Inventory, gold uint32) side {
		client, p := aPlayer(t, svc, vid, id, name, x)
		s := &session{svc: svc, conn: p.out.(*network.Conn), phase: protocol.PhaseGame, playing: playingAs(p),
			inventory: inv, gold: gold, taskbar: map[uint8]shownSlot{}}
		store.held[id], store.yang[id] = slices.Clone(inv), gold
		return side{s, client, p}
	}
	hendra := player(1000, 7, "Hendra", 0, item.Inventory{{ID: 1, VNum: 10, Count: 1, Cell: 0}}, 1000)
	hendra.s.taskbar[0] = shownSlot{account.Shortcut{Item: 1}, 0}
	kira := player(1001, 8, "Kira", 500, item.Inventory{{ID: 2, VNum: 27001, Count: 30, Cell: 3}}, 50)
	drain(t, hendra.client)
	drain(t, kira.client)
	return store, hendra, kira
}

func exchanging(sub byte, arg uint32, shown uint8, cell uint16) []byte {
	body := binary.LittleEndian.AppendUint32([]byte{sub}, arg)
	return binary.LittleEndian.AppendUint16(append(body, shown, 1), cell)
}

// do sends what the player does in the trade.
func (p side) do(t *testing.T, body []byte) {
	t.Helper()
	if err := p.s.Handle(context.Background(), protocol.Frame{Header: protocol.ClientExchange, Body: body}); err != nil {
		t.Fatal(err)
	}
}

// told cuts what the player was told since last asked into packets.
func (p side) told(t *testing.T) [][]byte {
	t.Helper()
	return packets(t, drain(t, p.client))
}

func exNews(n protocol.ExchangeNews) []byte { return protocol.EncodeExchange(n) }

func holds(got [][]byte, want ...[]byte) bool {
	for _, w := range want {
		if !slices.ContainsFunc(got, func(p []byte) bool { return bytes.Equal(p, w) }) {
			return false
		}
	}
	return true
}

// started is traders with Hendra having asked Kira to trade, and what both were told
// drained.
func started(t *testing.T) (*tradeStore, side, side) {
	t.Helper()
	store, hendra, kira := traders(t)
	hendra.do(t, exchanging(protocol.ExchangeStart, 1001, 0, 0))
	drain(t, hendra.client)
	drain(t, kira.client)
	return store, hendra, kira
}

// at moves the world's clock to noon and d.
func (p side) at(d time.Duration) { p.s.svc.now = func() time.Time { return noon.Add(d) } }

// Asking another player to trade opens the trade window on both sides at once, each
// naming the other.
func TestATradeOpensOnBothSidesAtOnce(t *testing.T) {
	_, hendra, kira := traders(t)

	hendra.do(t, exchanging(protocol.ExchangeStart, 1001, 0, 0))

	if got := hendra.told(t); !holds(got, exNews(protocol.ExchangeNews{Sub: protocol.ExchangeStarted, Arg: 1001})) {
		t.Fatalf("Hendra told %x", got)
	}
	if got := kira.told(t); !holds(got, exNews(protocol.ExchangeNews{Sub: protocol.ExchangeStarted, Arg: 1000})) {
		t.Fatalf("Kira told %x", got)
	}
}

// A trade is refused with a line with a player of another kingdom, one already trading,
// and one refusing trades; and without one with a player out of reach, dead, or oneself.
func TestATradeIsRefusedWhereItMayNotBe(t *testing.T) {
	cases := map[string]struct {
		change func(kira side)
		with   uint32
		why    string
	}{
		"of another kingdom": {func(k side) { k.p.info.Empire = 2 }, 1001, "Nie możesz handlować z graczem z innego królestwa."},
		"already trading": {func(k side) {
			aPlayer(t, k.s.svc, 1002, 9, "Ola", 700)
			k.s.svc.record.startTrade(1002, 1001)
		}, 1001, "Ten gracz już handluje."},
		"refusing trades": {func(k side) { k.p.blocks = account.Blocks{account.BlockExchange} }, 1001, "Ten gracz nie przyjmuje propozycji handlu."},
		"out of reach":    {func(k side) { k.p.add.X = homeX + tradeReach + 1 }, 1001, ""},
		"dead":            {func(k side) { k.p.char.HP = 0 }, 1001, ""},
		"herself":         {func(side) {}, 1000, ""},
	}
	for name, c := range cases {
		_, hendra, kira := traders(t)
		c.change(kira)
		drain(t, hendra.client)

		hendra.do(t, exchanging(protocol.ExchangeStart, c.with, 0, 0))

		got := hendra.told(t)
		if c.why != "" && (len(got) != 1 || !holds(got, infoLine(c.why))) || c.why == "" && len(got) != 0 {
			t.Errorf("%s: told %x", name, got)
		}
	}
}

// Asking again to trade with the one traded with changes nothing, as dragging a second
// item onto them does; asking someone else is refused with a line.
func TestAskingToTradeWhileTrading(t *testing.T) {
	_, hendra, kira := started(t)
	aPlayer(t, hendra.s.svc, 1002, 9, "Ola", 700)
	drain(t, hendra.client)
	drain(t, kira.client)

	hendra.do(t, exchanging(protocol.ExchangeStart, 1001, 0, 0))
	if got := hendra.told(t); len(got) != 0 || len(kira.told(t)) != 0 {
		t.Fatalf("told %x", got)
	}
	hendra.do(t, exchanging(protocol.ExchangeStart, 1002, 0, 0))
	if got := hendra.told(t); len(got) != 1 || !holds(got, infoLine("Już handlujesz.")) {
		t.Fatalf("told %x", got)
	}
}

// An item offered is shown on both sides, at the trade window cell it was dropped on when
// it fits there and else in the first it fits; an item worn, or offered already, is not.
func TestAnItemOfferedIsShownToBothSides(t *testing.T) {
	_, hendra, kira := started(t)
	hendra.s.inventory = append(hendra.s.inventory, item.Item{ID: 3, VNum: 11, Count: 1, Worn: item.Weapon},
		item.Item{ID: 4, VNum: 27001, Count: 5, Cell: 1})

	hendra.do(t, exchanging(protocol.ExchangeAdd, 0, 2, wornCells[item.Weapon]))
	hendra.do(t, exchanging(protocol.ExchangeAdd, 0, 9, 0)) // two cells tall from the bottom row: in the first free
	hendra.do(t, exchanging(protocol.ExchangeAdd, 0, 4, 1)) // where the sword stands: in the first free
	hendra.do(t, exchanging(protocol.ExchangeAdd, 0, 7, 0)) // again

	sword := protocol.ExchangeNews{Sub: protocol.ExchangeAdded, Mine: true, Arg: 10, Shown: 0, Count: 1}
	potions := protocol.ExchangeNews{Sub: protocol.ExchangeAdded, Mine: true, Arg: 27001, Shown: 1, Count: 5}
	if got := hendra.told(t); len(got) != 2 || !holds(got, exNews(sword), exNews(potions)) {
		t.Fatalf("Hendra told %x", got)
	}
	sword.Mine, potions.Mine = false, false
	if got := kira.told(t); len(got) != 2 || !holds(got, exNews(sword), exNews(potions)) {
		t.Fatalf("Kira told %x", got)
	}
}

// Yang is offered once and no more than is held, the other side told the amount in full.
func TestYangIsOfferedOnceAndNoMoreThanHeld(t *testing.T) {
	_, hendra, kira := started(t)

	hendra.do(t, exchanging(protocol.ExchangeYang, 1001, 0, 0))
	if got := hendra.told(t); !holds(got, exNews(protocol.ExchangeNews{Sub: protocol.ExchangeLessYang, Mine: true}), infoLine("Nie masz tyle yang.")) {
		t.Fatalf("too much: told %x", got)
	}
	hendra.do(t, exchanging(protocol.ExchangeYang, 1000, 0, 0))
	hendra.do(t, exchanging(protocol.ExchangeYang, 10, 0, 0))

	if got := hendra.told(t); len(got) != 2 || !holds(got, exNews(protocol.ExchangeNews{Sub: protocol.ExchangeYangSet, Mine: true, Arg: 1000}),
		infoLine("Nie możesz teraz zmienić kwoty.")) {
		t.Fatalf("Hendra told %x", got)
	}
	if got := kira.told(t); len(got) != 2 || !holds(got, exNews(protocol.ExchangeNews{Sub: protocol.ExchangeYangSet, Arg: 1000}),
		infoLine("Hendra oferuje 1.000 yang.")) {
		t.Fatalf("Kira told %x", got)
	}
}

// Two tall items offered never stand over each other in a column of the trade window.
func TestTallItemsOfferedNeverOverlap(t *testing.T) {
	sword, _ := item.Lookup(10)
	potion, _ := item.Lookup(27001)
	s := &trader{offer: []offered{{item.Item{ID: 1, VNum: 10}, 4}}} // in cells 4 and 8

	for _, c := range []struct {
		p    item.Proto
		cell uint8
		fits bool
	}{{sword, 0, false}, {sword, 1, true}, {potion, 8, false}, {potion, 0, true}, {sword, 9, false}} {
		if s.fits(c.p, c.cell) != c.fits {
			t.Errorf("kind %d at %d: fits %v", c.p.VNum, c.cell, !c.fits)
		}
	}
}

func TestYangIsWrittenInFull(t *testing.T) {
	for n, want := range map[uint32]string{7: "7", 999: "999", 1000: "1.000", 2_000_000_000: "2.000.000.000"} {
		if got := inFull(n); got != want {
			t.Errorf("%d written %q", n, got)
		}
	}
}

// An accept is refused within 2 s of the last change, the button given back; at 2 s it
// counts, both told.
func TestAnAcceptSoonAfterAChangeIsRefused(t *testing.T) {
	_, hendra, kira := started(t)
	hendra.do(t, exchanging(protocol.ExchangeYang, 100, 0, 0))
	drain(t, kira.client)

	kira.at(acceptAfter - time.Millisecond)
	kira.do(t, exchanging(protocol.ExchangeAccept, 0, 0, 0))
	if got := kira.told(t); !holds(got, infoLine("Oferta właśnie się zmieniła. Poczekaj chwilę."),
		exNews(protocol.ExchangeNews{Sub: protocol.ExchangeAccepted, Mine: true})) {
		t.Fatalf("too soon: told %x", got)
	}
	kira.at(acceptAfter)
	kira.do(t, exchanging(protocol.ExchangeAccept, 0, 0, 0))

	if got := kira.told(t); !holds(got, exNews(protocol.ExchangeNews{Sub: protocol.ExchangeAccepted, Mine: true, Arg: 1})) {
		t.Fatalf("Kira told %x", got)
	}
	if got := hendra.told(t); !holds(got, exNews(protocol.ExchangeNews{Sub: protocol.ExchangeAccepted, Arg: 1})) {
		t.Fatalf("Hendra told %x", got)
	}
}

// A change to either offer takes back both accepts, both sides told.
func TestAChangeTakesBackTheAccepts(t *testing.T) {
	_, hendra, kira := started(t)
	kira.at(acceptAfter)
	kira.do(t, exchanging(protocol.ExchangeAccept, 0, 0, 0))
	drain(t, hendra.client)
	drain(t, kira.client)

	hendra.do(t, exchanging(protocol.ExchangeYang, 100, 0, 0))

	if got := kira.told(t); !holds(got, exNews(protocol.ExchangeNews{Sub: protocol.ExchangeAccepted, Mine: true})) {
		t.Fatalf("Kira told %x", got)
	}
	if got := hendra.told(t); !holds(got, exNews(protocol.ExchangeNews{Sub: protocol.ExchangeAccepted})) {
		t.Fatalf("Hendra told %x", got)
	}
	if hendra.do(t, exchanging(protocol.ExchangeAccept, 0, 0, 0)); len(hendra.told(t)) != 2 { // within 2 s of the change
		t.Fatal("an accept counted after the change")
	}
}

// An accept is refused, both sides told, when what the other side offers would not fit:
// no room in the inventory, with what the player offers counted as gone, or yang past
// what they may hold.
func TestAnAcceptOfWhatWouldNotFitIsRefused(t *testing.T) {
	cases := map[string]struct {
		change func(hendra, kira side)
		why    string
	}{
		"no room":       {func(_, k side) { k.s.inventory = fullBag() }, "Nie masz miejsca w ekwipunku."},
		"too much yang": {func(_, k side) { k.s.gold = maxGold - 99 }, "Miałbyś za dużo yang."},
		"as much yang as she gives": {func(_, k side) {
			k.s.gold = maxGold
			k.do(t, exchanging(protocol.ExchangeYang, 100, 0, 0))
		}, ""},
		"room for one of two": {func(h, k side) { // the sword fits at 0 and 5, and then the potions nowhere
			k.s.inventory = slices.DeleteFunc(fullBag(), func(it item.Item) bool { return it.Cell == 0 || it.Cell == 5 })
			h.s.inventory = append(h.s.inventory, item.Item{ID: 5, VNum: 27001, Count: 2, Cell: 1})
			h.do(t, exchanging(protocol.ExchangeAdd, 0, 1, 1))
		}, "Nie masz miejsca w ekwipunku."},
		"room made by what she gives": {func(_, k side) { // two cells, one above the other, for the sword
			k.s.inventory = fullBag()
			k.do(t, exchanging(protocol.ExchangeAdd, 0, 0, 3))
			k.do(t, exchanging(protocol.ExchangeAdd, 0, 1, 8))
		}, ""},
	}
	for name, c := range cases {
		_, hendra, kira := started(t)
		hendra.do(t, exchanging(protocol.ExchangeAdd, 0, 0, 0))
		hendra.do(t, exchanging(protocol.ExchangeYang, 100, 0, 0))
		c.change(hendra, kira)
		drain(t, hendra.client)
		drain(t, kira.client)

		kira.at(acceptAfter)
		kira.do(t, exchanging(protocol.ExchangeAccept, 0, 0, 0))

		refused := holds(hendra.told(t), infoLine("Kira nie może przyjąć tej oferty."))
		if got := kira.told(t); refused != (c.why != "") || c.why != "" && !holds(got, infoLine(c.why)) {
			t.Errorf("%s: refused %v, Kira told %x", name, refused, got)
		}
	}
}

// fullBag is Kira's potions at cell 3 and a potion in every other cell.
func fullBag() item.Inventory {
	bag := item.Inventory{{ID: 2, VNum: 27001, Count: 30, Cell: 3}}
	for cell := range uint16(item.Cells) {
		if cell != 3 {
			bag = append(bag, item.Item{ID: int64(100 + cell), VNum: 27001, Count: 1, Cell: cell})
		}
	}
	return bag
}

// accepted is started with Hendra offering her sword and 300 yang and Kira her potions,
// Kira having accepted and then Hendra, keeping the trade; what Kira was told before
// Hendra's accept is drained.
func accepted(t *testing.T) (*tradeStore, side, side) {
	t.Helper()
	store, hendra, kira := started(t)
	hendra.do(t, exchanging(protocol.ExchangeAdd, 0, 0, 0))
	hendra.do(t, exchanging(protocol.ExchangeYang, 300, 0, 0))
	kira.do(t, exchanging(protocol.ExchangeAdd, 0, 3, 3))
	kira.at(acceptAfter)
	kira.do(t, exchanging(protocol.ExchangeAccept, 0, 0, 0))
	drain(t, kira.client)
	hendra.do(t, exchanging(protocol.ExchangeAccept, 0, 0, 0))
	return store, hendra, kira
}

// A trade both sides accept is kept whole: each item goes to the first cells it fits on
// the other side, those of what that side gives counted free, with the yang.
func TestATradeBothAcceptIsKeptWhole(t *testing.T) {
	store, _, _ := accepted(t)

	if store.kept[0].Gold != 300 || store.kept[1].Gold != 0 ||
		!reflect.DeepEqual(store.kept[0].Items, []item.Item{{ID: 1, VNum: 10, Count: 1, Cell: 0}}) ||
		!reflect.DeepEqual(store.kept[1].Items, []item.Item{{ID: 2, VNum: 27001, Count: 30, Cell: 0}}) {
		t.Fatalf("kept %+v", store.kept)
	}
}

// Each side of a trade kept is told what went and came, its new yang and its taskbar slot
// of what it gave emptied, and the window closes.
func TestEachSideIsToldWhatATradeKeptChanged(t *testing.T) {
	_, hendra, kira := accepted(t)

	ended := exNews(protocol.ExchangeNews{Sub: protocol.ExchangeEnded})
	if got := hendra.told(t); !holds(got, itemCleared(item.Item{Cell: 0}), itemSet(item.Item{ID: 2, VNum: 27001, Count: 30, Cell: 0}),
		pointTold(1000, protocol.PointGold, -300, 700), protocol.EncodeQuickslotDel(0), ended) {
		t.Fatalf("Hendra told %x", got)
	}
	if got := kira.told(t); !holds(got, itemCleared(item.Item{Cell: 3}), itemSet(item.Item{ID: 1, VNum: 10, Count: 1, Cell: 0}),
		pointTold(1001, protocol.PointGold, 300, 350), ended) {
		t.Fatalf("Kira told %x", got)
	}
}

// The side that accepted last holds what the trade changed at once.
func TestTheSideKeepingATradeHoldsItsChange(t *testing.T) {
	_, hendra, _ := accepted(t)

	if !reflect.DeepEqual(hendra.s.inventory, item.Inventory{{ID: 2, VNum: 27001, Count: 30, Cell: 0}}) || hendra.s.gold != 700 || len(hendra.s.taskbar) != 0 {
		t.Fatalf("Hendra holds %+v, %d yang, taskbar %v", hendra.s.inventory, hendra.s.gold, hendra.s.taskbar)
	}
}

// The side that accepted first holds what the trade changed from its next packet that
// touches what it holds on, applied to what it holds then: arrows it shot meanwhile stay
// shot.
func TestTheOtherSideHoldsItsChangeFromItsNextPacket(t *testing.T) {
	_, _, kira := accepted(t)
	if kira.s.gold != 50 {
		t.Fatal("Kira's connection changed before her next packet")
	}
	kira.s.inventory = append(kira.s.inventory, item.Item{ID: 9, VNum: 8000, Count: 40, Worn: item.Arrow}) // 10 of her 50 shot

	if err := kira.s.Handle(context.Background(), protocol.Frame{Header: protocol.ClientItemUse, Body: []byte{1, 9, 0}}); err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(kira.s.inventory, item.Inventory{{ID: 9, VNum: 8000, Count: 40, Worn: item.Arrow}, {ID: 1, VNum: 10, Count: 1, Cell: 0}}) || kira.s.gold != 350 {
		t.Fatalf("Kira holds %+v and %d yang", kira.s.inventory, kira.s.gold)
	}
}

// A trade the store cannot keep changes nothing, and both sides are told; each reads
// again what it holds, as the database may have kept it after all.
func TestATradeThatCannotBeKeptChangesNothing(t *testing.T) {
	store, hendra, kira := started(t)
	store.fail, store.yang[7], store.yang[8] = errors.New("the database is away"), 999, 49
	hendra.do(t, exchanging(protocol.ExchangeYang, 300, 0, 0))
	kira.at(acceptAfter)
	kira.do(t, exchanging(protocol.ExchangeAccept, 0, 0, 0))
	drain(t, kira.client)
	hendra.do(t, exchanging(protocol.ExchangeAccept, 0, 0, 0))

	for _, p := range []side{hendra, kira} {
		if got := p.told(t); !holds(got, exNews(protocol.ExchangeNews{Sub: protocol.ExchangeEnded}), infoLine("Handel nie powiódł się.")) {
			t.Fatalf("%s told %x", p.p.info.Name, got)
		}
	}
	if _, err := kira.s.catchUp(context.Background()); err != nil {
		t.Fatal(err)
	}
	if hendra.s.gold != 999 || len(hendra.s.inventory) != 1 || kira.s.gold != 49 || kira.s.inventory[0].ID != 2 {
		t.Fatal("what they hold was not read again")
	}
	if len(hendra.s.svc.record.state.trades) != 0 {
		t.Fatal("the trade goes on")
	}
}

// While trading, a packet that would change what either side holds is refused with a
// line, also while the trade is being kept, but closing a shop is not; and once it is
// being kept, the offers change no more.
func TestWhileTradingWhatIsHeldStaysAsItIs(t *testing.T) {
	_, hendra, kira := started(t)
	refused := func(p side, f protocol.Frame) bool {
		t.Helper()
		if err := p.s.Handle(context.Background(), f); err != nil {
			t.Fatal(err)
		}
		got := p.told(t)
		return len(got) == 1 && holds(got, infoLine("Nie możesz tego zrobić podczas handlu."))
	}
	drag := protocol.Frame{Header: protocol.ClientItemMove, Body: dragged(0, 17, 1)}

	if !refused(hendra, drag) || !refused(kira, drag) || refused(kira, protocol.Frame{Header: protocol.ClientShop, Body: []byte{protocol.ShopEnd}}) {
		t.Fatal("refused wrongly while trading")
	}
	kira.at(acceptAfter)
	kira.do(t, exchanging(protocol.ExchangeAccept, 0, 0, 0))
	if hendra.s.svc.record.accept(1000, noon.Add(acceptAfter), hendra.s.plan) == nil {
		t.Fatal("not being kept")
	}
	drain(t, kira.client)
	if !refused(kira, drag) {
		t.Fatal("a drag passed while the trade was being kept")
	}
	if kira.do(t, exchanging(protocol.ExchangeYang, 10, 0, 0)); len(kira.told(t)) != 0 {
		t.Fatal("the offer changed while the trade was being kept")
	}
}

// A trade ends, both windows closed, when a side cancels, walks more than 1000 along an
// axis from where it began, dies, or leaves the world; the other side is told why.
func TestATradeEnds(t *testing.T) {
	cases := map[string]struct {
		end func(hendra, kira side)
		why string
	}{
		"cancelled": {func(h, _ side) { h.do(t, exchanging(protocol.ExchangeCancel, 0, 0, 0)) }, "Handel został anulowany."},
		"walked away": {func(h, _ side) {
			h.s.svc.record.move(1000, protocol.Move{X: homeX - walkAway - 1, Y: homeY})
		}, "Handel został przerwany."},
		"died": {func(h, _ side) {
			h.s.svc.record.held(func(l locked) {
				h.p.char.HP = 0
				l.die(h.p)
			})
		}, "Handel został przerwany."},
		"left": {func(h, _ side) { h.s.leaveTrade() }, "Handel został anulowany."},
	}
	for name, c := range cases {
		_, hendra, kira := started(t)

		c.end(hendra, kira)

		if got := kira.told(t); !holds(got, exNews(protocol.ExchangeNews{Sub: protocol.ExchangeEnded}), infoLine(c.why)) {
			t.Errorf("%s: Kira told %x", name, got)
		}
		if got := hendra.told(t); !holds(got, exNews(protocol.ExchangeNews{Sub: protocol.ExchangeEnded})) {
			t.Errorf("%s: Hendra told %x", name, got)
		}
		if len(kira.s.svc.record.state.trades) != 0 {
			t.Errorf("%s: the trade goes on", name)
		}
	}
}

// A walk of 1000 along each axis keeps the trade going.
func TestATradeGoesOnWithinItsWalk(t *testing.T) {
	_, hendra, _ := started(t)

	hendra.s.svc.record.move(1000, protocol.Move{X: homeX - walkAway, Y: homeY + walkAway})

	if len(hendra.s.svc.record.state.trades) != 2 {
		t.Fatal("the trade ended")
	}
}

// A side leaving while the trade is being kept waits until it is, then holds what it got,
// so it never leaves before the trade is stored.
func TestLeavingWhileATradeIsKeptWaitsForIt(t *testing.T) {
	_, hendra, kira := started(t)
	w := &hendra.s.svc.record
	tr := w.state.trades[1000]
	hendra.do(t, exchanging(protocol.ExchangeYang, 300, 0, 0))
	kira.at(acceptAfter)
	kira.do(t, exchanging(protocol.ExchangeAccept, 0, 0, 0))
	if w.accept(1000, noon.Add(acceptAfter), hendra.s.plan) != tr {
		t.Fatal("the trade is not to be kept")
	}
	left := make(chan struct{})
	go func() {
		kira.s.leaveTrade()
		close(left)
	}()
	select { // a moment is enough for a leave that does not wait to be done
	case <-left:
		t.Fatal("left before the trade was kept")
	case <-time.After(50 * time.Millisecond):
	}

	if err := hendra.s.keepTrade(context.Background(), tr); err != nil {
		t.Fatal(err)
	}

	<-left
	if hendra.s.gold != 700 || len(w.state.settled) != 0 || len(w.state.trades) != 0 {
		t.Fatalf("Hendra holds %d yang; %d settlements wait", hendra.s.gold, len(w.state.settled))
	}
}
