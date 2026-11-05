package store

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/SatoriAI/MT2-Go/internal/account"
	"github.com/SatoriAI/MT2-Go/internal/item"
)

// hendrasItems is a fresh database with Hendra, a sura, owning nothing.
func hendrasItems(t *testing.T) (Items, uint32) {
	t.Helper()
	accounts, id := accountInShinsoo(t)
	var cid uint32
	if err := accounts.Pool.QueryRow(context.Background(), `INSERT INTO characters (account_id, slot, name, class, sex,
		strength, vitality, dexterity, intelligence, x, y) VALUES ($1, 0, 'Hendra', 'sura', 'female', 5, 3, 3, 5, 0, 0)
		RETURNING id`, id).Scan(&cid); err != nil {
		t.Fatal(err)
	}
	return Items(accounts), cid
}

func (s Items) mustOrder(t *testing.T, o item.Order) int64 {
	t.Helper()
	id, err := s.Order(context.Background(), "hendra", o)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func (s Items) mustApply(t *testing.T, cid uint32) (item.Inventory, []item.Order) {
	t.Helper()
	inv, done, err := s.ApplyOrders(context.Background(), cid)
	if err != nil {
		t.Fatal(err)
	}
	return inv, done
}

// Given items go, oldest order first, into the first cell each fits: the sword at the top,
// two cells tall, so the potion goes beside it. The orders are then done.
func TestAGiveGoesIntoTheFirstCellItFits(t *testing.T) {
	s, cid := hendrasItems(t)
	s.mustOrder(t, item.Order{Kind: item.Give, VNum: 10, Count: 1})
	s.mustOrder(t, item.Order{Kind: item.Give, VNum: 27001, Count: 50})

	applied, done := s.mustApply(t, cid)

	inv, orders, err := s.Owned(context.Background(), "Hendra")
	if err != nil || !slices.Equal(applied, inv) || len(done) != 2 || len(orders) != 0 || len(inv) != 2 ||
		inv[0].VNum != 10 || inv[0].Cell != 0 || inv[1].VNum != 27001 || inv[1].Count != 50 || inv[1].Cell != 1 {
		t.Fatalf("did %d, owns %+v, waiting %+v (%v)", len(done), inv, orders, err)
	}
}

// A give that fits nowhere waits for room, and the orders after it are still carried out.
func TestAGiveThatFitsNowhereWaits(t *testing.T) {
	s, cid := hendrasItems(t)
	for cell := range item.Cells - 1 { // all but the last cell, at the second page's foot
		if _, err := s.Pool.Exec(context.Background(),
			`INSERT INTO items (character_id, vnum, count, cell) VALUES ($1, 27001, 1, $2)`, cid, cell); err != nil {
			t.Fatal(err)
		}
	}
	sword := s.mustOrder(t, item.Order{Kind: item.Give, VNum: 10, Count: 1})
	s.mustOrder(t, item.Order{Kind: item.Give, VNum: 1000, Count: 1})

	_, done := s.mustApply(t, cid)

	_, orders, _ := s.Owned(context.Background(), "Hendra")
	if len(done) != 1 || done[0].VNum != 1000 || len(orders) != 1 || orders[0].ID != sword {
		t.Fatalf("did %+v, waiting %+v; want the dagger done and the sword waiting", done, orders)
	}
}

// A take removes its item; one whose item is gone by then is done with nothing to take.
func TestATakeRemovesItsItem(t *testing.T) {
	s, cid := hendrasItems(t)
	s.mustOrder(t, item.Order{Kind: item.Give, VNum: 10, Count: 1})
	inv, _ := s.mustApply(t, cid)
	s.mustOrder(t, item.Order{Kind: item.Take, ItemID: inv[0].ID})
	s.mustOrder(t, item.Order{Kind: item.Take, ItemID: inv[0].ID})

	applied, done := s.mustApply(t, cid)

	inv, orders, _ := s.Owned(context.Background(), "Hendra")
	if len(done) != 2 || len(inv) != 0 || len(applied) != 0 || len(orders) != 0 {
		t.Fatalf("did %d, owns %+v (%+v), waiting %+v", len(done), inv, applied, orders)
	}
}

// A take frees its cells for the orders after it: in a full inventory, a potion taken
// makes room for a dagger given next, at the same login.
func TestATakeMakesRoomForTheGivesAfterIt(t *testing.T) {
	s, cid := hendrasItems(t)
	for cell := range item.Cells {
		if _, err := s.Pool.Exec(context.Background(),
			`INSERT INTO items (character_id, vnum, count, cell) VALUES ($1, 27001, 1, $2)`, cid, cell); err != nil {
			t.Fatal(err)
		}
	}
	full, _, _ := s.Owned(context.Background(), "Hendra")
	s.mustOrder(t, item.Order{Kind: item.Take, ItemID: full[7].ID})
	s.mustOrder(t, item.Order{Kind: item.Give, VNum: 1000, Count: 1})

	inv, done := s.mustApply(t, cid)

	if dagger, ok := inv.At(7); len(done) != 2 || !ok || dagger.VNum != 1000 {
		t.Fatalf("did %d; cell 7 holds %+v (%v)", len(done), dagger, ok)
	}
}

// An order names a character that exists, in any case, and a take an item they own.
func TestAnOrderIsRefusedForNobodyOrAnItemNotTheirs(t *testing.T) {
	s, _ := hendrasItems(t)

	if _, err := s.Order(context.Background(), "Kira", item.Order{Kind: item.Give, VNum: 10, Count: 1}); !errors.Is(err,
		account.ErrNotFound) {
		t.Errorf("an order for nobody: %v", err)
	}
	if _, err := s.Order(context.Background(), "HENDRA", item.Order{Kind: item.Take, ItemID: 99}); !errors.Is(err,
		ErrNotOwned) {
		t.Errorf("a take of an item not hers: %v", err)
	}
}

// The database keeps what the client can show: a count it can carry, a cell in the
// inventory, and one item to a cell.
func TestTheDatabaseRefusesWhatTheInventoryCannotHold(t *testing.T) {
	s, cid := hendrasItems(t)
	for _, row := range [][2]int{{0, 0}, {201, 0}, {1, 90}, {1, -1}} {
		if _, err := s.Pool.Exec(context.Background(), `INSERT INTO items (character_id, vnum, count, cell)
			VALUES ($1, 10, $2, $3)`, cid, row[0], row[1]); err == nil {
			t.Errorf("stored %d at cell %d", row[0], row[1])
		}
	}
	s.mustOrder(t, item.Order{Kind: item.Give, VNum: 10, Count: 1})
	s.mustApply(t, cid)
	if _, err := s.Pool.Exec(context.Background(),
		`INSERT INTO items (character_id, vnum, count, cell) VALUES ($1, 1000, 1, 0)`, cid); err == nil {
		t.Error("stored a second item in cell 0")
	}
}

// Kinds owned or ordered that the game no longer has are found, so the server can refuse
// to start rather than lose them.
func TestKindsTheGameNoLongerHasAreFound(t *testing.T) {
	s, cid := hendrasItems(t)
	s.mustOrder(t, item.Order{Kind: item.Give, VNum: 10, Count: 1})
	s.mustOrder(t, item.Order{Kind: item.Give, VNum: 1000, Count: 1})
	s.mustApply(t, cid)
	s.mustOrder(t, item.Order{Kind: item.Give, VNum: 27001, Count: 5})

	unknown, err := s.UnknownKinds(context.Background(), []uint32{1000})

	if err != nil || !slices.Equal(unknown, []uint32{10, 27001}) {
		t.Fatalf("got %v %v, want the sword owned and the potion ordered", unknown, err)
	}
}

// A moved item is kept where it was moved to.
func TestAMovedItemIsKept(t *testing.T) {
	s, cid := hendrasItems(t)
	s.mustOrder(t, item.Order{Kind: item.Give, VNum: 10, Count: 1})
	inv, _ := s.mustApply(t, cid)

	if err := s.Move(context.Background(), inv[0].ID, 17); err != nil {
		t.Fatal(err)
	}

	if inv, _, _ := s.Owned(context.Background(), "Hendra"); inv[0].Cell != 17 {
		t.Fatalf("kept at %d", inv[0].Cell)
	}
}
