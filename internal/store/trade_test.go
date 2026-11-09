package store

import (
	"context"
	"errors"
	"testing"

	"github.com/SatoriAI/MT2-Go/internal/item"
)

// traders are Hendra and Kira, on one account: Hendra with 1000 yang, a sword at cell 0
// on her taskbar and claws at cell 5; Kira with 50 yang and a potion at cell 0.
func traders(t *testing.T) (s Items, hendra, kira uint32, sword, claws, potion int64) {
	t.Helper()
	s, hendra = hendrasItems(t)
	ctx := context.Background()
	if err := s.Pool.QueryRow(ctx, `INSERT INTO characters (account_id, slot, name, class, sex,
		strength, vitality, dexterity, intelligence, x, y)
		SELECT account_id, 1, 'Kira', 'warrior', 'female', 6, 4, 3, 3, 0, 0 FROM characters WHERE id = $1
		RETURNING id`, hendra).Scan(&kira); err != nil {
		t.Fatal(err)
	}
	var err error
	for _, give := range []struct {
		to   uint32
		it   item.Item
		into *int64
	}{{hendra, item.Item{VNum: 14, Count: 1, Cell: 0}, &sword}, {hendra, item.Item{VNum: 30028, Count: 2, Cell: 5}, &claws},
		{kira, item.Item{VNum: 27001, Count: 3, Cell: 0}, &potion}} {
		if *give.into, err = s.Add(ctx, give.to, give.it); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.AddGold(ctx, hendra, 1000); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddGold(ctx, kira, 50); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Pool.Exec(ctx, `INSERT INTO taskbar (character_id, slot, item_id) VALUES ($1, 0, $2)`, hendra, sword); err != nil {
		t.Fatal(err)
	}
	return s, hendra, kira, sword, claws, potion
}

func ownerAndCell(t *testing.T, s Items, id int64) (uint32, uint16) {
	t.Helper()
	var owner uint32
	var cell uint16
	if err := s.Pool.QueryRow(context.Background(), `SELECT character_id, cell FROM items WHERE id = $1`, id).Scan(&owner, &cell); err != nil {
		t.Fatal(err)
	}
	return owner, cell
}

// A trade moves each side's items to the other at the cells planned, even into a cell the
// other side gives up, moves the yang, drops the taskbar slot of an item given, and is
// logged.
func TestATradeIsKeptWhole(t *testing.T) {
	s, hendra, kira, sword, claws, potion := traders(t)
	ctx := context.Background()

	gold, err := s.Trade(ctx, item.Trade{
		{Character: hendra, Items: []item.Item{{ID: sword, VNum: 14, Count: 1, Cell: 0}, {ID: claws, VNum: 30028, Count: 2, Cell: 1}}, Gold: 300},
		{Character: kira, Items: []item.Item{{ID: potion, VNum: 27001, Count: 3, Cell: 0}}, Gold: 20},
	})

	if err != nil || gold != [2]uint32{720, 330} {
		t.Fatalf("yang %v, %v", gold, err)
	}
	for id, want := range map[int64][2]uint32{sword: {kira, 0}, claws: {kira, 1}, potion: {hendra, 0}} {
		if owner, cell := ownerAndCell(t, s, id); owner != want[0] || uint32(cell) != want[1] {
			t.Errorf("item %d with %d at %d", id, owner, cell)
		}
	}
	var slots, logged, items int
	if err := s.Pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM taskbar), (SELECT count(*) FROM trades
		WHERE first_id = $1 AND second_id = $2 AND first_gold = 300 AND second_gold = 20),
		(SELECT count(*) FROM traded_items WHERE from_id = $1)`, hendra, kira).Scan(&slots, &logged, &items); err != nil {
		t.Fatal(err)
	}
	if slots != 0 || logged != 1 || items != 2 {
		t.Fatalf("%d slots left, %d trades and %d of Hendra's items logged", slots, logged, items)
	}
}

// A trade of an item the side no longer owns or wears, or of more yang than it has,
// changes nothing.
func TestATradeThatCannotBeKeptChangesNothing(t *testing.T) {
	s, hendra, kira, sword, _, potion := traders(t)
	ctx := context.Background()

	for name, trade := range map[string]item.Trade{
		"not owned": {{Character: hendra, Items: []item.Item{{ID: potion, Cell: 9}}}, {Character: kira}},
		"too much":  {{Character: hendra, Items: []item.Item{{ID: sword, Cell: 9}}}, {Character: kira, Gold: 51}},
		"worn":      {{Character: hendra}, {Character: kira, Items: []item.Item{{ID: potion, Cell: 9}}}},
	} {
		if name == "worn" {
			if err := s.Place(ctx, item.Item{ID: potion, Worn: item.Body}); err != nil {
				t.Fatal(err)
			}
		}
		_, err := s.Trade(ctx, trade)
		if name != "too much" && !errors.Is(err, ErrNotOwned) || err == nil {
			t.Errorf("%s: %v", name, err)
		}
	}

	if owner, cell := ownerAndCell(t, s, sword); owner != hendra || cell != 0 {
		t.Fatalf("the sword is with %d at %d", owner, cell)
	}
	var trades int
	if err := s.Pool.QueryRow(ctx, `SELECT count(*) FROM trades`).Scan(&trades); err != nil || trades != 0 {
		t.Fatalf("%d trades logged, %v", trades, err)
	}
}
