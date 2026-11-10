package store

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/SatoriAI/MT2-Go/internal/account"
	"github.com/SatoriAI/MT2-Go/internal/item"
)

// A skill raised is learnt at the level a point gives it, then kept at each next one;
// choosing a group, even the same one, forgets what was learnt.
func TestASkillRaisedIsKeptAndChoosingAGroupForgetsIt(t *testing.T) {
	items, cid := hendrasItems(t)
	s := Skills(items)
	ctx := context.Background()
	if err := s.Choose(ctx, cid, "weaponry"); err != nil {
		t.Fatal(err)
	}

	for level := uint8(1); level <= 2; level++ {
		if err := s.Raise(ctx, cid, "dragon_swirl", level); err != nil {
			t.Fatalf("raised to %d: %v", level, err)
		}
	}
	if learnt, reads, err := s.Learnt(ctx, cid); err != nil || len(learnt) != 1 || learnt["dragon_swirl"] != 2 || len(reads) != 0 {
		t.Fatalf("learnt %v, read %v, %v", learnt, reads, err)
	}
	if err := s.Choose(ctx, cid, "black_magic"); err != nil {
		t.Fatal(err)
	}

	var group account.SkillGroup
	if err := s.Pool.QueryRow(ctx, `SELECT skill_group FROM characters WHERE id = $1`, cid).Scan(&group); err != nil || group != "black_magic" {
		t.Fatalf("group %q, %v", group, err)
	}
	if learnt, _, _ := s.Learnt(ctx, cid); len(learnt) != 0 {
		t.Fatalf("still learnt %v", learnt)
	}
}

// The database refuses a skill the game does not have, a level past 39, G10, and a group
// of another class.
func TestTheDatabaseRefusesWhatSkillsCannotBe(t *testing.T) {
	items, cid := hendrasItems(t)
	s := Skills(items)
	ctx := context.Background()

	if err := s.Raise(ctx, cid, "fireball", 1); err == nil {
		t.Error("learnt a skill the game does not have")
	}
	if err := s.Raise(ctx, cid, "dragon_swirl", 39); err != nil {
		t.Fatal(err)
	}
	if err := s.Raise(ctx, cid, "dragon_swirl", 40); err == nil {
		t.Error("raised past 39")
	}
	if err := s.Choose(ctx, cid, "healing"); err == nil {
		t.Error("a sura chose the shaman's healing")
	}
}

// Every skill the game has is kept, the rest of Weaponry with the first two.
func TestEveryWeaponrySkillIsKept(t *testing.T) {
	items, cid := hendrasItems(t)
	s := Skills(items)
	ctx := context.Background()
	names := []account.Skill{"finger_strike", "dragon_swirl", "enchanted_blade", "enchanted_armour"}

	for _, sk := range names {
		if err := s.Raise(ctx, cid, sk, 1); err != nil {
			t.Fatalf("%s: %v", sk, err)
		}
	}

	if learnt, _, err := s.Learnt(ctx, cid); err != nil || len(learnt) != len(names) {
		t.Fatalf("learnt %v, %v", learnt, err)
	}
}

// Archery's skills are kept too, the last group's.
func TestEveryArcherySkillIsKept(t *testing.T) {
	items, cid := hendrasItems(t)
	s := Skills(items)
	ctx := context.Background()
	names := []account.Skill{"repetitive_shot", "arrow_shower", "fire_arrow", "feather_walk", "poison_arrow"}

	for _, sk := range names {
		if err := s.Raise(ctx, cid, sk, 1); err != nil {
			t.Fatalf("%s: %v", sk, err)
		}
	}

	if learnt, _, err := s.Learnt(ctx, cid); err != nil || len(learnt) != len(names) {
		t.Fatalf("learnt %v, %v", learnt, err)
	}
}

// booked is Hendra with Dragon Swirl at M1, 20, and its book at cell 0.
func booked(t *testing.T) (Skills, uint32, item.Item) {
	t.Helper()
	items, cid := hendrasItems(t)
	ctx := context.Background()
	id, err := items.Add(ctx, cid, item.Item{VNum: 50462, Count: 1, Cell: 0})
	if err != nil {
		t.Fatal(err)
	}
	s := Skills(items)
	if err := s.Raise(ctx, cid, "dragon_swirl", 20); err != nil {
		t.Fatal(err)
	}
	return s, cid, item.Item{ID: id, VNum: 50462, Count: 1, Cell: 0}
}

// A read is kept whole: the skill at its new level and the time it may next be read, and
// the book forgotten.
func TestAReadIsKeptWhole(t *testing.T) {
	s, cid, book := booked(t)
	ctx := context.Background()
	next := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)

	if err := s.Train(ctx, cid, account.Read{Skill: "dragon_swirl", From: 20, To: 21, Next: next, Used: book.ID}); err != nil {
		t.Fatal(err)
	}

	learnt, reads, err := s.Learnt(ctx, cid)
	if err != nil || learnt["dragon_swirl"] != 21 || !reads["dragon_swirl"].Equal(next) {
		t.Fatalf("learnt %v, read %v, %v", learnt, reads, err)
	}
	inv, _, _ := Items(s).Owned(ctx, "Hendra")
	if len(inv) != 0 {
		t.Fatalf("still holds %+v", inv)
	}
}

// A read of a skill no longer at the level it was read at, or with a book the character
// does not hold, changes nothing.
func TestAReadThatCannotBeKeptChangesNothing(t *testing.T) {
	s, cid, book := booked(t)
	ctx := context.Background()
	next := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)

	if err := s.Train(ctx, cid, account.Read{Skill: "dragon_swirl", From: 21, To: 22, Next: next, Used: book.ID}); !errors.Is(err, ErrNotTrained) {
		t.Errorf("at another level: %v", err)
	}
	if err := s.Train(ctx, cid, account.Read{Skill: "dragon_swirl", From: 20, To: 21, Next: next, Used: book.ID + 1000}); !errors.Is(err, ErrNotTrained) {
		t.Errorf("with another's book: %v", err)
	}

	learnt, reads, _ := s.Learnt(ctx, cid)
	inv, _, _ := Items(s).Owned(ctx, "Hendra")
	if learnt["dragon_swirl"] != 20 || len(reads) != 0 || len(inv) != 1 {
		t.Fatalf("learnt %v, read %v, holds %+v", learnt, reads, inv)
	}
}

// aidsOf is what the character holds.
func aidsOf(t *testing.T, s Skills, cid uint32) account.Aids {
	t.Helper()
	var aids account.Aids
	if err := s.Pool.QueryRow(context.Background(), `SELECT aids FROM characters WHERE id = $1`, cid).Scan(&aids); err != nil {
		t.Fatal(err)
	}
	return aids
}

// An aid an item leaves is kept with the item used up, and read with the character; a
// second of it is refused, and that item kept.
func TestAnAidIsKeptWithItsItemUsedUp(t *testing.T) {
	s, cid, _ := booked(t)
	ctx := context.Background()
	var scrolls [2]int64
	for i := range scrolls {
		var err error
		if scrolls[i], err = Items(s).Add(ctx, cid, item.Item{VNum: item.ExorcismScroll, Count: 1, Cell: uint16(1 + i)}); err != nil {
			t.Fatal(err)
		}
	}

	if err := s.TakeAid(ctx, cid, account.NoReadWait, item.Item{ID: scrolls[0]}); err != nil {
		t.Fatal(err)
	}
	if err := s.TakeAid(ctx, cid, account.NoReadWait, item.Item{ID: scrolls[1]}); !errors.Is(err, account.ErrAidHeld) {
		t.Fatalf("a second: %v", err)
	}

	inv, _, _ := Items(s).Owned(ctx, "Hendra")
	if aids := aidsOf(t, s, cid); !slices.Equal(aids, account.Aids{account.NoReadWait}) || len(inv) != 2 || held(inv, scrolls[0]) {
		t.Fatalf("holds %v and %+v", aids, inv)
	}
	var accountID int64
	if err := s.Pool.QueryRow(ctx, `SELECT account_id FROM characters WHERE id = $1`, cid).Scan(&accountID); err != nil {
		t.Fatal(err)
	}
	if chars, err := Accounts(s).Characters(ctx, accountID); err != nil || !chars[0].Aids.Has(account.NoReadWait) {
		t.Fatalf("read with %+v (%v)", chars, err)
	}
}

// A read spends the aids it names with it; one naming an aid not held changes nothing.
func TestAReadSpendsItsAids(t *testing.T) {
	s, cid, book := booked(t)
	ctx := context.Background()
	advice, err := Items(s).Add(ctx, cid, item.Item{VNum: item.HermitsAdvice, Count: 1, Cell: 1})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.TakeAid(ctx, cid, account.ReadBonus, item.Item{ID: advice}); err != nil {
		t.Fatal(err)
	}
	read := account.Read{Skill: "dragon_swirl", From: 20, To: 21, Next: time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC), Used: book.ID}

	notHeld := read
	notHeld.Spent = account.Aids{account.ReadBonus, account.NoReadWait}
	if err := s.Train(ctx, cid, notHeld); !errors.Is(err, ErrNotTrained) {
		t.Fatalf("spending one not held: %v", err)
	}
	if learnt, _, _ := s.Learnt(ctx, cid); learnt["dragon_swirl"] != 20 || !slices.Equal(aidsOf(t, s, cid), account.Aids{account.ReadBonus}) {
		t.Fatalf("after a refused read: learnt %v, holds %v", learnt, aidsOf(t, s, cid))
	}

	read.Spent = account.Aids{account.ReadBonus}
	if err := s.Train(ctx, cid, read); err != nil {
		t.Fatal(err)
	}
	if aids := aidsOf(t, s, cid); len(aids) != 0 {
		t.Fatalf("still holds %v", aids)
	}
}

func held(inv item.Inventory, id int64) bool { _, ok := inv.ByID(id); return ok }
