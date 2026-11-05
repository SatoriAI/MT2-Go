package game

import (
	"errors"
	"net"
	"testing"
	"time"

	"github.com/SatoriAI/MT2-Go/internal/item"
	"github.com/SatoriAI/MT2-Go/internal/protocol"
)

const effectSize = 6

// thirsty is Hendra, as dressing has her, at hp hit points and no mana, with a stack of
// red potions at cell 10 and of blue ones at cell 11, of reds and blues each.
func thirsty(t *testing.T, hp uint32, reds, blues uint8) (*session, *keptItems, net.Conn, net.Conn) {
	t.Helper()
	s, kept, client, viewer := dressing(t)
	s.playing.HP, s.playing.SP = hp, 0
	s.inventory = item.Inventory{{ID: 10, VNum: 27001, Count: reds, Cell: 10}, {ID: 11, VNum: 27004, Count: blues, Cell: 11}}
	return s, kept, client, viewer
}

func most(s *session) (uint32, uint32) {
	points, _ := s.playing.Points()
	return points.MaxHP, points.MaxSP
}

// A red potion heals 300: the stack is kept one fewer and shown so, she is told her hit
// points, and she and Kira see its effect.
func TestARedPotionHealsAndIsUsedUp(t *testing.T) {
	s, kept, client, viewer := thirsty(t, 1, 50, 1)

	if err := s.onItemUse(doubleClicked(10)); err != nil {
		t.Fatal(err)
	}

	want := string(itemSet(item.Item{ID: 10, VNum: 27001, Count: 49, Cell: 10})) +
		string(protocol.EncodePointChange(1000, protocol.PointHP, 300, 301)) +
		string(protocol.EncodeSpecialEffect(protocol.EffectRedPotion, 1000))
	if got := read(t, client, itemSetSize+pointChangeSize+effectSize); string(got) != want {
		t.Fatalf("got  %x\nwant %x", got, want)
	}
	if got := read(t, viewer, effectSize); string(got) != string(protocol.EncodeSpecialEffect(protocol.EffectRedPotion, 1000)) {
		t.Fatalf("Kira saw %x", got)
	}
	if s.playing.HP != 301 || kept.count[10] != 49 || s.inventory[0].Count != 49 {
		t.Fatalf("at %d HP, kept %d, carrying %d", s.playing.HP, kept.count[10], s.inventory[0].Count)
	}
}

// A potion heals no further than her maximum.
func TestAPotionHealsNoFurtherThanTheMaximum(t *testing.T) {
	s, _, client, _ := thirsty(t, 0, 50, 1)
	maxHP, _ := most(s)
	s.playing.HP = maxHP - 100

	if err := s.onItemUse(doubleClicked(10)); err != nil {
		t.Fatal(err)
	}

	read(t, client, itemSetSize)
	want := protocol.EncodePointChange(1000, protocol.PointHP, 100, int32(maxHP)) //nolint:gosec // small
	if got := read(t, client, pointChangeSize); string(got) != string(want) || s.playing.HP != maxHP {
		t.Fatalf("told %x, at %d HP", got, s.playing.HP)
	}
}

// A blue potion restores mana, with its own effect.
func TestABluePotionRestoresMana(t *testing.T) {
	s, kept, client, _ := thirsty(t, 1, 50, 2)
	_, maxSP := most(s)
	gained := min(100, maxSP)

	if err := s.onItemUse(doubleClicked(11)); err != nil {
		t.Fatal(err)
	}

	want := string(itemSet(item.Item{ID: 11, VNum: 27004, Count: 1, Cell: 11})) +
		string(protocol.EncodePointChange(1000, protocol.PointSP, int32(gained), int32(gained))) +
		string(protocol.EncodeSpecialEffect(protocol.EffectBluePotion, 1000))
	if got := read(t, client, itemSetSize+pointChangeSize+effectSize); string(got) != want {
		t.Fatalf("got  %x\nwant %x", got, want)
	}
	if s.playing.SP != gained || s.playing.HP != 1 || kept.count[11] != 1 {
		t.Fatalf("at %d SP and %d HP, kept %d", s.playing.SP, s.playing.HP, kept.count[11])
	}
}

// The last potion of a stack is forgotten, and its cell emptied.
func TestTheLastPotionEmptiesItsCell(t *testing.T) {
	s, kept, client, _ := thirsty(t, 1, 1, 1)

	if err := s.onItemUse(doubleClicked(10)); err != nil {
		t.Fatal(err)
	}

	if got := read(t, client, itemSetSize); string(got) != string(itemCleared(item.Item{Cell: 10})) {
		t.Fatalf("told %x", got)
	}
	if _, held := s.inventory.At(10); held || len(kept.gone) != 1 || kept.gone[0] != 10 || s.playing.HP != 301 {
		t.Fatalf("carrying %+v, forgot %v, at %d HP", s.inventory, kept.gone, s.playing.HP)
	}
}

// A potion that would do nothing is not used up: a red one at full hit points, a blue one
// at full mana, any to the dead.
func TestAPotionThatWouldDoNothingIsNotUsedUp(t *testing.T) {
	cases := []struct {
		name string
		hp   func(maxHP uint32) uint32
		full bool // mana
		cell uint16
	}{
		{"a red one at full hit points", func(maxHP uint32) uint32 { return maxHP }, false, 10},
		{"a blue one at full mana", func(uint32) uint32 { return 1 }, true, 11},
		{"a red one to the dead", func(uint32) uint32 { return 0 }, false, 10},
	}
	for _, c := range cases {
		s, kept, client, _ := thirsty(t, 0, 50, 50)
		maxHP, maxSP := most(s)
		s.playing.HP = c.hp(maxHP)
		if c.full {
			s.playing.SP = maxSP
		}

		if err := s.onItemUse(doubleClicked(c.cell)); err != nil {
			t.Fatal(err)
		}

		expectNothing(t, client)
		if len(kept.count) != 0 || s.inventory[0].Count != 50 || s.inventory[1].Count != 50 {
			t.Errorf("%s: kept %v, carrying %+v", c.name, kept.count, s.inventory)
		}
	}
}

// A potion that cannot be kept used up heals nothing, and is shown as it was.
func TestAPotionNotKeptUsedUpHealsNothing(t *testing.T) {
	s, kept, client, _ := thirsty(t, 1, 50, 1)
	kept.fail = errors.New("the database is gone")

	if err := s.onItemUse(doubleClicked(10)); err != nil {
		t.Fatal(err)
	}

	if got := read(t, client, itemSetSize); string(got) != string(itemSet(item.Item{ID: 10, VNum: 27001, Count: 50, Cell: 10})) {
		t.Fatalf("told %x", got)
	}
	expectNothing(t, client)
	if s.playing.HP != 1 || s.inventory[0].Count != 50 {
		t.Fatalf("at %d HP, carrying %d", s.playing.HP, s.inventory[0].Count)
	}
}

// A potion works in a fight, where regaining hit points waits for it to end.
func TestAPotionWorksInAFight(t *testing.T) {
	s, _, _, _ := thirsty(t, 1, 50, 1)
	s.playing.LastFought = time.Now()

	if err := s.onItemUse(doubleClicked(10)); err != nil {
		t.Fatal(err)
	}

	if s.playing.HP != 301 {
		t.Errorf("at %d HP", s.playing.HP)
	}
}

// One who died after the potion was used up, before it healed, stays dead: a potion does
// not raise the dead.
func TestAPotionDoesNotRaiseOneWhoDiedMeanwhile(t *testing.T) {
	s, _, client, _ := thirsty(t, 0, 50, 1)
	potion, _ := item.Lookup(27001)

	s.svc.presence.drink(1000, potion)

	expectNothing(t, client)
	if s.playing.HP != 0 {
		t.Fatalf("raised to %d HP", s.playing.HP)
	}
}

// One healed to full meanwhile, by regaining over time while the potion was being used
// up, is shown no effect: it did nothing.
func TestAPotionThatFoundHerFullShowsNoEffect(t *testing.T) {
	s, _, client, viewer := thirsty(t, 0, 50, 1)
	s.playing.HP, _ = most(s)
	potion, _ := item.Lookup(27001)

	s.svc.presence.drink(1000, potion)

	expectNothing(t, client)
	expectNothing(t, viewer)
}
