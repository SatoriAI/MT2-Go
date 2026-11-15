package game

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/SatoriAI/MT2-Go/internal/account"
	"github.com/SatoriAI/MT2-Go/internal/item"
)

// enchantable is Hendra's level-30 sword with two ordinary bonuses and the pair, at cell
// 3, and two of Zaczarowanie Przedmiotu at cell 7.
func enchantable() item.Inventory {
	return item.Inventory{
		{ID: 1, VNum: 290, Count: 1, Cell: 3, Bonuses: item.Bonuses{{Kind: account.BonusCritical, Value: 1}, {Kind: account.BonusPierce, Value: 1},
			{Kind: account.BonusAverage, Value: 20}, {Kind: account.BonusSkill, Value: -10}}},
		{ID: 5, VNum: item.EnchantItem, Count: 2, Cell: 7},
	}
}

const enchantQuestion = "Czy chcesz zmienić bonusy przedmiotu Miecz Pełni Księżyca+0?[ENTER]Szansa na cios krytyczny: 1%[ENTER]" +
	"Szansa na przeszywające Uderzenie: 1%[ENTER]Średnie Obrażenia: +20%[ENTER]Obrażenie Umiejętności: -10%[ENTER]" +
	"Zwykłe bonusy zostaną zastąpione nowymi, a para zostanie.[ENTER][QUESTION 1;Tak|2;Nie]"

// The scroll dropped onto gear asks first, showing every bonus the gear carries, and keeps
// both as they are until the answer.
func TestEnchantingAsksFirstShowingTheBonuses(t *testing.T) {
	s, kept, client := scrolling(t, 0, enchantable(), anyRoll)

	if err := s.onItemUseToItem(scrollDropped()); err != nil {
		t.Fatal(err)
	}

	expectSaid(t, client, enchantQuestion)
	if kept.enchanted.ID != 0 || s.talking == nil || s.talking.enchant != 5 || s.talking.enchanted != 1 {
		t.Fatalf("kept %+v, talking %+v", kept.enchanted, s.talking)
	}
}

// Yes rolls the two ordinary bonuses afresh, of the weapon's kinds, keeps the pair, spends
// one scroll and shows it all.
func TestYesRollsTheOrdinaryBonusesAndKeepsThePair(t *testing.T) {
	s, kept, client := scrolling(t, 0, enchantable(), anyRoll)
	if err := s.onItemUseToItem(scrollDropped()); err != nil {
		t.Fatal(err)
	}
	drain(t, client)

	if err := s.onScriptAnswer(context.Background(), answered(0)); err != nil {
		t.Fatal(err)
	}

	got := kept.enchanted.Bonuses
	if len(got) != 4 || got.Ordinary() != 2 || !reflect.DeepEqual(got[2:], enchantable()[0].Bonuses[2:]) {
		t.Fatalf("enchanted to %+v", got)
	}
	for _, b := range got[:2] {
		if b.Kind == account.BonusMaxHP || b.Kind == account.BonusResist || b.Kind == account.BonusAttackSpeed {
			t.Errorf("a weapon rolled %+v", b)
		}
	}
	left := item.Item{ID: 5, VNum: item.EnchantItem, Count: 1, Cell: 7}
	if !reflect.DeepEqual(kept.scroll, left) {
		t.Fatalf("scroll left %+v", kept.scroll)
	}
	sword, _ := s.inventory.ByID(1)
	if p := packets(t, drain(t, client)); !holds(p, itemSet(sword), itemSet(left), infoLine(enchantDone)) {
		t.Fatalf("told %x", p)
	}
}

// The last scroll spent leaves its cell empty.
func TestTheLastEnchantScrollIsUsedUp(t *testing.T) {
	carried := enchantable()
	carried[1].Count = 1
	s, kept, client := scrolling(t, 0, carried, anyRoll)
	if err := s.onItemUseToItem(scrollDropped()); err != nil {
		t.Fatal(err)
	}
	drain(t, client)

	if err := s.onScriptAnswer(context.Background(), answered(0)); err != nil {
		t.Fatal(err)
	}

	if _, held := s.inventory.ByID(5); held || kept.scroll.Count != 0 {
		t.Fatalf("holds %+v, kept %+v", s.inventory, kept.scroll)
	}
	if p := packets(t, drain(t, client)); !holds(p, itemCleared(item.Item{ID: 5, VNum: item.EnchantItem, Cell: 7})) {
		t.Fatalf("told %x", p)
	}
}

// No changes nothing, and nor does a yes after the gear left the bag.
func TestNoOrGearGoneEnchantsNothing(t *testing.T) {
	for name, after := range map[string]func(s *session) uint8{
		"no":        func(*session) uint8 { return 1 },
		"gear gone": func(s *session) uint8 { s.inventory = s.inventory[1:]; return 0 },
		"worn":      func(s *session) uint8 { s.inventory[0].Worn = item.Weapon; return 0 },
	} {
		s, kept, client := scrolling(t, 0, enchantable(), anyRoll)
		if err := s.onItemUseToItem(scrollDropped()); err != nil {
			t.Fatal(err)
		}
		drain(t, client)

		if err := s.onScriptAnswer(context.Background(), answered(after(s))); err != nil {
			t.Fatal(err)
		}

		if kept.enchanted.ID != 0 {
			t.Errorf("%s: enchanted %+v", name, kept.enchanted)
		}
		expectNothing(t, client)
	}
}

// Gear with no ordinary bonus, only the pair or none, and what is not gear, are refused,
// the scroll kept and nothing asked.
func TestEnchantingIsRefusedWithNothingToRoll(t *testing.T) {
	for name, target := range map[string]item.Item{
		"no bonus":  {ID: 1, VNum: 290, Count: 1, Cell: 3},
		"only pair": {ID: 1, VNum: 290, Count: 1, Cell: 3, Bonuses: item.Bonuses{{Kind: account.BonusAverage, Value: 20}, {Kind: account.BonusSkill, Value: -10}}},
		"a potion":  {ID: 1, VNum: 27001, Count: 5, Cell: 3},
	} {
		s, _, client := scrolling(t, 0, item.Inventory{target, {ID: 5, VNum: item.EnchantItem, Count: 2, Cell: 7}}, anyRoll)

		if err := s.onItemUseToItem(scrollDropped()); err != nil {
			t.Fatal(err)
		}

		if got := packets(t, drain(t, client)); len(got) != 1 || string(got[0]) != string(infoLine(enchantNothing)) || s.talking != nil {
			t.Errorf("%s: told %x", name, got)
		}
	}
}

// A write that fails changes nothing the player holds.
func TestAnEnchantingNotKeptChangesNothing(t *testing.T) {
	s, kept, client := scrolling(t, 0, enchantable(), anyRoll)
	if err := s.onItemUseToItem(scrollDropped()); err != nil {
		t.Fatal(err)
	}
	drain(t, client)
	kept.fail = errors.New("down")

	if err := s.onScriptAnswer(context.Background(), answered(0)); err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(s.inventory, enchantable()) {
		t.Fatalf("holds %+v", s.inventory)
	}
	expectNothing(t, client)
}

// A yes while a trade is open, the gear perhaps offered in it, is refused: the other side
// would take bonuses they never saw.
func TestAYesDuringATradeIsRefused(t *testing.T) {
	s, kept, client := scrolling(t, 0, enchantable(), anyRoll)
	if err := s.onItemUseToItem(scrollDropped()); err != nil {
		t.Fatal(err)
	}
	drain(t, client)
	s.svc.record.held(func(l locked) { l.trades[1000] = &trade{} })

	if err := s.onScriptAnswer(context.Background(), answered(0)); err != nil {
		t.Fatal(err)
	}

	expectChatInfo(t, client, "Nie możesz tego zrobić podczas handlu.")
	if kept.enchanted.ID != 0 {
		t.Fatalf("enchanted %+v", kept.enchanted)
	}
}
