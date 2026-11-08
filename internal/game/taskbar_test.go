package game

import (
	"context"
	"encoding/hex"
	"errors"
	"net"
	"testing"

	"github.com/SatoriAI/MT2-Go/internal/account"
	"github.com/SatoriAI/MT2-Go/internal/item"
	"github.com/SatoriAI/MT2-Go/internal/protocol"
	"github.com/SatoriAI/MT2-Go/internal/skill"
)

// keptTaskbar stands in for the store's taskbar: it keeps what each slot holds, and fails
// every write with fail set.
type keptTaskbar struct {
	slots map[uint8]account.Shortcut
	fail  error
}

func (k *keptTaskbar) Slots(context.Context, uint32) (map[uint8]account.Shortcut, error) {
	return k.slots, k.fail
}

func (k *keptTaskbar) Set(_ context.Context, _ uint32, slot uint8, held account.Shortcut) error {
	if k.fail == nil {
		k.slots[slot] = held
	}
	return k.fail
}

func (k *keptTaskbar) Clear(_ context.Context, _ uint32, slot uint8) error {
	if k.fail == nil {
		delete(k.slots, slot)
	}
	return k.fail
}

func (k *keptTaskbar) Swap(_ context.Context, _ uint32, a, b uint8) error {
	if k.fail == nil {
		heldA, okA := k.slots[a]
		heldB, okB := k.slots[b]
		delete(k.slots, a)
		delete(k.slots, b)
		if okA {
			k.slots[b] = heldA
		}
		if okB {
			k.slots[a] = heldB
		}
	}
	return k.fail
}

// onTheTaskbar gives the session a taskbar whose slot 0 holds the item id, read as
// entering reads it, and Hendra as its player if it has none.
func onTheTaskbar(t *testing.T, s *session, id int64) *keptTaskbar {
	t.Helper()
	kept := &keptTaskbar{slots: map[uint8]account.Shortcut{0: {Item: id}}}
	s.svc.taskbar = kept
	if s.playing == nil {
		s.playing = &character{vid: 1000, Character: account.Character{ID: 7, Name: "Hendra"}}
	}
	if err := s.loadTaskbar(context.Background(), 7); err != nil {
		t.Fatal(err)
	}
	return kept
}

func slotAdded(slot, typ, position uint8) []byte { return []byte{slot, typ, position} }

// Her potions dragged to slot 3 fill it: kept, and shown at their cell.
func TestPotionsDraggedToTheTaskbarFillASlot(t *testing.T) {
	s, _, client := carrying(t)
	bar := onTheTaskbar(t, s, 99)
	delete(bar.slots, 0)

	if err := s.onQuickslotAdd(slotAdded(3, 1, 1)); err != nil {
		t.Fatal(err)
	}

	if got := read(t, client, 4); string(got) != string(slotShown(3, shownSlot{cell: 1})) || bar.slots[3] != (account.Shortcut{Item: 2}) || s.taskbar[3] != (shownSlot{account.Shortcut{Item: 2}, 1}) {
		t.Fatalf("told %x, kept %v", got, bar.slots)
	}
}

// Only what is drunk goes on the taskbar from the bag: not a sword, not an empty cell; nor
// a place in the skill window no group has a skill at, nor a slot past the client's 36.
func TestOnlyPotionsFillATaskbarSlot(t *testing.T) {
	for name, body := range map[string][]byte{
		"a sword":       slotAdded(3, 1, 0),
		"an empty cell": slotAdded(3, 1, 9),
		"a skill":       slotAdded(3, 2, 6),
		"slot 36":       slotAdded(36, 1, 1),
	} {
		s, _, client := carrying(t)
		bar := onTheTaskbar(t, s, 2)
		s.svc.presence.all = map[uint32]*present{1000: {char: &s.playing.Character}}

		if err := s.onQuickslotAdd(body); err != nil {
			t.Fatal(err)
		}

		expectNothing(t, client)
		if len(bar.slots) != 1 || len(s.taskbar) != 1 {
			t.Errorf("%s: kept %v", name, bar.slots)
		}
	}
}

// A slot dragged off the taskbar is emptied, and one dragged onto another swaps with it.
func TestTaskbarSlotsAreEmptiedAndSwapped(t *testing.T) {
	s, _, client := carrying(t)
	bar := onTheTaskbar(t, s, 2)

	if err := s.onQuickslotSwap([]byte{0, 5}); err != nil {
		t.Fatal(err)
	}
	if got := read(t, client, 3); string(got) != string(protocol.EncodeQuickslotSwap(0, 5)) || bar.slots[5] != (account.Shortcut{Item: 2}) ||
		s.taskbar[5].held.Item != 2 || len(s.taskbar) != 1 {
		t.Fatalf("told %x, kept %v", got, bar.slots)
	}
	if err := s.onQuickslotDel([]byte{5}); err != nil {
		t.Fatal(err)
	}
	if got := read(t, client, 2); string(got) != string(protocol.EncodeQuickslotDel(5)) || len(bar.slots) != 0 || len(s.taskbar) != 0 {
		t.Fatalf("told %x, kept %v", got, bar.slots)
	}
}

// A slot follows its potions: moved to another cell, the client is shown the slot there.
func TestATaskbarSlotFollowsItsPotionsAsTheyMove(t *testing.T) {
	s, _, client := carrying(t)
	onTheTaskbar(t, s, 2)

	if err := s.onItemMove(dragged(1, 17, 50)); err != nil {
		t.Fatal(err)
	}

	read(t, client, 2*itemSetSize)
	if got := read(t, client, 4); string(got) != string(slotShown(0, shownSlot{cell: 17})) || s.taskbar[0].cell != 17 {
		t.Fatalf("told %x, shown %+v", got, s.taskbar[0])
	}
}

// A potion drunk from a stack that stays where it is tells the taskbar nothing: the slot
// still holds it, at the same cell.
func TestATaskbarSlotIsNotToldAgainWhenItsPotionsStay(t *testing.T) {
	s, _, client, _ := thirsty(t, 1, 50, 1)
	onTheTaskbar(t, s, 10)

	if err := s.onItemUse(doubleClicked(10)); err != nil {
		t.Fatal(err)
	}

	want := string(itemSet(item.Item{ID: 10, VNum: 27001, Count: 49, Cell: 10})) +
		string(protocol.EncodePointChange(1000, protocol.PointHP, 300, 301)) +
		string(protocol.EncodeSpecialEffect(protocol.EffectRedPotion, 1000))
	if got := read(t, client, itemSetSize+pointChangeSize+effectSize); string(got) != want {
		t.Fatalf("got  %x\nwant %x", got, want)
	}
	expectNothing(t, client)
}

// The last potion drunk empties its slot; so does a stack sold.
func TestATaskbarSlotIsEmptiedWhenItsPotionsAreGone(t *testing.T) {
	drinker, _, drinkerClient, _ := thirsty(t, 1, 1, 1)
	onTheTaskbar(t, drinker, 10)
	if err := drinker.onItemUse(doubleClicked(10)); err != nil {
		t.Fatal(err)
	}
	read(t, drinkerClient, itemSetSize)
	expectSlotEmptied(t, drinkerClient, drinker)

	seller, _, sellerClient, store := shopping(t, 100, 10, item.Inventory{{ID: 1, VNum: 27001, Count: 5, Cell: 0}})
	onTheTaskbar(t, seller, 1)
	open(t, seller, sellerClient, store)
	if err := seller.onShop(sold(0, 5)); err != nil {
		t.Fatal(err)
	}
	read(t, sellerClient, itemSetSize+pointChangeSize)
	expectSlotEmptied(t, sellerClient, seller)
}

func expectSlotEmptied(t *testing.T, c net.Conn, s *session) {
	t.Helper()
	if got := read(t, c, 2); string(got) != string(protocol.EncodeQuickslotDel(0)) || len(s.taskbar) != 0 {
		t.Fatalf("got %x, taskbar %v", got, s.taskbar)
	}
}

// A slot that cannot be kept filled, emptied or swapped stays as it was, and the client
// is told nothing.
func TestATaskbarWriteThatFailsChangesNothing(t *testing.T) {
	s, _, client := carrying(t)
	bar := onTheTaskbar(t, s, 2)
	bar.fail = errors.New("the database is gone")

	for _, do := range []func() error{
		func() error { return s.onQuickslotAdd(slotAdded(3, 1, 1)) },
		func() error { return s.onQuickslotDel([]byte{0}) },
		func() error { return s.onQuickslotSwap([]byte{0, 3}) },
	} {
		if err := do(); err != nil {
			t.Fatal(err)
		}
	}

	expectNothing(t, client)
	if len(s.taskbar) != 1 || s.taskbar[0].held.Item != 2 {
		t.Fatalf("taskbar %v", s.taskbar)
	}
}

// onTheTaskbarWithSkills gives a session a taskbar holding nothing yet.
func onTheTaskbarWithSkills(s *session) *keptTaskbar {
	kept := &keptTaskbar{slots: map[uint8]account.Shortcut{}}
	s.svc.taskbar, s.taskbar = kept, map[uint8]shownSlot{}
	return kept
}

// Dragon Swirl, dragged from the skill window's second place, fills a slot: kept as the
// skill and shown as it.
func TestASkillLearntFillsATaskbarSlot(t *testing.T) {
	s, client, _ := swirling(t)
	bar := onTheTaskbarWithSkills(s)

	if err := s.onQuickslotAdd(slotAdded(3, 2, 2)); err != nil {
		t.Fatal(err)
	}

	if got := read(t, client, 4); string(got) != string(mustHexIn(t, "1c030202")) ||
		bar.slots[3] != (account.Shortcut{Skill: skill.DragonSwirl}) {
		t.Fatalf("told %x, kept %v", got, bar.slots)
	}
}

// A skill she has not learnt does not fill a slot.
func TestASkillNotLearntFillsNoTaskbarSlot(t *testing.T) {
	s, client, _ := swirling(t)
	bar := onTheTaskbarWithSkills(s)
	delete(s.svc.presence.all[1000].char.Skills, skill.EnchantedBlade)

	if err := s.onQuickslotAdd(slotAdded(4, 2, 3)); err != nil {
		t.Fatal(err)
	}

	expectNothing(t, client)
	if _, held := bar.slots[4]; held {
		t.Fatal("kept a skill not learnt")
	}
}

// A skill's slot follows no item: a change in the bag leaves it as it is.
func TestASkillsSlotStaysWhenTheBagChanges(t *testing.T) {
	s, _, _ := swirling(t)
	onTheTaskbarWithSkills(s)
	s.taskbar[3] = shownSlot{held: account.Shortcut{Skill: skill.DragonSwirl}}

	if packets := s.followTaskbar(); len(packets) != 0 || s.taskbar[3].held.Skill != skill.DragonSwirl {
		t.Fatalf("told %x, taskbar %v", packets, s.taskbar)
	}
}

// A change of skill group empties the slots of the skills it forgets, and keeps items'.
func TestAChangeOfGroupEmptiesTheSkillsSlots(t *testing.T) {
	s, _, client, _, blackMagic := training(t, account.Sura, 10, "weaponry", map[account.Skill]uint8{skill.DragonSwirl: 1})
	onTheTaskbarWithSkills(s)
	s.taskbar[3] = shownSlot{held: account.Shortcut{Skill: skill.DragonSwirl}}
	s.taskbar[0] = shownSlot{held: account.Shortcut{Item: 9}}
	if err := s.onClick(vidBody(blackMagic)); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := s.onScriptAnswer(context.Background(), answered(0)); err != nil {
			t.Fatal(err)
		}
	}

	read(t, client, len(script("Chcesz porzucić swoją drogę i uczyć się drogi Czarnej Magii? Zapomnisz to, czego się nauczyłeś, ale odzyskasz wszystkie punkty.[ENTER][QUESTION 1;Tak|2;Nie]"))+
		len(script("Czy na pewno? Wybór drogi Czarnej Magii to poważna decyzja.[ENTER][QUESTION 1;Tak|2;Nie]"))+
		2+1531+pointChangeSize+len(script("Od dziś podążasz drogą Czarnej Magii.[ENTER][DONE]")))
	if got := read(t, client, 2); string(got) != string(protocol.EncodeQuickslotDel(3)) {
		t.Fatalf("got %x, want slot 3 emptied", got)
	}
	if _, held := s.taskbar[3]; held || s.taskbar[0].held.Item != 9 {
		t.Fatalf("taskbar %v", s.taskbar)
	}
}

// A skill's slot is shown as the skill at its place in the skill window: Enchanted Blade
// at 3.
func TestASkillsSlotIsShownAtItsPlaceInTheSkillWindow(t *testing.T) {
	if got := slotShown(5, shownSlot{held: account.Shortcut{Skill: skill.EnchantedBlade}}); string(got) != string(mustHexIn(t, "1c050203")) {
		t.Fatalf("got %x", got)
	}
}

func mustHexIn(t *testing.T, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
