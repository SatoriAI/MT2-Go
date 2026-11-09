package store

import (
	"context"
	"testing"

	"github.com/SatoriAI/MT2-Go/internal/account"
)

// A skill raised is learnt at 1, then rises; choosing a group, even the same one, forgets
// what was learnt.
func TestASkillRaisedIsKeptAndChoosingAGroupForgetsIt(t *testing.T) {
	items, cid := hendrasItems(t)
	s := Skills(items)
	ctx := context.Background()
	if err := s.Choose(ctx, cid, "weaponry"); err != nil {
		t.Fatal(err)
	}

	for want := uint8(1); want <= 2; want++ {
		if level, err := s.Raise(ctx, cid, "dragon_swirl"); err != nil || level != want {
			t.Fatalf("raised to %d, %v; want %d", level, err, want)
		}
	}
	if learnt, err := s.Learnt(ctx, cid); err != nil || len(learnt) != 1 || learnt["dragon_swirl"] != 2 {
		t.Fatalf("learnt %v, %v", learnt, err)
	}
	if err := s.Choose(ctx, cid, "black_magic"); err != nil {
		t.Fatal(err)
	}

	var group account.SkillGroup
	if err := s.Pool.QueryRow(ctx, `SELECT skill_group FROM characters WHERE id = $1`, cid).Scan(&group); err != nil || group != "black_magic" {
		t.Fatalf("group %q, %v", group, err)
	}
	if learnt, _ := s.Learnt(ctx, cid); len(learnt) != 0 {
		t.Fatalf("still learnt %v", learnt)
	}
}

// The database refuses a skill the game does not have, a level past 20, and a group of
// another class.
func TestTheDatabaseRefusesWhatSkillsCannotBe(t *testing.T) {
	items, cid := hendrasItems(t)
	s := Skills(items)
	ctx := context.Background()

	if _, err := s.Raise(ctx, cid, "fireball"); err == nil {
		t.Error("learnt a skill the game does not have")
	}
	if _, err := s.Pool.Exec(ctx, `INSERT INTO skills (character_id, skill, level) VALUES ($1, 'dragon_swirl', 20)`, cid); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Raise(ctx, cid, "dragon_swirl"); err == nil {
		t.Error("raised past 20")
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
		if _, err := s.Raise(ctx, cid, sk); err != nil {
			t.Fatalf("%s: %v", sk, err)
		}
	}

	if learnt, err := s.Learnt(ctx, cid); err != nil || len(learnt) != len(names) {
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
		if _, err := s.Raise(ctx, cid, sk); err != nil {
			t.Fatalf("%s: %v", sk, err)
		}
	}

	if learnt, err := s.Learnt(ctx, cid); err != nil || len(learnt) != len(names) {
		t.Fatalf("learnt %v, %v", learnt, err)
	}
}
