package game

import (
	"encoding/binary"
	"net"
	"testing"

	"github.com/SatoriAI/MT2-Go/internal/item"
	"github.com/SatoriAI/MT2-Go/internal/network"
	"github.com/SatoriAI/MT2-Go/internal/protocol"
	"github.com/SatoriAI/MT2-Go/internal/upgrade"
)

// scrolling is Hendra with gold yang, carrying what is given, with no blacksmith about; her
// first roll of 0 to 99 comes up as rolled says.
func scrolling(t *testing.T, gold uint32, carrying item.Inventory, rolled func(int) bool) (*session, *keptItems, net.Conn) {
	t.Helper()
	s := worldWith(t)
	kept := &keptItems{gold: gold, count: map[int64]uint8{}}
	s.items = kept
	client, hendra := aPlayer(t, s, 1000, 7, "Hendra", 100)
	drain(t, client)
	sess := &session{svc: s, conn: hendra.out.(*network.Conn), phase: protocol.PhaseGame,
		playing: playingAs(hendra), inventory: carrying, gold: gold, rand: rolling(rolled)}
	return sess, kept, client
}

// scrollDropped is ITEM_USE_TO_ITEM's body: the bag item at cell 7, the scroll in these
// tests, dropped onto the one at cell 3.
func scrollDropped() []byte {
	b := binary.LittleEndian.AppendUint16([]byte{1}, 7)
	b = append(b, 1)
	return binary.LittleEndian.AppendUint16(b, 3)
}

// A sword +4 and its claw, with the scroll at cell 7.
func blessable() item.Inventory {
	return item.Inventory{{ID: 1, VNum: 14, Count: 1, Cell: 3}, {ID: 2, VNum: upgrade.WolfClaw, Count: 1, Cell: 9},
		{ID: 5, VNum: item.BlessingScroll, Count: 1, Cell: 7}}
}

// The scroll dropped onto a sword +4 opens the refine dialog for the step to +5, as the
// blacksmith's would but asking no question, with no blacksmith about, and says what it
// guards.
func TestTheScrollOntoGearOpensTheBlessedRefineDialog(t *testing.T) {
	s, _, client := scrolling(t, 3000, blessable(), anyRoll)

	if err := s.onItemUseToItem(scrollDropped()); err != nil {
		t.Fatal(err)
	}

	want := protocol.EncodeRefineInfo(protocol.RefineInfo{Type: protocol.RefineNoQuestion, Cell: 3, From: 14, Into: 15, Cost: 3000, Chance: 60,
		Materials: []protocol.Material{{VNum: upgrade.WolfClaw, Count: 1}}})
	if got := packets(t, drain(t, client)); len(got) != 2 || string(got[0]) != string(want) || string(got[1]) != string(infoLine(scrollWarns)) {
		t.Fatalf("told %x", got)
	}
}

// A failure under the scroll spends the yang, the claw and the scroll, and keeps the sword
// at +4.
func TestAFailureUnderTheScrollKeepsTheGrade(t *testing.T) {
	s, kept, client := scrolling(t, 3000, blessable(), func(r int) bool { return r == 60 })
	if err := s.onItemUseToItem(scrollDropped()); err != nil {
		t.Fatal(err)
	}
	drain(t, client)

	if err := s.onRefine(t.Context(), refined(3, protocol.RefineNoQuestion)); err != nil {
		t.Fatal(err)
	}

	got := packets(t, drain(t, client))
	if !holds(got, itemCleared(item.Item{ID: 5, VNum: item.BlessingScroll, Cell: 7})) || !holds(got, infoLine(upgradeKept)) {
		t.Fatalf("told %x", got)
	}
	_, claw := s.inventory.ByID(2)
	_, scroll := s.inventory.ByID(5)
	if kept.upgraded.VNum != 14 || claw || scroll || s.gold != 0 || len(kept.spent) != 2 {
		t.Fatalf("kept %+v and spent %+v, holds %+v, yang %d", kept.upgraded, kept.spent, s.inventory, s.gold)
	}
}

// A success under the scroll raises the sword to +5 and spends the scroll too.
func TestASuccessUnderTheScrollSpendsItToo(t *testing.T) {
	s, kept, client := scrolling(t, 3000, blessable(), func(r int) bool { return r < 60 })
	if err := s.onItemUseToItem(scrollDropped()); err != nil {
		t.Fatal(err)
	}
	drain(t, client)

	if err := s.onRefine(t.Context(), refined(3, protocol.RefineNoQuestion)); err != nil {
		t.Fatal(err)
	}

	if _, scroll := s.inventory.ByID(5); kept.upgraded.VNum != 15 || scroll {
		t.Fatalf("kept %+v, holds %+v", kept.upgraded, s.inventory)
	}
	if got := packets(t, drain(t, client)); !holds(got, infoLine(upgradeSucceeded)) {
		t.Fatalf("told %x", got)
	}
}

// Below the step to +5, where a failure costs only yang, the scroll is refused and kept.
func TestTheScrollIsRefusedWhereAFailureCostsOnlyYang(t *testing.T) {
	s, _, client := scrolling(t, 3000, item.Inventory{{ID: 1, VNum: 13, Count: 1, Cell: 3},
		{ID: 5, VNum: item.BlessingScroll, Count: 1, Cell: 7}}, anyRoll)

	if err := s.onItemUseToItem(scrollDropped()); err != nil {
		t.Fatal(err)
	}

	if got := packets(t, drain(t, client)); len(got) != 1 || string(got[0]) != string(infoLine(scrollNotNeeded)) || s.upgrading != nil {
		t.Fatalf("told %x, upgrading %+v", got, s.upgrading)
	}
}

// Onto what is not upgraded, a potion, the scroll is refused as the blacksmith refuses it.
func TestTheScrollIsRefusedOntoWhatIsNotUpgraded(t *testing.T) {
	s, _, client := scrolling(t, 0, item.Inventory{{ID: 1, VNum: 27001, Count: 5, Cell: 3},
		{ID: 5, VNum: item.BlessingScroll, Count: 1, Cell: 7}}, anyRoll)

	if err := s.onItemUseToItem(scrollDropped()); err != nil {
		t.Fatal(err)
	}

	if got := packets(t, drain(t, client)); len(got) != 1 || string(got[0]) != string(infoLine(upgradeNotPossible)) {
		t.Fatalf("told %x", got)
	}
}

// Without the yang, the upgrade is refused and the scroll kept.
func TestAnUpgradeUnderTheScrollWithoutItsYangKeepsTheScroll(t *testing.T) {
	s, kept, client := scrolling(t, 2999, blessable(), anyRoll)
	if err := s.onItemUseToItem(scrollDropped()); err != nil {
		t.Fatal(err)
	}
	drain(t, client)

	if err := s.onRefine(t.Context(), refined(3, protocol.RefineNoQuestion)); err != nil {
		t.Fatal(err)
	}

	expectChatInfo(t, client, upgradeNoYang)
	if _, scroll := s.inventory.ByID(5); !scroll || kept.upgraded.ID != 0 {
		t.Fatalf("kept %+v, holds %+v", kept.upgraded, s.inventory)
	}
}

// A scroll gone from the bag before the answer, traded away, upgrades nothing.
func TestAScrollGoneBeforeTheAnswerUpgradesNothing(t *testing.T) {
	s, kept, client := scrolling(t, 3000, blessable(), anyRoll)
	if err := s.onItemUseToItem(scrollDropped()); err != nil {
		t.Fatal(err)
	}
	drain(t, client)
	s.inventory = s.inventory[:2]

	if err := s.onRefine(t.Context(), refined(3, protocol.RefineNoQuestion)); err != nil {
		t.Fatal(err)
	}

	expectNothing(t, client)
	if kept.upgraded.ID != 0 {
		t.Fatalf("kept %+v", kept.upgraded)
	}
}

// Anything but the scroll dropped onto gear does nothing yet.
func TestAnotherItemDroppedOntoGearDoesNothing(t *testing.T) {
	s, _, client := scrolling(t, 0, item.Inventory{{ID: 1, VNum: 14, Count: 1, Cell: 3},
		{ID: 5, VNum: upgrade.WolfClaw, Count: 1, Cell: 7}}, anyRoll)

	if err := s.onItemUseToItem(scrollDropped()); err != nil {
		t.Fatal(err)
	}

	expectNothing(t, client)
}

// Onto a worn sword the scroll is refused, as the blacksmith refuses worn gear.
func TestTheScrollIsRefusedOntoWornGear(t *testing.T) {
	carried := blessable()
	carried[0].Worn, carried[0].Cell = item.Weapon, 0
	s, _, client := scrolling(t, 3000, carried, anyRoll)

	if err := s.onItemUseToItem(append(scrollDropped()[:3], 1, 94, 0)); err != nil { // the weapon's place
		t.Fatal(err)
	}

	if got := packets(t, drain(t, client)); len(got) != 1 || string(got[0]) != string(infoLine(upgradeNotPossible)) {
		t.Fatalf("told %x", got)
	}
}

// The sword given the blacksmith after the scroll's dialog opened is his to upgrade: a
// failure costs a grade, and the scroll is kept.
func TestABlacksmithsUpgradeAfterTheScrollsDialogLeavesTheScroll(t *testing.T) {
	s, kept, client, smith := atTheSmith(t, 3000, blessable(), func(r int) bool { return r == 60 })
	if err := s.onItemUseToItem(scrollDropped()); err != nil {
		t.Fatal(err)
	}
	drain(t, client)
	giveAndRead(t, s, client, smith)

	if err := s.onRefine(t.Context(), refined(3, protocol.RefineMayDowngrade)); err != nil {
		t.Fatal(err)
	}

	if _, scroll := s.inventory.ByID(5); kept.upgraded.VNum != 13 || !scroll {
		t.Fatalf("kept %+v, holds %+v", kept.upgraded, s.inventory)
	}
}

// A scroll refused onto a potion leaves the blacksmith's dialog open, and its yes upgrades.
func TestARefusedScrollLeavesTheBlacksmithsDialogOpen(t *testing.T) {
	carried := append(blessable(), item.Item{ID: 6, VNum: 27001, Count: 1, Cell: 20})
	s, kept, client, smith := atTheSmith(t, 3000, carried, func(r int) bool { return r < 60 })
	giveAndRead(t, s, client, smith)
	if err := s.onItemUseToItem(append(scrollDropped()[:3], 1, 20, 0)); err != nil {
		t.Fatal(err)
	}
	drain(t, client)

	if err := s.onRefine(t.Context(), refined(3, protocol.RefineMayDowngrade)); err != nil {
		t.Fatal(err)
	}

	if _, scroll := s.inventory.ByID(5); kept.upgraded.VNum != 15 || !scroll {
		t.Fatalf("kept %+v, holds %+v", kept.upgraded, s.inventory)
	}
}
