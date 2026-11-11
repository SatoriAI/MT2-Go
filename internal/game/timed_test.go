package game

import (
	"context"
	"encoding/binary"
	"net"
	"slices"
	"testing"
	"time"

	"github.com/SatoriAI/MT2-Go/internal/account"
	"github.com/SatoriAI/MT2-Go/internal/item"
	"github.com/SatoriAI/MT2-Go/internal/protocol"
)

// gloved is Hendra with the thief's gloves at cell 20, worn for wornFor, and a second
// pair at cell 21, at noon, her client read up to now.
func gloved(t *testing.T, wornFor time.Duration) (*session, *keptItems, net.Conn) {
	t.Helper()
	s, kept, client, _ := dressing(t)
	s.playing.ID = 7
	s.svc.now = func() time.Time { return noon }
	s.inventory = append(s.inventory, item.Item{ID: 5, VNum: item.ThiefGloves, Count: 1, Cell: 20, WornFor: wornFor},
		item.Item{ID: 6, VNum: item.ThiefGloves, Count: 1, Cell: 21})
	return s, kept, client
}

// putOnGloves double-clicks the gloves at cell 20 and reads what it changed for her.
func putOnGloves(t *testing.T, s *session, client net.Conn) [][]byte {
	t.Helper()
	if err := s.onItemUse(context.Background(), doubleClicked(20)); err != nil {
		t.Fatal(err)
	}
	return packets(t, drain(t, client))
}

func at(s *session, d time.Duration) { s.svc.now = func() time.Time { return noon.Add(d) } }

// glovesShown is the gloves' ITEM_SET where they are, with minutes left in the socket the
// client's tooltip reads.
func glovesShown(cell uint16, minutes int32) []byte {
	return protocol.EncodeItemSet(protocol.ItemPos{Window: 1, Cell: cell}, item.ThiefGloves, 1,
		protocol.Sockets{2: minutes}, protocol.Bonuses{})
}

// A double-click puts the gloves in the first unique place, 97, showing their 120 minutes.
func TestTheGlovesGoInTheFirstUniquePlaceWithTheirTimeShown(t *testing.T) {
	s, _, client := gloved(t, 0)

	got := putOnGloves(t, s, client)

	if !holds(got, glovesShown(97, 120)) {
		t.Fatalf("told %x", got)
	}
	if it, _ := s.inventory.ByID(5); it.Worn != item.Unique1 {
		t.Fatalf("worn in %q", it.Worn)
	}
}

// Dragged onto the second unique place, 98, they are worn there.
func TestTheGlovesMayBeWornInTheSecondUniquePlace(t *testing.T) {
	s, _, client := gloved(t, 0)

	moved := slices.Concat([]byte{1}, binary.LittleEndian.AppendUint16(nil, 20), []byte{1},
		binary.LittleEndian.AppendUint16(nil, 98), []byte{1})
	if err := s.onItemMove(moved); err != nil {
		t.Fatal(err)
	}

	if got := packets(t, drain(t, client)); !holds(got, glovesShown(98, 120)) {
		t.Fatalf("told %x", got)
	}
}

// A second pair is refused while one is worn, as both would only spend their time.
func TestASecondPairOfGlovesIsRefused(t *testing.T) {
	s, _, client := gloved(t, 0)
	putOnGloves(t, s, client)

	if err := s.onItemUse(context.Background(), doubleClicked(21)); err != nil {
		t.Fatal(err)
	}

	if got := packets(t, drain(t, client)); !holds(got, infoLine("Nie możesz założyć dwóch takich samych przedmiotów.")) {
		t.Fatalf("told %x", got)
	}
	if it, _ := s.inventory.ByID(6); it.Worn != "" {
		t.Fatal("the second pair was put on")
	}
}

// Nor may the second pair be dragged onto the other unique place.
func TestASecondPairOfGlovesIsRefusedInTheOtherPlaceToo(t *testing.T) {
	s, _, client := gloved(t, 0)
	putOnGloves(t, s, client)

	moved := slices.Concat([]byte{1}, binary.LittleEndian.AppendUint16(nil, 21), []byte{1},
		binary.LittleEndian.AppendUint16(nil, 98), []byte{1})
	if err := s.onItemMove(moved); err != nil {
		t.Fatal(err)
	}

	if it, _ := s.inventory.ByID(6); it.Worn != "" {
		t.Fatal("the second pair was put on")
	}
}

// Worn, they show a minute less each minute: 119 once a minute is over, not before.
func TestWornGlovesShowAMinuteLessEachMinute(t *testing.T) {
	s, _, client := gloved(t, 0)
	putOnGloves(t, s, client)

	s.svc.tick(noon.Add(59 * time.Second))
	expectNothing(t, client)
	s.svc.tick(noon.Add(61 * time.Second))

	if got := packets(t, drain(t, client)); !holds(got, glovesShown(97, 119)) {
		t.Fatalf("told %x", got)
	}
}

// At their 120th minute the gloves are gone: the client is told so, and at her next move
// of an item her connection forgets them, here and in the store.
func TestGlovesWornTheirTimeAreGone(t *testing.T) {
	s, kept, client := gloved(t, 119*time.Minute)
	putOnGloves(t, s, client)

	s.svc.tick(noon.Add(time.Minute))

	got := packets(t, drain(t, client))
	if !holds(got, itemCleared(item.Item{Worn: item.Unique1}), infoLine("Czas działania przedmiotu Rękawica Złodzieja minął.")) {
		t.Fatalf("told %x", got)
	}
	if _, err := s.catchUp(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, held := s.inventory.ByID(5); held || !slices.Contains(kept.gone, 5) {
		t.Fatalf("still held %v, forgotten %v", held, kept.gone)
	}
}

// Taken off after half an hour, they keep it: 90 minutes left, shown and kept.
func TestGlovesTakenOffKeepTheTimeWorn(t *testing.T) {
	s, kept, client := gloved(t, 0)
	putOnGloves(t, s, client)
	at(s, 30*time.Minute)

	if err := s.onItemUse(context.Background(), doubleClicked(97)); err != nil {
		t.Fatal(err)
	}

	if got := packets(t, drain(t, client)); !holds(got, glovesShown(4, 90)) {
		t.Fatalf("told %x", got)
	}
	if kept.wornFor[5] != 30*time.Minute {
		t.Fatalf("kept worn for %v", kept.wornFor[5])
	}
}

// Off, they count no more: worn half an hour, then put on an hour later, they have 90
// minutes left, and still 90 a moment after.
func TestGlovesCountOnlyWhileWorn(t *testing.T) {
	s, _, client := gloved(t, 30*time.Minute)
	at(s, time.Hour)

	got := putOnGloves(t, s, client)
	s.svc.tick(noon.Add(time.Hour + 30*time.Second))

	if !holds(got, glovesShown(97, 90)) {
		t.Fatalf("told %x", got)
	}
	expectNothing(t, client)
}

// Leaving the world keeps how long the gloves were worn.
func TestLeavingKeepsTheTimeTheGlovesWereWorn(t *testing.T) {
	s, kept, client := gloved(t, 10*time.Minute)
	putOnGloves(t, s, client)
	s.claim = &claim{character: 7, done: make(chan struct{})}
	s.svc.accounts = newSaves()
	at(s, 5*time.Minute)

	s.leaveWorld()

	if kept.wornFor[5] != 15*time.Minute {
		t.Fatalf("kept worn for %v, want 15m", kept.wornFor[5])
	}
}

// The periodic save keeps it too, so a crash loses at most its round.
func TestThePeriodicSaveKeepsTheTimeTheGlovesWereWorn(t *testing.T) {
	s, kept, client := gloved(t, 0)
	putOnGloves(t, s, client)
	s.svc.accounts = newSaves()
	at(s, 7*time.Minute)

	s.svc.saveInWorld(s.svc.record.state.all[1000], quiet)

	if kept.wornFor[5] != 7*time.Minute {
		t.Fatalf("kept worn for %v, want 7m", kept.wornFor[5])
	}
}

// Gloves worn to their end before she last left are forgotten as she is loaded.
func TestSpentGlovesAreForgottenOnLoading(t *testing.T) {
	s, kept, _ := gloved(t, 0)
	s.svc.items = &ordered{keptItems: kept, inv: item.Inventory{
		{ID: 5, VNum: item.ThiefGloves, Count: 1, Worn: item.Unique1, WornFor: 120 * time.Minute},
		{ID: 1, VNum: 10, Count: 1, Cell: 0}}}

	if err := s.loadInventory(context.Background(), 7); err != nil {
		t.Fatal(err)
	}

	if len(s.inventory) != 1 || !slices.Contains(kept.gone, 5) {
		t.Fatalf("carrying %v, forgotten %v", s.inventory, kept.gone)
	}
}

// ordered stands in for the store's items reading a character's: it gives back inv.
type ordered struct {
	*keptItems
	inv item.Inventory
}

func (o *ordered) ApplyOrders(context.Context, uint32) (item.Inventory, []item.Order, error) {
	return slices.Clone(o.inv), nil, nil
}

// Entering the world wearing gloves counts them from then: 119 minutes once a minute in.
func TestGlovesWornOnEnteringCountFromThen(t *testing.T) {
	conn, client := watcher(t)
	svc := worldWith(t)
	svc.now = func() time.Time { return noon }
	entering := &session{svc: svc, conn: conn, phase: protocol.PhaseLoading, playing: &character{vid: 2000,
		Character: account.Character{Name: "Kira", HP: 1, X: homeX, Y: homeY}},
		inventory: item.Inventory{{ID: 5, VNum: item.ThiefGloves, Count: 1, Worn: item.Unique2}}}
	if err := entering.onEnterGame(); err != nil {
		t.Fatal(err)
	}
	drain(t, client)

	svc.tick(noon.Add(time.Minute))

	if got := packets(t, drain(t, client)); !holds(got, glovesShown(98, 119)) {
		t.Fatalf("told %x", got)
	}
}
