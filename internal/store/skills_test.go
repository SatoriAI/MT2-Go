package store

import (
	"context"
	"errors"
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

// booked is Hendra with Dragon Swirl at M1, 20, and two books of it at cell 0.
func booked(t *testing.T) (Skills, uint32, item.Item) {
	t.Helper()
	items, cid := hendrasItems(t)
	ctx := context.Background()
	id, err := items.Add(ctx, cid, item.Item{VNum: 50462, Count: 2, Cell: 0})
	if err != nil {
		t.Fatal(err)
	}
	s := Skills(items)
	if err := s.Raise(ctx, cid, "dragon_swirl", 20); err != nil {
		t.Fatal(err)
	}
	return s, cid, item.Item{ID: id, VNum: 50462, Count: 2, Cell: 0}
}

// A read is kept whole: the skill at its new level and the time it may next be read, and
// the book used, a stack at its count left, or forgotten with none left.
func TestAReadIsKeptWhole(t *testing.T) {
	s, cid, book := booked(t)
	ctx := context.Background()
	next := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)

	book.Count = 1
	if err := s.Train(ctx, cid, "dragon_swirl", 20, 21, next, book); err != nil {
		t.Fatal(err)
	}
	book.Count = 0
	if err := s.Train(ctx, cid, "dragon_swirl", 21, 21, next.Add(time.Hour), book); err != nil {
		t.Fatal(err)
	}

	learnt, reads, err := s.Learnt(ctx, cid)
	if err != nil || learnt["dragon_swirl"] != 21 || !reads["dragon_swirl"].Equal(next.Add(time.Hour)) {
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

	if err := s.Train(ctx, cid, "dragon_swirl", 21, 22, next, item.Item{ID: book.ID, Count: 1}); !errors.Is(err, ErrNotTrained) {
		t.Errorf("at another level: %v", err)
	}
	if err := s.Train(ctx, cid, "dragon_swirl", 20, 21, next, item.Item{ID: book.ID + 1000, Count: 1}); !errors.Is(err, ErrNotTrained) {
		t.Errorf("with another's book: %v", err)
	}

	learnt, reads, _ := s.Learnt(ctx, cid)
	inv, _, _ := Items(s).Owned(ctx, "Hendra")
	if learnt["dragon_swirl"] != 20 || len(reads) != 0 || len(inv) != 1 || inv[0].Count != 2 {
		t.Fatalf("learnt %v, read %v, holds %+v", learnt, reads, inv)
	}
}
