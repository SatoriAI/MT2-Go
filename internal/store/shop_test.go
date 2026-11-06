package store

import (
	"context"
	"testing"

	"github.com/SatoriAI/MT2-Go/internal/item"
)

// hendraWithYang is Hendra with gold yang and a stack of five red potions at cell 4.
func hendraWithYang(t *testing.T, gold uint32) (Items, uint32, int64) {
	t.Helper()
	s, cid := hendrasItems(t)
	if _, err := s.AddGold(context.Background(), cid, gold); err != nil {
		t.Fatal(err)
	}
	id, err := s.Add(context.Background(), cid, item.Item{VNum: 27001, Count: 5, Cell: 4})
	if err != nil {
		t.Fatal(err)
	}
	return s, cid, id
}

func (s Items) mustOwn(t *testing.T) (item.Inventory, uint32) {
	t.Helper()
	inv, _, err := s.Owned(context.Background(), "Hendra")
	if err != nil {
		t.Fatal(err)
	}
	var gold uint32
	if err := s.Pool.QueryRow(context.Background(), `SELECT gold FROM characters WHERE name = 'Hendra'`).Scan(&gold); err != nil {
		t.Fatal(err)
	}
	return inv, gold
}

// A buy takes the yang and keeps the item together: a new item in its cell, or a stack at
// its new count.
func TestABuyIsKeptWhole(t *testing.T) {
	s, cid, stack := hendraWithYang(t, 200)
	ctx := context.Background()

	sword, gold, err := s.Buy(ctx, cid, 100, item.Item{VNum: 10, Count: 1, Cell: 0})
	if err != nil || gold != 100 || sword.ID == 0 {
		t.Fatalf("bought %+v, left %d, %v", sword, gold, err)
	}
	if _, gold, err = s.Buy(ctx, cid, 64, item.Item{ID: stack, VNum: 27001, Count: 7, Cell: 4}); err != nil || gold != 36 {
		t.Fatalf("left %d, %v", gold, err)
	}

	inv, kept := s.mustOwn(t)
	if s, ok := inv.ByID(stack); kept != 36 || len(inv) != 2 || !ok || s.Count != 7 {
		t.Fatalf("owns %+v with %d yang", inv, kept)
	}
}

// A buy the character cannot pay for is refused, and nothing changes.
func TestABuyTheYangCannotCoverChangesNothing(t *testing.T) {
	s, cid, _ := hendraWithYang(t, 50)

	if _, _, err := s.Buy(context.Background(), cid, 100, item.Item{VNum: 10, Count: 1, Cell: 0}); err == nil {
		t.Fatal("bought")
	}

	if inv, gold := s.mustOwn(t); gold != 50 || len(inv) != 1 {
		t.Fatalf("owns %+v with %d yang", inv, gold)
	}
}

// A buy whose item cannot be kept, here into a cell another stands in, takes no yang.
func TestABuyWhoseItemCannotBeKeptTakesNoYang(t *testing.T) {
	s, cid, _ := hendraWithYang(t, 200)

	if _, _, err := s.Buy(context.Background(), cid, 100, item.Item{VNum: 10, Count: 1, Cell: 4}); err == nil {
		t.Fatal("bought into a taken cell")
	}

	if inv, gold := s.mustOwn(t); gold != 200 || len(inv) != 1 {
		t.Fatalf("owns %+v with %d yang", inv, gold)
	}
}

// A sale pays the yang and keeps what is left together: a stack at its lower count, or the
// item forgotten with none left.
func TestASaleIsKeptWhole(t *testing.T) {
	s, cid, stack := hendraWithYang(t, 10)
	ctx := context.Background()

	if gold, err := s.Sell(ctx, cid, 12, item.Item{ID: stack, VNum: 27001, Count: 3, Cell: 4}); err != nil || gold != 22 {
		t.Fatalf("has %d, %v", gold, err)
	}
	if inv, _ := s.mustOwn(t); len(inv) != 1 || inv[0].Count != 3 {
		t.Fatalf("owns %+v", inv)
	}
	if gold, err := s.Sell(ctx, cid, 18, item.Item{ID: stack, VNum: 27001, Count: 0, Cell: 4}); err != nil || gold != 40 {
		t.Fatalf("has %d, %v", gold, err)
	}
	if inv, gold := s.mustOwn(t); len(inv) != 0 || gold != 40 {
		t.Fatalf("owns %+v with %d yang", inv, gold)
	}
}
