package store

import (
	"context"
	"maps"
	"testing"

	"github.com/SatoriAI/MT2-Go/internal/item"
)

// hendrasTaskbar is Hendra with a stack of red potions at cell 0 and blue ones at cell 1,
// and her taskbar.
func hendrasTaskbar(t *testing.T) (Taskbar, uint32, int64, int64) {
	t.Helper()
	items, cid := hendrasItems(t)
	red, err := items.Add(context.Background(), cid, item.Item{VNum: 27001, Count: 5, Cell: 0})
	if err != nil {
		t.Fatal(err)
	}
	blue, err := items.Add(context.Background(), cid, item.Item{VNum: 27004, Count: 5, Cell: 1})
	if err != nil {
		t.Fatal(err)
	}
	return Taskbar(items), cid, red, blue
}

func (s Taskbar) mustHold(t *testing.T, cid uint32, want map[uint8]int64) {
	t.Helper()
	got, err := s.Slots(context.Background(), cid)
	if err != nil || !maps.Equal(got, want) {
		t.Fatalf("holds %v, %v; want %v", got, err, want)
	}
}

// A slot is filled, filled again in place of what it held, and emptied.
func TestATaskbarSlotIsSetReplacedAndCleared(t *testing.T) {
	s, cid, red, blue := hendrasTaskbar(t)
	ctx := context.Background()

	if err := s.Set(ctx, cid, 0, red); err != nil {
		t.Fatal(err)
	}
	if err := s.Set(ctx, cid, 0, blue); err != nil {
		t.Fatal(err)
	}
	s.mustHold(t, cid, map[uint8]int64{0: blue})
	if err := s.Clear(ctx, cid, 0); err != nil {
		t.Fatal(err)
	}
	s.mustHold(t, cid, map[uint8]int64{})
}

// Two slots swap what they hold, and a slot moved onto an empty one leaves its own empty.
func TestTaskbarSlotsSwap(t *testing.T) {
	s, cid, red, blue := hendrasTaskbar(t)
	ctx := context.Background()
	if err := s.Set(ctx, cid, 0, red); err != nil {
		t.Fatal(err)
	}
	if err := s.Set(ctx, cid, 1, blue); err != nil {
		t.Fatal(err)
	}

	if err := s.Swap(ctx, cid, 0, 1); err != nil {
		t.Fatal(err)
	}
	s.mustHold(t, cid, map[uint8]int64{0: blue, 1: red})
	if err := s.Swap(ctx, cid, 1, 7); err != nil {
		t.Fatal(err)
	}
	s.mustHold(t, cid, map[uint8]int64{0: blue, 7: red})
}

// A slot goes with its item: the last potion drunk takes the slot holding it.
func TestATaskbarSlotGoesWithItsItem(t *testing.T) {
	s, cid, red, blue := hendrasTaskbar(t)
	ctx := context.Background()
	if err := s.Set(ctx, cid, 0, red); err != nil {
		t.Fatal(err)
	}
	if err := s.Set(ctx, cid, 1, blue); err != nil {
		t.Fatal(err)
	}

	if err := Items(s).Remove(ctx, red); err != nil {
		t.Fatal(err)
	}

	s.mustHold(t, cid, map[uint8]int64{1: blue})
}

// Each character's slots are their own.
func TestTaskbarSlotsAreEachCharactersOwn(t *testing.T) {
	s, cid, red, _ := hendrasTaskbar(t)
	ctx := context.Background()
	var kira uint32
	if err := s.Pool.QueryRow(ctx, `INSERT INTO characters (account_id, slot, name, class, sex, strength, vitality,
		dexterity, intelligence, x, y) SELECT account_id, 1, 'Kira', 'sura', 'female', 5, 3, 3, 5, 0, 0
		FROM characters WHERE id = $1 RETURNING id`, cid).Scan(&kira); err != nil {
		t.Fatal(err)
	}

	if err := s.Set(ctx, cid, 0, red); err != nil {
		t.Fatal(err)
	}
	if err := s.Clear(ctx, kira, 0); err != nil {
		t.Fatal(err)
	}

	s.mustHold(t, cid, map[uint8]int64{0: red})
	s.mustHold(t, kira, map[uint8]int64{})
}
