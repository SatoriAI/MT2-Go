package game

import (
	"context"
	"reflect"
	"testing"

	"github.com/SatoriAI/MT2-Go/internal/item"
)

// tester is Hendra with the development switch on, out of sight of anyone, with gold yang
// and carrying what is given.
func tester(t *testing.T, gold uint32, carrying item.Inventory) (*session, *keptItems, func() [][]byte) {
	t.Helper()
	s, kept, client, _ := shopping(t, 20000, gold, carrying)
	s.svc.dev = true
	return s, kept, func() [][]byte { return packets(t, drain(t, client)) }
}

// command types the line, without its slash, as the chat sends it.
func command(t *testing.T, s *session, line string) {
	t.Helper()
	if err := s.onCommand(context.Background(), line); err != nil {
		t.Fatal(err)
	}
}

// /getitem puts one of the kind in her bag's first free cells, kept and shown.
func TestGetItemPutsTheKindInTheBag(t *testing.T) {
	s, kept, told := tester(t, 0, nil)

	command(t, s, "getitem 10")

	got := item.Item{ID: 101, VNum: 10, Count: 1, Cell: 0}
	if !reflect.DeepEqual(kept.added, []item.Item{{VNum: 10, Count: 1}}) || !reflect.DeepEqual(s.inventory, item.Inventory{got}) {
		t.Fatalf("kept %+v, holds %+v", kept.added, s.inventory)
	}
	if p := told(); !holds(p, itemSet(got)) || len(p) != 1 {
		t.Fatalf("told %x", p)
	}
}

// A count joins her stack of the kind where it has room, and is at most a stack.
func TestGetItemCountsJoinAStackAndStopAtAFullOne(t *testing.T) {
	s, kept, _ := tester(t, 0, item.Inventory{{ID: 1, VNum: 8000, Count: 50, Cell: 0}})

	command(t, s, "getitem 8000 100")
	command(t, s, "getitem 27001 300")

	if kept.count[1] != 150 || !reflect.DeepEqual(kept.added, []item.Item{{VNum: 27001, Count: item.MaxStack, Cell: 1}}) {
		t.Fatalf("stack %d, kept %+v", kept.count[1], kept.added)
	}
}

// /getitem says what it cannot do and gives nothing: a kind the server does not know, a
// line it cannot read, and a full bag.
func TestGetItemSaysWhatItCannotGive(t *testing.T) {
	full := item.Inventory{}
	for cell := range uint16(item.Cells) {
		full = append(full, item.Item{ID: int64(cell + 1), VNum: 27001, Count: item.MaxStack, Cell: cell})
	}
	for line, want := range map[string]string{
		"getitem 4242":    "Serwer nie zna przedmiotu 4242.",
		"getitem":         "Użycie: /getitem VNUM [ilość]",
		"getitem miecz":   "Użycie: /getitem VNUM [ilość]",
		"getitem 10 0":    "Użycie: /getitem VNUM [ilość]",
		"getitem 10 1 2":  "Użycie: /getitem VNUM [ilość]",
		"getitem 27001 1": "Nie masz miejsca w ekwipunku.",
	} {
		s, kept, told := tester(t, 0, full)
		command(t, s, line)
		if p := told(); len(kept.added) != 0 || len(kept.count) != 0 || !holds(p, infoLine(want)) {
			t.Errorf("/%s: kept %+v %v, told %x", line, kept.added, kept.count, p)
		}
	}
}

// /getyang adds to her yang, kept and told, up to exactly what a character may hold and
// never past it.
func TestGetYangAddsToHerYang(t *testing.T) {
	s, kept, told := tester(t, 100, nil)

	command(t, s, "getyang 5000")

	if kept.gold != 5100 || s.gold != 5100 {
		t.Fatalf("kept %d, holds %d", kept.gold, s.gold)
	}
	if p := told(); !holds(p, s.yangChanged(5000)) || len(p) != 1 {
		t.Fatalf("told %x", p)
	}
	command(t, s, "getyang 1999994900")
	if p := told(); s.gold != maxGold || !holds(p, s.yangChanged(1999994900)) {
		t.Fatalf("up to the most: holds %d, told %x", s.gold, p)
	}
	for line, want := range map[string]string{
		"getyang 1":  "Tyle się nie zmieści: postać może mieć najwyżej 2000000000 Yang.",
		"getyang":    "Użycie: /getyang ilość",
		"getyang -5": "Użycie: /getyang ilość",
		"getyang 0":  "Użycie: /getyang ilość",
	} {
		command(t, s, line)
		if p := told(); kept.gold != maxGold || !holds(p, infoLine(want)) {
			t.Errorf("/%s: kept %d, told %x", line, kept.gold, p)
		}
	}
}

// With the development switch off, as it is by default, both are ignored like any other
// command the server does not know.
func TestWithoutTheSwitchNothingIsGiven(t *testing.T) {
	s, kept, told := tester(t, 100, nil)
	s.svc.dev = false

	command(t, s, "getitem 10")
	command(t, s, "getyang 5000")

	if p := told(); len(kept.added) != 0 || kept.gold != 100 || len(p) != 0 {
		t.Fatalf("kept %+v and %d yang, told %x", kept.added, kept.gold, p)
	}
}

// The switch reaches the game port from what it is made with.
func TestTheSwitchComesFromTheDeps(t *testing.T) {
	if !NewService(Deps{DevCommands: true}).dev || NewService(Deps{}).dev {
		t.Fatal("the switch was not passed on")
	}
}
