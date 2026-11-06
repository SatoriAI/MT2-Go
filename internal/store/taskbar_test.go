package store

import (
	"context"
	"maps"
	"testing"

	"github.com/SatoriAI/MT2-Go/internal/account"
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

func (s Taskbar) mustHold(t *testing.T, cid uint32, want map[uint8]account.Shortcut) {
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

	if err := s.Set(ctx, cid, 0, account.Shortcut{Item: red}); err != nil {
		t.Fatal(err)
	}
	if err := s.Set(ctx, cid, 0, account.Shortcut{Item: blue}); err != nil {
		t.Fatal(err)
	}
	s.mustHold(t, cid, map[uint8]account.Shortcut{0: {Item: blue}})
	if err := s.Clear(ctx, cid, 0); err != nil {
		t.Fatal(err)
	}
	s.mustHold(t, cid, map[uint8]account.Shortcut{})
}

// Two slots swap what they hold, and a slot moved onto an empty one leaves its own empty.
func TestTaskbarSlotsSwap(t *testing.T) {
	s, cid, red, blue := hendrasTaskbar(t)
	ctx := context.Background()
	if err := s.Set(ctx, cid, 0, account.Shortcut{Item: red}); err != nil {
		t.Fatal(err)
	}
	if err := s.Set(ctx, cid, 1, account.Shortcut{Item: blue}); err != nil {
		t.Fatal(err)
	}

	if err := s.Swap(ctx, cid, 0, 1); err != nil {
		t.Fatal(err)
	}
	s.mustHold(t, cid, map[uint8]account.Shortcut{0: {Item: blue}, 1: {Item: red}})
	if err := s.Swap(ctx, cid, 1, 7); err != nil {
		t.Fatal(err)
	}
	s.mustHold(t, cid, map[uint8]account.Shortcut{0: {Item: blue}, 7: {Item: red}})
}

// A slot goes with its item: the last potion drunk takes the slot holding it.
func TestATaskbarSlotGoesWithItsItem(t *testing.T) {
	s, cid, red, blue := hendrasTaskbar(t)
	ctx := context.Background()
	if err := s.Set(ctx, cid, 0, account.Shortcut{Item: red}); err != nil {
		t.Fatal(err)
	}
	if err := s.Set(ctx, cid, 1, account.Shortcut{Item: blue}); err != nil {
		t.Fatal(err)
	}

	if err := Items(s).Remove(ctx, red); err != nil {
		t.Fatal(err)
	}

	s.mustHold(t, cid, map[uint8]account.Shortcut{1: {Item: blue}})
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

	if err := s.Set(ctx, cid, 0, account.Shortcut{Item: red}); err != nil {
		t.Fatal(err)
	}
	if err := s.Clear(ctx, kira, 0); err != nil {
		t.Fatal(err)
	}

	s.mustHold(t, cid, map[uint8]account.Shortcut{0: {Item: red}})
	s.mustHold(t, kira, map[uint8]account.Shortcut{})
}

// learnDragonSwirl puts Hendra on Weaponry with Dragon Swirl learnt.
func learnDragonSwirl(t *testing.T, s Taskbar, cid uint32) {
	t.Helper()
	skills := Skills(s)
	if err := skills.Choose(context.Background(), cid, "weaponry"); err != nil {
		t.Fatal(err)
	}
	if _, err := skills.Raise(context.Background(), cid, "dragon_swirl"); err != nil {
		t.Fatal(err)
	}
}

// A slot holds a skill learnt, and swaps like any other.
func TestATaskbarSlotHoldsASkillLearnt(t *testing.T) {
	s, cid, red, _ := hendrasTaskbar(t)
	ctx := context.Background()
	learnDragonSwirl(t, s, cid)
	if err := s.Set(ctx, cid, 4, account.Shortcut{Skill: "dragon_swirl"}); err != nil {
		t.Fatal(err)
	}
	if err := s.Set(ctx, cid, 0, account.Shortcut{Item: red}); err != nil {
		t.Fatal(err)
	}

	if err := s.Swap(ctx, cid, 4, 5); err != nil {
		t.Fatal(err)
	}

	s.mustHold(t, cid, map[uint8]account.Shortcut{0: {Item: red}, 5: {Skill: "dragon_swirl"}})
}

// A skill's slot goes when choosing a skill group forgets the skill; an item's stays.
func TestATaskbarSlotOfASkillGoesWhenTheSkillIsForgotten(t *testing.T) {
	s, cid, red, _ := hendrasTaskbar(t)
	ctx := context.Background()
	learnDragonSwirl(t, s, cid)
	for slot, held := range map[uint8]account.Shortcut{0: {Item: red}, 4: {Skill: "dragon_swirl"}} {
		if err := s.Set(ctx, cid, slot, held); err != nil {
			t.Fatal(err)
		}
	}

	if err := Skills(s).Choose(ctx, cid, "black_magic"); err != nil {
		t.Fatal(err)
	}

	s.mustHold(t, cid, map[uint8]account.Shortcut{0: {Item: red}})
}

// The database refuses a slot of a skill not learnt, of nothing, or of both an item and a
// skill.
func TestTheDatabaseRefusesASlotThatIsNoShortcut(t *testing.T) {
	s, cid, red, _ := hendrasTaskbar(t)
	ctx := context.Background()
	if err := Skills(s).Choose(ctx, cid, "weaponry"); err != nil {
		t.Fatal(err)
	}
	for name, held := range map[string]account.Shortcut{
		"a skill not learnt": {Skill: "dragon_swirl"},
		"nothing":            {},
		"both":               {Item: red, Skill: "dragon_swirl"},
	} {
		if err := s.Set(ctx, cid, 4, held); err == nil {
			t.Errorf("%s: kept", name)
		}
	}
}
