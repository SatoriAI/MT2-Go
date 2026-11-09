package game

import (
	"encoding/binary"
	"errors"
	"net"
	"testing"

	"github.com/SatoriAI/MT2-Go/internal/item"
	"github.com/SatoriAI/MT2-Go/internal/network"
	"github.com/SatoriAI/MT2-Go/internal/protocol"
	"github.com/SatoriAI/MT2-Go/internal/world"
)

const shopErrorSize = 4

// shopping is Hendra, with gold yang and carrying what is given, x east of a village with
// the general store at home, a monster beside it and the storekeeper, who sells nothing;
// it returns the general store's vid.
func shopping(t *testing.T, x int32, gold uint32, carrying item.Inventory) (*session, *keptItems, net.Conn, uint32) {
	t.Helper()
	s := worldWith(t, world.Spawn{VNum: 9003, X: homeX, Y: homeY, Count: 1})
	kept := &keptItems{gold: gold, count: map[int64]uint8{}}
	s.items = kept
	client, hendra := aPlayer(t, s, 1000, 7, "Hendra", x)
	if x < viewRange {
		read(t, client, addSize+infoSize) // the general store
	}
	sess := &session{svc: s, conn: hendra.out.(*network.Conn), phase: protocol.PhaseGame,
		playing: playingAs(hendra), inventory: carrying, gold: gold}
	return sess, kept, client, s.lastVID.Load() // the one character spawned
}

// open clicks the general store and reads its window.
func open(t *testing.T, s *session, client net.Conn, store uint32) {
	t.Helper()
	if err := s.onClick(vidBody(store)); err != nil {
		t.Fatal(err)
	}
	read(t, client, 1728)
}

func vidBody(vid uint32) []byte { return binary.LittleEndian.AppendUint32(nil, vid) }

func bought(pos uint8) []byte       { return []byte{protocol.ShopBuy, 1, pos} }
func sold(cell, count uint8) []byte { return []byte{protocol.ShopSell2, cell, count} }
func expectRefused(t *testing.T, c net.Conn, e protocol.ShopError) {
	t.Helper()
	if got := read(t, c, shopErrorSize); string(got) != string(protocol.EncodeShopError(e)) {
		t.Fatalf("got %x, want refusal %d", got, e)
	}
}

// A click on the general store beside her opens its window: each potion singly on the top
// row and twenty to a pack below, priced for the whole of it, the store named as its owner.
func TestAClickOnAShopkeeperOpensTheShop(t *testing.T) {
	s, _, client, store := shopping(t, 100, 0, nil)

	if err := s.onClick(vidBody(store)); err != nil {
		t.Fatal(err)
	}

	var cells [protocol.ShopCells]protocol.ShopItem
	cells[0] = protocol.ShopItem{VNum: 27001, Price: 32, Count: 1}
	cells[1] = protocol.ShopItem{VNum: 27002, Price: 96, Count: 1}
	cells[2] = protocol.ShopItem{VNum: 27004, Price: 96, Count: 1}
	cells[5] = protocol.ShopItem{VNum: 27001, Price: 640, Count: 20}
	cells[6] = protocol.ShopItem{VNum: 27002, Price: 1920, Count: 20}
	cells[7] = protocol.ShopItem{VNum: 27004, Price: 1920, Count: 20}
	if got := read(t, client, 1728); string(got) != string(protocol.EncodeShopStart(store, cells)) || s.shop != 9003 {
		t.Fatalf("shop %d, got %x", s.shop, got[:16])
	}
}

// Nothing opens from too far, to the dead, on a monster, or on an NPC who sells nothing.
func TestOnlyAShopkeeperWithinReachOpensAShop(t *testing.T) {
	far, _, farClient, store := shopping(t, npcReach+1, 0, nil)
	if err := far.onClick(vidBody(store)); err != nil {
		t.Fatal(err)
	}
	expectNothing(t, farClient)

	dead, _, deadClient, beside := shopping(t, 100, 0, nil)
	dead.svc.presence.all[1000].char.HP = 0
	if err := dead.onClick(vidBody(beside)); err != nil {
		t.Fatal(err)
	}
	expectNothing(t, deadClient)

	for vnum, shown := range map[uint16]int{101: addSize, 9005: addSize + infoSize} {
		s := worldWith(t, world.Spawn{VNum: vnum, X: homeX, Y: homeY, Count: 1})
		client, hendra := aPlayer(t, s, 1000, 7, "Hendra", 100)
		read(t, client, shown)
		sess := &session{svc: s, conn: hendra.out.(*network.Conn), phase: protocol.PhaseGame,
			playing: playingAs(hendra)}
		if err := sess.onClick(vidBody(s.lastVID.Load())); err != nil {
			t.Fatal(err)
		}
		expectNothing(t, client)
		if far.shop != 0 || dead.shop != 0 || sess.shop != 0 {
			t.Fatalf("%d opened a shop", vnum)
		}
	}
}

// A potion bought goes into the first free cell, for its price.
func TestABuyTakesTheYangAndGivesTheWare(t *testing.T) {
	s, kept, client, store := shopping(t, 100, 100, item.Inventory{{ID: 1, VNum: 10, Count: 1, Cell: 0}})
	open(t, s, client, store)

	if err := s.onShop(bought(0)); err != nil {
		t.Fatal(err)
	}

	want := item.Item{ID: 101, VNum: 27001, Count: 1, Cell: 1}
	if got := read(t, client, itemSetSize+pointChangeSize); string(got) != string(itemSet(want))+
		string(protocol.EncodePointChange(1000, protocol.PointGold, -32, 68)) {
		t.Fatalf("told %x", got)
	}
	if s.gold != 68 || kept.gold != 68 || len(s.inventory) != 2 || s.inventory[1] != want {
		t.Fatalf("has %d, kept %d, carries %+v", s.gold, kept.gold, s.inventory)
	}
}

// A pack bought joins her stack of its kind, when the stack has room for all of it.
func TestAPackBoughtJoinsHerStack(t *testing.T) {
	s, kept, client, store := shopping(t, 100, 1000, item.Inventory{{ID: 1, VNum: 27001, Count: 5, Cell: 0}})
	open(t, s, client, store)

	if err := s.onShop(bought(5)); err != nil {
		t.Fatal(err)
	}

	if got := read(t, client, itemSetSize); string(got) != string(itemSet(item.Item{ID: 1, VNum: 27001, Count: 25})) ||
		kept.count[1] != 25 || s.inventory[0].Count != 25 || s.gold != 360 {
		t.Fatalf("told %x, has %d, carries %+v", got, s.gold, s.inventory)
	}
}

// A buy she cannot pay for, with no room for, or of a cell that holds nothing is refused
// with its reason, and nothing changes.
func TestABuyThatCannotBeIsRefusedWithItsReason(t *testing.T) {
	var full item.Inventory
	for cell := range uint16(item.Cells) {
		full = append(full, item.Item{ID: int64(cell) + 10, VNum: 10, Count: 1, Cell: cell})
	}
	cases := []struct {
		name     string
		gold     uint32
		carrying item.Inventory
		pos      uint8
		want     protocol.ShopError
	}{
		{"too dear", 31, nil, 0, protocol.ShopNotEnoughYang},
		{"no room", 100, full, 0, protocol.ShopBagFull},
		{"an empty cell", 100, nil, 3, protocol.ShopInvalid},
	}
	for _, c := range cases {
		s, kept, client, store := shopping(t, 100, c.gold, c.carrying)
		open(t, s, client, store)

		if err := s.onShop(bought(c.pos)); err != nil {
			t.Fatal(err)
		}

		expectRefused(t, client, c.want)
		if s.gold != c.gold || kept.gold != c.gold || len(s.inventory) != len(c.carrying) {
			t.Errorf("%s: has %d, carries %d", c.name, s.gold, len(s.inventory))
		}
	}
}

// With no shop open, a buy or sale is dropped; closing the shop closes it.
func TestNothingIsBoughtOrSoldWithNoShopOpen(t *testing.T) {
	s, kept, client, store := shopping(t, 100, 100, item.Inventory{{ID: 1, VNum: 27001, Count: 5, Cell: 0}})
	if err := s.onShop(bought(0)); err != nil {
		t.Fatal(err)
	}
	open(t, s, client, store)
	if err := s.onShop([]byte{protocol.ShopEnd}); err != nil {
		t.Fatal(err)
	}
	if err := s.onShop(sold(0, 1)); err != nil {
		t.Fatal(err)
	}

	expectNothing(t, client)
	if s.shop != 0 || kept.gold != 100 || s.inventory[0].Count != 5 {
		t.Fatalf("shop %d, kept %d, carries %+v", s.shop, kept.gold, s.inventory)
	}
}

// A sale pays what the client's tooltip says, for as many as she sells: part of a stack
// leaves the rest, all of it empties the cell, and a first weapon goes for nothing.
func TestASalePaysWhatTheTooltipSays(t *testing.T) {
	cases := []struct {
		name  string
		count uint8
		left  []byte
		paid  int32
	}{
		{"two of five potions", 2, itemSet(item.Item{ID: 1, VNum: 27001, Count: 3}), 12},
		{"all five", 5, itemCleared(item.Item{Cell: 0}), 30},
	}
	for _, c := range cases {
		s, kept, client, store := shopping(t, 100, 10, item.Inventory{{ID: 1, VNum: 27001, Count: 5, Cell: 0}})
		open(t, s, client, store)

		if err := s.onShop(sold(0, c.count)); err != nil {
			t.Fatal(err)
		}

		if got := read(t, client, itemSetSize+pointChangeSize); string(got) != string(c.left)+
			string(protocol.EncodePointChange(1000, protocol.PointGold, c.paid, 10+c.paid)) ||
			s.gold != uint32(10+c.paid) || kept.gold != s.gold { //nolint:gosec // small
			t.Errorf("%s: told %x, has %d", c.name, got, s.gold)
		}
	}

	s, kept, client, store := shopping(t, 100, 10, item.Inventory{{ID: 1, VNum: 10, Count: 1, Cell: 0}})
	open(t, s, client, store)
	if err := s.onShop(sold(0, 1)); err != nil {
		t.Fatal(err)
	}
	if got := read(t, client, itemSetSize+pointChangeSize); string(got) != string(itemCleared(item.Item{Cell: 0}))+
		string(protocol.EncodePointChange(1000, protocol.PointGold, 0, 10)) || len(s.inventory) != 0 || kept.gone[0] != 1 {
		t.Fatalf("told %x, carries %+v", got, s.inventory)
	}
}

// A worn item, more than the stack holds, or none at all cannot be sold.
func TestASaleThatCannotBeIsRefused(t *testing.T) {
	carrying := item.Inventory{{ID: 1, VNum: 27001, Count: 5, Cell: 0}, {ID: 2, VNum: 10, Count: 1, Worn: item.Weapon}}
	wornAlone := item.Inventory{{ID: 2, VNum: 10, Count: 1, Worn: item.Weapon}} // its cell left 0, as worn items' is
	for body, carried := range map[string]item.Inventory{
		string(sold(94, 1)): carrying, string(sold(0, 1)): wornAlone, string(sold(0, 6)): carrying,
		string(sold(0, 0)): carrying, string(sold(1, 1)): carrying,
	} {
		s, kept, client, store := shopping(t, 100, 10, carried)
		open(t, s, client, store)

		if err := s.onShop([]byte(body)); err != nil {
			t.Fatal(err)
		}

		expectRefused(t, client, protocol.ShopInvalid)
		if s.gold != 10 || kept.gold != 10 || len(s.inventory) != len(carried) {
			t.Errorf("%x: has %d, carries %+v", []byte(body), s.gold, s.inventory)
		}
	}
}

// A sale that would take her past what a character may hold is refused.
func TestASalePastWhatACharacterMayHoldIsRefused(t *testing.T) {
	s, kept, client, store := shopping(t, 100, maxGold-5, item.Inventory{{ID: 1, VNum: 27001, Count: 5, Cell: 0}})
	open(t, s, client, store)

	if err := s.onShop(sold(0, 1)); err != nil {
		t.Fatal(err)
	}

	expectRefused(t, client, protocol.ShopInvalid)
	if s.gold != maxGold-5 || kept.gold != maxGold-5 || s.inventory[0].Count != 5 {
		t.Fatalf("has %d, carries %+v", s.gold, s.inventory)
	}
}

// A buy or sale that cannot be kept changes nothing.
func TestAShopWriteThatFailsChangesNothing(t *testing.T) {
	s, kept, client, store := shopping(t, 100, 100, item.Inventory{{ID: 1, VNum: 27001, Count: 5, Cell: 0}})
	open(t, s, client, store)
	kept.fail = errors.New("the database is gone")

	for _, body := range [][]byte{bought(0), sold(0, 1)} {
		if err := s.onShop(body); err != nil {
			t.Fatal(err)
		}
	}

	expectNothing(t, client)
	if s.gold != 100 || len(s.inventory) != 1 || s.inventory[0].Count != 5 {
		t.Fatalf("has %d, carries %+v", s.gold, s.inventory)
	}
}
