package game

import (
	"encoding/binary"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/SatoriAI/MT2-Go/internal/event"
	"github.com/SatoriAI/MT2-Go/internal/item"
	"github.com/SatoriAI/MT2-Go/internal/mob"
	"github.com/SatoriAI/MT2-Go/internal/network"
	"github.com/SatoriAI/MT2-Go/internal/protocol"
	"github.com/SatoriAI/MT2-Go/internal/world"
)

// aDogLeaving is a world with one wild dog at home that leaves yang alone, as given, and
// Hendra (character 7) beside it, with the dog read.
func aDogLeaving(t *testing.T, yang mob.Yang) (*Service, *monster, net.Conn) {
	t.Helper()
	s := worldWith(t, world.Spawn{VNum: 101, X: homeX, Y: homeY, Count: 1})
	m := s.monsters[0]
	rolled := *m.proto
	rolled.Drops, rolled.Yang = nil, yang
	m.proto = &rolled
	beside, _ := aPlayer(t, s, 1000, 7, "Hendra", 100)
	read(t, beside, addSize)
	return s, m, beside
}

// A dog certain to leave 7 yang leaves it near its body for its owner, shown as the
// client's yang with her name over it.
func TestADeathLeavesYangShownAsYang(t *testing.T) {
	s, m, beside := aDogLeaving(t, mob.Yang{Chance: 10000, Min: 7, Max: 7})

	s.dropFrom(m, death{owner: owner{7, "Hendra"}}, noon)

	if len(s.presence.ground) != 1 {
		t.Fatalf("%d fell", len(s.presence.ground))
	}
	for vid, it := range s.presence.ground {
		if it.gold != 7 || it.vnum != 0 || !near(it.x, it.y, m.x, m.y, 2*dropSpread) {
			t.Fatalf("fell %+v", it)
		}
		want := string(protocol.EncodeGroundAdd(vid, it.x, it.y, 1)) + string(protocol.EncodeItemOwner(vid, "Hendra"))
		if got := read(t, beside, groundAddSize+ownerSize); string(got) != want {
			t.Fatalf("got  %x\nwant %x", got, want)
		}
	}
}

// The amount is rolled from the least to the most, each of them, and a monster with no
// chance of yang leaves none.
func TestYangIsRolledWithinItsRange(t *testing.T) {
	s, m, _ := aDogLeaving(t, mob.Yang{Chance: 10000, Min: 5, Max: 7})
	seen := map[uint32]bool{}
	for range 200 {
		s.dropFrom(m, death{owner: owner{7, "Hendra"}}, noon)
	}
	for _, it := range s.presence.ground {
		seen[it.gold] = true
	}
	if len(seen) != 3 || !seen[5] || !seen[6] || !seen[7] {
		t.Fatalf("rolled %v", seen)
	}

	none, dog, _ := aDogLeaving(t, mob.Yang{})
	none.dropFrom(dog, death{owner: owner{7, "Hendra"}}, noon)
	if len(none.presence.ground) != 0 {
		t.Fatalf("%d fell", len(none.presence.ground))
	}
}

// A running event multiplies the yang that falls, rounded down.
func TestAnEventMultipliesTheYangThatFalls(t *testing.T) {
	s, m, _ := aDogLeaving(t, mob.Yang{Chance: 10000, Min: 7, Max: 7})
	s.presence.event = &event.Event{ExpPercent: 100, YangPercent: 150}

	s.dropFrom(m, death{owner: owner{7, "Hendra"}}, noon)

	if len(s.presence.ground) != 1 {
		t.Fatalf("%d fell", len(s.presence.ground))
	}
	for _, it := range s.presence.ground {
		if it.gold != 10 {
			t.Fatalf("fell %d, want 10", it.gold)
		}
	}
}

// pickingUpYang is Hendra, with 100 yang and a bag full of swords, beside 25 yang of hers.
func pickingUpYang(t *testing.T) (*session, *keptItems, net.Conn) {
	t.Helper()
	s := worldWith(t)
	kept := &keptItems{gold: 100}
	s.items = kept
	client, hendra := aPlayer(t, s, 1000, 7, "Hendra", 100)
	s.presence.drop(hendra, []*groundItem{{vid: 5000, x: homeX, y: homeY, gold: 25, owner: owner{7, "Hendra"}, dropped: noon}})
	read(t, client, groundAddSize+ownerSize)
	s.now = func() time.Time { return noon.Add(time.Second) }
	var full item.Inventory
	for cell := range uint16(item.Cells) {
		full = append(full, item.Item{ID: int64(cell) + 10, VNum: 10, Count: 1, Cell: cell})
	}
	sess := &session{svc: s, conn: hendra.out.(*network.Conn), phase: protocol.PhaseGame,
		playing: &character{vid: 1000, Character: *hendra.char}, inventory: full, gold: 100}
	return sess, kept, client
}

var theYang = binary.LittleEndian.AppendUint32(nil, 5000)

// Yang picked up is kept and added to her yang, even with her bag full: she is told her
// new balance, and the client says how much she got.
func TestYangPickedUpIsAddedToHerYang(t *testing.T) {
	s, kept, client := pickingUpYang(t)

	if err := s.onItemPickup(theYang); err != nil {
		t.Fatal(err)
	}

	read(t, client, groundDelSize)
	if got, want := read(t, client, pointChangeSize), protocol.EncodePointChange(1000, protocol.PointGold, 25, 125); string(got) != string(want) {
		t.Fatalf("got  %x\nwant %x", got, want)
	}
	if s.gold != 125 || kept.gold != 125 || len(s.inventory) != item.Cells {
		t.Fatalf("has %d, kept %d, carries %d items", s.gold, kept.gold, len(s.inventory))
	}
}

// Yang that cannot be kept, as when it would pass what a character may hold, goes back on
// the ground, and her yang is as it was.
func TestYangThatCannotBeKeptGoesBack(t *testing.T) {
	s, kept, client := pickingUpYang(t)
	kept.fail = errors.New("more than she may hold")

	if err := s.onItemPickup(theYang); err != nil {
		t.Fatal(err)
	}

	read(t, client, groundDelSize)
	want := string(protocol.EncodeGroundAdd(5000, homeX, homeY, 1)) + string(protocol.EncodeItemOwner(5000, "Hendra"))
	if got := read(t, client, len(want)); string(got) != want || s.gold != 100 || s.svc.presence.ground[5000] == nil {
		t.Fatalf("got %x, has %d", got, s.gold)
	}
}

// Yang that would take her past what a character may hold is left lying, untouched and
// unwritten, and she is told nothing.
func TestYangPastWhatACharacterMayHoldIsLeftLying(t *testing.T) {
	s, kept, client := pickingUpYang(t)
	s.gold = maxGold - 24

	if err := s.onItemPickup(theYang); err != nil {
		t.Fatal(err)
	}

	expectNothing(t, client)
	if s.gold != maxGold-24 || kept.gold != 100 || s.svc.presence.ground[5000] == nil {
		t.Fatalf("has %d, kept %d", s.gold, kept.gold)
	}
}
