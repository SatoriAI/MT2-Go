package item

import (
	"errors"
	"testing"

	"github.com/SatoriAI/MT2-Go/internal/account"
)

var (
	sword, _  = Lookup(10)    // two cells tall
	glaive, _ = Lookup(3000)  // three
	potion, _ = Lookup(27001) // one, and stacks
)

// Every kind's values are what the client can hold: a size of one to three cells, and a
// stack of one or the game's most, under its u8 count.
func TestEveryKindIsOneTheClientCanHold(t *testing.T) {
	for _, vnum := range VNums() {
		p, _ := Lookup(vnum)
		if p.VNum != vnum || p.Name == "" || p.Size < 1 || p.Size > 3 || (p.Stack != 1 && p.Stack != MaxStack) {
			t.Errorf("kind %d: %+v", vnum, p)
		}
	}
}

// An item fits where its column is free from its top down, all on one page: a sword in
// the bottom row has no cell below it, and one in the first page's last row but one would
// run onto the second page.
func TestAnItemFitsWhereItsCellsAreFreeOnOnePage(t *testing.T) {
	cases := []struct {
		name string
		p    Proto
		cell uint16
		want bool
	}{
		{"a sword at the top", sword, 0, true},
		{"a sword in a page's last row", sword, 40, false},
		{"a sword in the row above it", sword, 35, true},
		{"a glaive two rows above a page's end", glaive, 30, true},
		{"a glaive one row lower", glaive, 35, false},
		{"a potion in the very last cell", potion, 89, true},
		{"a sword at the second page's end", sword, 85, false},
	}
	for _, c := range cases {
		if got := (Inventory{}).Fits(c.p, c.cell, 0); got != c.want {
			t.Errorf("%s: fits %v, want %v", c.name, got, c.want)
		}
	}
}

// Another item is in the way where it stands, from its top down, and only in its own
// column: a sword at 0 takes 0 and 5.
func TestAnotherItemIsInTheWayWhereItStands(t *testing.T) {
	inv := Inventory{{ID: 1, VNum: 10, Count: 1, Cell: 0}}
	cases := []struct {
		name string
		p    Proto
		cell uint16
		want bool
	}{
		{"on its top", potion, 0, false},
		{"on its second cell", potion, 5, false},
		{"under it", potion, 10, true},
		{"beside it", potion, 1, true},
		{"a glaive from its second cell down", glaive, 5, false},
	}
	for _, c := range cases {
		if got := inv.Fits(c.p, c.cell, 0); got != c.want {
			t.Errorf("%s: fits %v, want %v", c.name, got, c.want)
		}
	}
}

// An item being moved is not in its own way, so it can move one row down over itself.
func TestAnItemIsNotInItsOwnWay(t *testing.T) {
	inv := Inventory{{ID: 1, VNum: 10, Count: 1, Cell: 0}}

	if !inv.Fits(sword, 5, 1) {
		t.Fatal("the sword cannot move one row down over itself")
	}
}

// The first free cell for an item is the first, in order, where all of it fits.
func TestTheFirstFreeCellIsTheFirstWhereItFits(t *testing.T) {
	inv := Inventory{{ID: 1, VNum: 10, Count: 1, Cell: 0}, {ID: 2, VNum: 27001, Count: 1, Cell: 11}}

	if cell, ok := inv.Free(glaive); !ok || cell != 2 { // 1 would run into the potion at 11
		t.Fatalf("got cell %d (%v), want 2", cell, ok)
	}
}

// A full inventory has no cell for another item.
func TestAFullInventoryHasNoFreeCell(t *testing.T) {
	var inv Inventory
	for cell := range uint16(Cells) {
		inv = append(inv, Item{ID: int64(cell) + 1, VNum: 27001, Count: 1, Cell: cell})
	}

	if cell, ok := inv.Free(potion); ok {
		t.Fatalf("found cell %d in a full inventory", cell)
	}
}

// A give names a kind the game has, and no more than one item of it holds: one sword, up to
// 200 potions.
func TestAGiveIsOfAKindAndAsManyAsOneItemHolds(t *testing.T) {
	cases := []struct {
		vnum  uint32
		count uint8
		ok    bool
	}{
		{10, 1, true}, {10, 2, false}, {27001, 200, true}, {27001, 201, false}, {27001, 0, false}, {11, 1, false},
	}
	for _, c := range cases {
		o, err := GiveOrder(c.vnum, int(c.count))
		if (err == nil) != c.ok || (c.ok && o != (Order{Kind: Give, VNum: c.vnum, Count: c.count})) {
			t.Errorf("%d of %d: got %+v, %v", c.count, c.vnum, o, err)
		}
	}
}

// A worn item stands in no cell: its old cell is free, and another item fits there.
func TestAWornItemTakesNoCell(t *testing.T) {
	inv := Inventory{{ID: 1, VNum: 10, Count: 1, Cell: 0, Worn: Weapon}}

	if _, ok := inv.At(0); ok || !inv.Fits(potion, 0, 0) {
		t.Fatal("the worn sword still stands in cell 0")
	}
	if worn, ok := inv.WornIn(Weapon); !ok || worn.ID != 1 {
		t.Fatalf("worn as a weapon: %+v (%v)", worn, ok)
	}
	if cell, ok := inv.Free(sword); !ok || cell != 0 {
		t.Fatalf("the first free cell is %d (%v)", cell, ok)
	}
}

// Who may wear what: the sword is for warriors, ninjas and suras; a potion is not worn at
// all.
func TestOnlyTheClassesAKindIsForMayWearIt(t *testing.T) {
	cases := []struct {
		p     Proto
		class account.Class
		want  error
	}{
		{sword, account.Sura, nil},
		{sword, account.Shaman, ErrWrongClass},
		{potion, account.Sura, ErrNotWearable},
	}
	for _, c := range cases {
		if got := c.p.CanWear(c.class); !errors.Is(got, c.want) || (got == nil) != (c.want == nil) {
			t.Errorf("%s wearing %q: got %v, want %v", c.class, c.p.Name, got, c.want)
		}
	}
}

// Every kind that is worn says who may wear it and adds to a blow or sets against one.
func TestEveryWornKindHasItsClassesAndStats(t *testing.T) {
	for _, vnum := range VNums() {
		p, _ := Lookup(vnum)
		weapon := p.Wear == Weapon && p.AttackMin > 0 && p.AttackMin <= p.AttackMax && p.Defence == 0
		armour := p.Wear == Body && p.Defence > 0 && p.AttackMax == 0
		if p.Wear != "" && (len(p.Classes) == 0 || p.Stack != 1 || (!weapon && !armour)) {
			t.Errorf("kind %d: %+v", vnum, p)
		}
	}
}

// The potions are the kinds drunk: the red ones heal, the blue one restores mana, as the
// client's item_proto says; nothing else restores anything.
func TestOnlyThePotionsRestoreHPOrSP(t *testing.T) {
	want := map[uint32][2]uint16{27001: {300, 0}, 27002: {800, 0}, 27004: {0, 100}}
	for _, vnum := range VNums() {
		p, _ := Lookup(vnum)
		if got := [2]uint16{p.HP, p.SP}; got != want[vnum] {
			t.Errorf("kind %d restores %v, want %v", vnum, got, want[vnum])
		}
	}
}

// Potions picked up join a stack of their kind with room for all of them; when none has,
// or for a kind that does not stack, they take the first free cell.
func TestAnItemPickedUpJoinsItsStackOrTakesAFreeCell(t *testing.T) {
	inv := Inventory{{ID: 1, VNum: 27001, Count: 199, Cell: 0}, {ID: 2, VNum: 27001, Count: 5, Cell: 1},
		{ID: 3, VNum: 10, Count: 1, Cell: 2}}

	if got, ok := inv.RoomFor(potion, 3); !ok || got != (Item{ID: 2, VNum: 27001, Count: 8, Cell: 1}) {
		t.Errorf("3 potions: %+v (%v), want the stack of 5 at 8", got, ok)
	}
	if got, ok := inv.RoomFor(potion, 196); !ok || got.ID != 0 || got.Cell != 3 || got.Count != 196 {
		t.Errorf("196 potions: %+v (%v), want a new stack at 3", got, ok)
	}
	if got, ok := inv.RoomFor(sword, 1); !ok || got.ID != 0 || got.Cell != 3 {
		t.Errorf("a sword: %+v (%v), want a new one at 3", got, ok)
	}
}

// Every kind costs what Dawid chose or the client's item_proto says, and a shop pays the
// client's sell price, never more than it asks, so buying and selling back makes nothing.
func TestEveryKindHasItsShopPrices(t *testing.T) {
	want := map[uint32][2]uint32{
		10: {100, 0}, 1000: {100, 0}, 2000: {100, 0}, 3000: {100, 0}, 7000: {100, 0},
		11200: {3000, 600}, 11400: {3000, 600}, 11600: {3000, 600}, 11800: {3000, 600},
		27001: {32, 6}, 27002: {96, 18}, 27004: {96, 24},
	}
	for _, vnum := range VNums() {
		p, _ := Lookup(vnum)
		if got := [2]uint32{p.Price, p.SellPrice}; got != want[vnum] || p.SellPrice > p.Price {
			t.Errorf("kind %d: %v, want %v", vnum, got, want[vnum])
		}
	}
}
