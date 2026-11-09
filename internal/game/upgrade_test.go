package game

import (
	"encoding/binary"
	"errors"
	"math/rand/v2"
	"net"
	"testing"

	"github.com/SatoriAI/MT2-Go/internal/item"
	"github.com/SatoriAI/MT2-Go/internal/network"
	"github.com/SatoriAI/MT2-Go/internal/protocol"
	"github.com/SatoriAI/MT2-Go/internal/upgrade"
	"github.com/SatoriAI/MT2-Go/internal/world"
)

// atTheSmith is Hendra, with gold yang and carrying what is given, beside a blacksmith,
// whose vid it returns; her blacksmith's roll of 0 to 99 comes up first as rolled says.
func atTheSmith(t *testing.T, gold uint32, carrying item.Inventory, rolled func(int) bool) (*session, *keptItems, net.Conn, uint32) {
	t.Helper()
	s := worldWith(t, world.Spawn{VNum: 20016, X: homeX, Y: homeY, Count: 1})
	kept := &keptItems{gold: gold, count: map[int64]uint8{}}
	s.items = kept
	client, hendra := aPlayer(t, s, 1000, 7, "Hendra", 100)
	read(t, client, addSize+infoSize) // the blacksmith
	sess := &session{svc: s, conn: hendra.out.(*network.Conn), phase: protocol.PhaseGame,
		playing: playingAs(hendra), inventory: carrying, gold: gold, rand: rolling(rolled)}
	return sess, kept, client, s.lastVID.Load()
}

// rolling is a generator whose first roll of 0 to 99 is one rolled accepts.
func rolling(rolled func(int) bool) *rand.Rand {
	for seed := uint64(1); ; seed++ {
		if rolled(rand.New(rand.NewPCG(seed, 0)).IntN(100)) { //nolint:gosec // a test's fixed seed
			return rand.New(rand.NewPCG(seed, 0)) //nolint:gosec // a test's fixed seed
		}
	}
}

func anyRoll(int) bool { return true }

func givenTo(smith uint32, cell uint16) []byte {
	b := binary.LittleEndian.AppendUint32(nil, smith)
	b = append(b, 1)
	b = binary.LittleEndian.AppendUint16(b, cell)
	return append(b, 1)
}

func refined(cell, typ uint8) []byte { return []byte{cell, typ} }

func expectChatInfo(t *testing.T, c net.Conn, line string) {
	t.Helper()
	want := chatInfo(line)
	if got := read(t, c, len(want)); string(got) != string(want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}

// A sword +0 given to the blacksmith opens the refine dialog on it: into +1 for 200 yang,
// sure to succeed, with no material and no question asked.
func TestASwordGivenToTheBlacksmithOpensTheRefineDialog(t *testing.T) {
	s, _, client, smith := atTheSmith(t, 0, item.Inventory{{ID: 1, VNum: 10, Count: 1, Cell: 3}}, anyRoll)

	if err := s.onGiveItem(givenTo(smith, 3)); err != nil {
		t.Fatal(err)
	}

	want := protocol.EncodeRefineInfo(protocol.RefineInfo{Type: protocol.RefineNoQuestion, Cell: 3, From: 10, Into: 11, Cost: 200, Chance: 100})
	if got := read(t, client, len(want)); string(got) != string(want) || s.upgrading == nil || s.upgrading.item != 1 {
		t.Fatalf("got %x, upgrading %+v", got, s.upgrading)
	}
}

// From +4 the dialog lists the claw it takes and warns of a lower grade.
func TestTheStepToFiveAsksAClawAndWarnsOfADowngrade(t *testing.T) {
	s, _, client, smith := atTheSmith(t, 0, item.Inventory{{ID: 1, VNum: 14, Count: 1, Cell: 3}}, anyRoll)

	if err := s.onGiveItem(givenTo(smith, 3)); err != nil {
		t.Fatal(err)
	}

	want := protocol.EncodeRefineInfo(protocol.RefineInfo{Type: protocol.RefineMayDowngrade, Cell: 3, From: 14, Into: 15, Cost: 3000, Chance: 60,
		Materials: []protocol.Material{{VNum: upgrade.WolfClaw, Count: 1}}})
	if got := read(t, client, len(want)); string(got) != string(want) {
		t.Fatalf("got %x", got)
	}
}

// A potion, a sword at +9, a worn sword and an empty cell are not upgraded, and the
// blacksmith says so.
func TestTheBlacksmithSaysWhatCannotBeUpgraded(t *testing.T) {
	carried := item.Inventory{{ID: 1, VNum: 27001, Count: 5, Cell: 0}, {ID: 2, VNum: 19, Count: 1, Cell: 1},
		{ID: 3, VNum: 10, Count: 1, Worn: item.Weapon}}
	for name, cell := range map[string]uint16{"a potion": 0, "a +9": 1, "worn": wornCells[item.Weapon], "nothing": 9} {
		s, _, client, smith := atTheSmith(t, 0, carried, anyRoll)

		if err := s.onGiveItem(givenTo(smith, cell)); err != nil {
			t.Fatal(err)
		}

		expectSaid(t, client, upgradeNotPossible+"[ENTER][DONE]")
		if s.upgrading != nil {
			t.Errorf("%s: upgrading", name)
		}
	}
}

// Given to anyone but a blacksmith in reach, nothing happens.
func TestOnlyABlacksmithTakesAnItem(t *testing.T) {
	s, _, client, smith := atTheSmith(t, 0, item.Inventory{{ID: 1, VNum: 10, Count: 1, Cell: 3}}, anyRoll)
	for _, to := range []uint32{1000, smith + 1} { // herself, and no one
		if err := s.onGiveItem(givenTo(to, 3)); err != nil {
			t.Fatal(err)
		}
	}
	expectNothing(t, client)
	if s.upgrading != nil {
		t.Fatal("upgrading")
	}
}

// Yes in the dialog upgrades the sword: its new kind and the yang spent are kept and told,
// and the blacksmith says it worked.
func TestAnUpgradeThatSucceedsRaisesTheGrade(t *testing.T) {
	s, kept, client, smith := atTheSmith(t, 500, item.Inventory{{ID: 1, VNum: 10, Count: 1, Cell: 3}}, anyRoll)
	giveAndRead(t, s, client, smith)

	if err := s.onRefine(t.Context(), refined(3, protocol.RefineNoQuestion)); err != nil {
		t.Fatal(err)
	}

	upgraded := item.Item{ID: 1, VNum: 11, Count: 1, Cell: 3}
	if got := read(t, client, itemSetSize); string(got) != string(itemSet(upgraded)) {
		t.Fatalf("told %x", got)
	}
	if got := read(t, client, pointChangeSize); string(got) != string(protocol.EncodePointChange(1000, protocol.PointGold, -200, 300)) {
		t.Fatalf("told %x", got)
	}
	expectChatInfo(t, client, upgradeSucceeded)
	if it, _ := s.inventory.ByID(1); kept.upgraded != upgraded || it != upgraded || s.gold != 300 || s.upgrading != nil {
		t.Fatalf("kept %+v, holds %+v, yang %d", kept.upgraded, it, s.gold)
	}
}

// The step to +3 succeeds on a roll of 79, the last of its 80 in 100.
func TestAnUpgradeSucceedsOnItsChance(t *testing.T) {
	s, kept, client, smith := atTheSmith(t, 1000, item.Inventory{{ID: 1, VNum: 12, Count: 1, Cell: 3}}, func(r int) bool { return r == 79 })
	giveAndRead(t, s, client, smith)

	if err := s.onRefine(t.Context(), refined(3, protocol.RefineNoQuestion)); err != nil {
		t.Fatal(err)
	}

	read(t, client, itemSetSize+pointChangeSize)
	expectChatInfo(t, client, upgradeSucceeded)
	if kept.upgraded.VNum != 13 {
		t.Fatalf("kept %+v", kept.upgraded)
	}
}

// A failure on the step to +3, on a roll of 80, loses the yang and keeps the sword at +2.
func TestAFailureBelowFiveKeepsTheGrade(t *testing.T) {
	s, kept, client, smith := atTheSmith(t, 1000, item.Inventory{{ID: 1, VNum: 12, Count: 1, Cell: 3}}, func(r int) bool { return r == 80 })
	giveAndRead(t, s, client, smith)

	if err := s.onRefine(t.Context(), refined(3, protocol.RefineNoQuestion)); err != nil {
		t.Fatal(err)
	}

	read(t, client, itemSetSize+pointChangeSize)
	expectChatInfo(t, client, upgradeFailed)
	if kept.upgraded.VNum != 12 || s.gold != 200 {
		t.Fatalf("kept %+v, yang %d", kept.upgraded, s.gold)
	}
}

// A failure on the step to +5 spends the claw too, and drops the sword to +3.
func TestAFailureFromTheStepToFiveCostsAGrade(t *testing.T) {
	carried := item.Inventory{{ID: 1, VNum: 14, Count: 1, Cell: 3}, {ID: 2, VNum: upgrade.WolfClaw, Count: 1, Cell: 9}}
	s, kept, client, smith := atTheSmith(t, 3000, carried, func(r int) bool { return r == 60 })
	giveAndRead(t, s, client, smith)

	if err := s.onRefine(t.Context(), refined(3, protocol.RefineMayDowngrade)); err != nil {
		t.Fatal(err)
	}

	if got := read(t, client, 2*itemSetSize); string(got) != string(itemSet(item.Item{ID: 1, VNum: 13, Count: 1, Cell: 3}))+string(itemCleared(item.Item{ID: 2, VNum: upgrade.WolfClaw, Cell: 9})) {
		t.Fatalf("told %x", got)
	}
	read(t, client, pointChangeSize)
	expectChatInfo(t, client, upgradeFailed)
	if _, claw := s.inventory.ByID(2); kept.upgraded.VNum != 13 || claw || s.gold != 0 {
		t.Fatalf("kept %+v, holds %+v", kept.upgraded, s.inventory)
	}
}

// The two claws the step to +6 takes come from the first stacks found: the one claw at 9
// is used up, the stack of three at 10 left with two, and the stack at 11 untouched.
func TestMaterialsAreTakenFromTheFirstStacks(t *testing.T) {
	carried := item.Inventory{{ID: 1, VNum: 15, Count: 1, Cell: 3}, {ID: 2, VNum: upgrade.WolfClaw, Count: 1, Cell: 9},
		{ID: 3, VNum: upgrade.WolfClaw, Count: 3, Cell: 10}, {ID: 4, VNum: upgrade.WolfClaw, Count: 7, Cell: 11}}
	s, kept, client, smith := atTheSmith(t, 5000, carried, anyRoll)
	giveAndRead(t, s, client, smith)

	if err := s.onRefine(t.Context(), refined(3, protocol.RefineMayDowngrade)); err != nil {
		t.Fatal(err)
	}

	read(t, client, itemSetSize)
	left := item.Item{ID: 3, VNum: upgrade.WolfClaw, Count: 2, Cell: 10}
	if got := read(t, client, 2*itemSetSize); string(got) != string(itemCleared(item.Item{ID: 2, VNum: upgrade.WolfClaw, Cell: 9}))+string(itemSet(left)) {
		t.Fatalf("told %x", got)
	}
	if len(kept.spent) != 2 || kept.spent[0].Count != 0 || kept.spent[1] != left || len(s.inventory) != 3 {
		t.Fatalf("spent %+v, holds %+v", kept.spent, s.inventory)
	}
}

// Without the yang or the claws, the upgrade is refused and told why; nothing is spent.
func TestAnUpgradeWithoutItsYangOrMaterialIsRefused(t *testing.T) {
	cases := map[string]struct {
		gold  uint32
		claws uint8
		said  string
	}{
		"yang":   {2999, 1, upgradeNoYang},
		"a claw": {3000, 0, upgradeNoMaterial},
	}
	for name, c := range cases {
		carried := item.Inventory{{ID: 1, VNum: 14, Count: 1, Cell: 3}}
		if c.claws > 0 {
			carried = append(carried, item.Item{ID: 2, VNum: upgrade.WolfClaw, Count: c.claws, Cell: 9})
		}
		s, kept, client, smith := atTheSmith(t, c.gold, carried, anyRoll)
		giveAndRead(t, s, client, smith)

		if err := s.onRefine(t.Context(), refined(3, protocol.RefineMayDowngrade)); err != nil {
			t.Fatal(err)
		}

		expectChatInfo(t, client, c.said)
		if kept.upgraded.ID != 0 || s.gold != c.gold {
			t.Errorf("%s: upgraded %+v", name, kept.upgraded)
		}
	}
}

// Closing the dialog, an answer with none open, an answer for another cell, and one given
// once the blacksmith is out of reach do nothing; the dialog is closed by any answer.
func TestAnUpgradeIsOnlyOfTheItemGivenWhileTheBlacksmithIsNear(t *testing.T) {
	cases := map[string]func(s *session) []byte{
		"closed":       func(*session) []byte { return refined(protocol.RefineCancel, protocol.RefineCancel) },
		"none open":    func(s *session) []byte { s.upgrading = nil; return refined(3, protocol.RefineNoQuestion) },
		"another cell": func(*session) []byte { return refined(4, protocol.RefineNoQuestion) },
		"another item": func(s *session) []byte { s.upgrading.item = 2; return refined(3, protocol.RefineNoQuestion) },
		"walked off": func(s *session) []byte {
			s.svc.presence.all[1000].add.X += npcReach + 100
			return refined(3, protocol.RefineNoQuestion)
		},
	}
	for name, answer := range cases {
		carried := item.Inventory{{ID: 1, VNum: 10, Count: 1, Cell: 3}, {ID: 2, VNum: 10, Count: 1, Cell: 4}}
		s, kept, client, smith := atTheSmith(t, 500, carried, anyRoll)
		giveAndRead(t, s, client, smith)

		if err := s.onRefine(t.Context(), answer(s)); err != nil {
			t.Fatal(err)
		}

		expectNothing(t, client)
		if kept.upgraded.ID != 0 || s.gold != 500 || s.upgrading != nil {
			t.Errorf("%s: upgraded %+v", name, kept.upgraded)
		}
	}
}

// An upgrade the database does not keep changes nothing she holds.
func TestAnUpgradeNotKeptChangesNothing(t *testing.T) {
	s, kept, client, smith := atTheSmith(t, 500, item.Inventory{{ID: 1, VNum: 10, Count: 1, Cell: 3}}, anyRoll)
	giveAndRead(t, s, client, smith)
	kept.fail = errors.New("down")

	if err := s.onRefine(t.Context(), refined(3, protocol.RefineNoQuestion)); err != nil {
		t.Fatal(err)
	}

	expectNothing(t, client)
	if it, _ := s.inventory.ByID(1); it.VNum != 10 || s.gold != 500 {
		t.Fatalf("holds %+v, yang %d", it, s.gold)
	}
}

// giveAndRead gives the blacksmith the item at cell 3 and reads the dialog.
func giveAndRead(t *testing.T, s *session, client net.Conn, smith uint32) {
	t.Helper()
	if err := s.onGiveItem(givenTo(smith, 3)); err != nil {
		t.Fatal(err)
	}
	read(t, client, 60)
}

// The general store beside her takes no sword: nothing happens.
func TestAShopkeeperTakesNoItem(t *testing.T) {
	s, _, client, store := shopping(t, 100, 0, item.Inventory{{ID: 1, VNum: 10, Count: 1, Cell: 3}})

	if err := s.onGiveItem(givenTo(store, 3)); err != nil {
		t.Fatal(err)
	}

	expectNothing(t, client)
	if s.upgrading != nil {
		t.Fatal("upgrading")
	}
}

// A potion given after the sword closes the sword's dialog: a yes for the sword then
// upgrades nothing.
func TestAGiveAfterwardsClosesTheDialog(t *testing.T) {
	carried := item.Inventory{{ID: 1, VNum: 10, Count: 1, Cell: 3}, {ID: 2, VNum: 27001, Count: 1, Cell: 4}}
	s, kept, client, smith := atTheSmith(t, 500, carried, anyRoll)
	giveAndRead(t, s, client, smith)
	if err := s.onGiveItem(givenTo(smith, 4)); err != nil {
		t.Fatal(err)
	}
	expectSaid(t, client, upgradeNotPossible+"[ENTER][DONE]")

	if err := s.onRefine(t.Context(), refined(3, protocol.RefineNoQuestion)); err != nil {
		t.Fatal(err)
	}

	expectNothing(t, client)
	if kept.upgraded.ID != 0 || s.gold != 500 {
		t.Fatalf("upgraded %+v", kept.upgraded)
	}
}
