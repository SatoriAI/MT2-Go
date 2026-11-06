package game

import (
	"encoding/binary"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/SatoriAI/MT2-Go/internal/item"
	"github.com/SatoriAI/MT2-Go/internal/network"
	"github.com/SatoriAI/MT2-Go/internal/protocol"
)

// dropping is Hendra with 20 yang, carrying five red potions at cell 0 and wearing a
// sword, with Kira beside her; their clients have read each other.
func dropping(t *testing.T) (*session, *keptItems, net.Conn, net.Conn) {
	t.Helper()
	s := worldWith(t)
	kept := &keptItems{gold: 20, count: map[int64]uint8{}}
	s.items = kept
	s.now = func() time.Time { return noon }
	client, hendra := aPlayer(t, s, 1000, 7, "Hendra", 100)
	kira, _ := aPlayer(t, s, 1001, 8, "Kira", 200)
	read(t, client, addSize+infoSize)
	read(t, kira, addSize+infoSize)
	sess := &session{svc: s, conn: hendra.out.(*network.Conn), phase: protocol.PhaseGame, rand: newRand(),
		playing: &character{vid: 1000, Character: *hendra.char}, gold: 20, inventory: item.Inventory{
			{ID: 1, VNum: 27001, Count: 5, Cell: 0}, {ID: 2, VNum: 10, Count: 1, Worn: item.Weapon}}}
	return sess, kept, client, kira
}

func dropped(window uint8, cell uint16, gold uint32, count uint8) []byte {
	body := binary.LittleEndian.AppendUint16([]byte{window}, cell)
	return append(binary.LittleEndian.AppendUint32(body, gold), count)
}

// theOneLying is the one item on the ground.
func theOneLying(t *testing.T, s *session) *groundItem {
	t.Helper()
	if len(s.svc.presence.ground) != 1 {
		t.Fatalf("%d lie on the ground", len(s.svc.presence.ground))
	}
	for _, it := range s.svc.presence.ground {
		return it
	}
	return nil
}

// Her whole stack dropped is forgotten and its cell emptied; it lies beside her, anyone's,
// shown to Kira as it is to her, and its taskbar slot is emptied.
func TestAStackDroppedLiesBesideHerForAnyone(t *testing.T) {
	s, kept, client, kira := dropping(t)
	onTheTaskbar(t, s, 1)

	if err := s.onItemDrop(dropped(1, 0, 0, 5)); err != nil {
		t.Fatal(err)
	}

	if got := read(t, client, itemSetSize+2); string(got) != string(itemCleared(item.Item{Cell: 0}))+
		string(protocol.EncodeQuickslotDel(0)) {
		t.Fatalf("told %x", got)
	}
	it := theOneLying(t, s)
	want := protocol.EncodeGroundAdd(it.vid, it.x, it.y, 27001)
	for _, c := range []net.Conn{client, kira} {
		if got := read(t, c, groundAddSize); string(got) != string(want) {
			t.Fatalf("shown %x, want %x", got, want)
		}
	}
	if it.count != 5 || it.owner.id != 0 || !near(it.x, it.y, homeX+100, homeY, 2*dropSpread) || len(kept.gone) != 1 ||
		len(s.inventory) != 1 {
		t.Fatalf("lies %+v, forgot %v, carries %+v", it, kept.gone, s.inventory)
	}
}

// Part of a stack dropped leaves the rest in her bag.
func TestPartOfAStackDroppedLeavesTheRest(t *testing.T) {
	s, kept, client, _ := dropping(t)

	if err := s.onItemDrop(dropped(1, 0, 0, 2)); err != nil {
		t.Fatal(err)
	}

	if got := read(t, client, itemSetSize); string(got) != string(itemSet(item.Item{ID: 1, VNum: 27001, Count: 3})) ||
		kept.count[1] != 3 || theOneLying(t, s).count != 2 {
		t.Fatalf("told %x, kept %v", got, kept.count)
	}
}

// A worn item, an empty cell, none or more than the stack, and a dead player drop nothing.
func TestADropThatCannotBeIsRefused(t *testing.T) {
	for name, body := range map[string][]byte{
		"the sword worn": dropped(1, 94, 0, 1),
		"an empty cell":  dropped(1, 3, 0, 1),
		"none":           dropped(1, 0, 0, 0),
		"too many":       dropped(1, 0, 0, 6),
		"while dead":     dropped(1, 0, 0, 1),
	} {
		s, kept, client, _ := dropping(t)
		if name == "while dead" {
			s.svc.presence.all[1000].char.HP = 0
		}

		if err := s.onItemDrop(body); err != nil {
			t.Fatal(err)
		}

		expectNothing(t, client)
		if len(s.svc.presence.ground) != 0 || len(kept.gone) != 0 || len(kept.count) != 0 || len(s.inventory) != 2 {
			t.Errorf("%s: dropped", name)
		}
	}
}

// A drop that cannot be kept changes nothing and lays nothing down.
func TestADropThatCannotBeKeptLaysNothing(t *testing.T) {
	s, kept, client, _ := dropping(t)
	kept.fail = errors.New("the database is gone")

	for _, body := range [][]byte{dropped(1, 0, 0, 5), dropped(0, 0, 5, 0)} {
		if err := s.onItemDrop(body); err != nil {
			t.Fatal(err)
		}
	}

	expectNothing(t, client)
	if len(s.svc.presence.ground) != 0 || s.gold != 20 || s.inventory[0].Count != 5 {
		t.Fatalf("lie %d, has %d", len(s.svc.presence.ground), s.gold)
	}
}

// Yang dropped is spent and lies beside her as yang, anyone's; more than she has, or
// none, is not dropped.
func TestYangDroppedIsSpentAndLiesAsYang(t *testing.T) {
	s, kept, client, _ := dropping(t)
	for _, body := range [][]byte{dropped(0, 0, 21, 0), dropped(0, 0, 0, 0)} {
		if err := s.onItemDrop(body); err != nil {
			t.Fatal(err)
		}
	}
	expectNothing(t, client)

	if err := s.onItemDrop(dropped(0, 0, 5, 0)); err != nil {
		t.Fatal(err)
	}

	if got := read(t, client, pointChangeSize); string(got) != string(protocol.EncodePointChange(1000, protocol.PointGold, -5, 15)) {
		t.Fatalf("told %x", got)
	}
	it := theOneLying(t, s)
	if got := read(t, client, groundAddSize); string(got) != string(protocol.EncodeGroundAdd(it.vid, it.x, it.y, yangVNum)) ||
		it.gold != 5 || it.owner.id != 0 || s.gold != 15 || kept.gold != 15 {
		t.Fatalf("shown %x, lies %+v, has %d", got, it, s.gold)
	}
}

// What a player drops the world loop clears away a minute after it was dropped.
func TestADroppedItemIsClearedAwayAfterAMinute(t *testing.T) {
	s, _, client, _ := dropping(t)
	if err := s.onItemDrop(dropped(1, 0, 0, 5)); err != nil {
		t.Fatal(err)
	}
	read(t, client, itemSetSize+groundAddSize)
	vid := theOneLying(t, s).vid

	s.svc.tick(noon.Add(lieFor - time.Millisecond))
	if s.svc.presence.ground[vid] == nil {
		t.Fatal("cleared before its minute")
	}
	s.svc.tick(noon.Add(lieFor))

	if got := read(t, client, groundDelSize); string(got) != string(protocol.EncodeGroundDel(vid)) || s.svc.presence.ground[vid] != nil {
		t.Fatalf("got %x", got)
	}
}

// Kira, beside her, picks up what Hendra dropped at once.
func TestAnotherPlayerPicksUpWhatWasDroppedAtOnce(t *testing.T) {
	s, _, client, _ := dropping(t)
	if err := s.onItemDrop(dropped(1, 0, 0, 5)); err != nil {
		t.Fatal(err)
	}
	read(t, client, itemSetSize+groundAddSize)

	if _, got := s.svc.presence.pickUp(1001, theOneLying(t, s).vid, func(*groundItem) bool { return true }); got != picked {
		t.Fatalf("Kira's pick-up: %v", got)
	}
}

// A player's drop laid just after the tick's time, and a monster's drop in that tick,
// each go when due: the monster's at its minute, though it reached the queue after hers.
func TestDropsOfPlayersAndMonstersAreEachClearedWhenDue(t *testing.T) {
	s, _, client, _ := dropping(t)
	s.svc.now = func() time.Time { return noon.Add(2 * time.Millisecond) }
	if err := s.onItemDrop(dropped(1, 0, 0, 5)); err != nil {
		t.Fatal(err)
	}
	read(t, client, itemSetSize+groundAddSize)
	hers := theOneLying(t, s).vid
	s.svc.tick(noon) // takes her drop

	monsters := &groundItem{vid: 9000, x: homeX, y: homeY, vnum: 27004, count: 1, dropped: noon}
	s.svc.presence.drop(s.svc.presence.all[1000], []*groundItem{monsters})
	s.svc.vanishing = inOrder(s.svc.vanishing, timed{9000, noon.Add(lieFor)})
	read(t, client, groundAddSize)

	s.svc.keepGround(noon.Add(lieFor))
	if s.svc.presence.ground[9000] != nil || s.svc.presence.ground[hers] == nil {
		t.Fatalf("the monster's lies: %v; hers: %v", s.svc.presence.ground[9000] != nil, s.svc.presence.ground[hers] != nil)
	}
	s.svc.keepGround(noon.Add(lieFor + 2*time.Millisecond))
	if s.svc.presence.ground[hers] != nil {
		t.Fatal("hers outlived its minute")
	}
}
