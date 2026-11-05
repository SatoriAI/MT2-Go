package game

import (
	"context"
	"encoding/binary"
	"errors"
	"net"
	"testing"

	"github.com/SatoriAI/MT2-Go/internal/item"
	"github.com/SatoriAI/MT2-Go/internal/protocol"
)

// keptItems stands in for the store's items: it keeps the cells items were moved to, and
// fails every move with fail set.
type keptItems struct {
	Items // the rest is never called
	moved map[int64]uint16
	fail  error
}

func (k *keptItems) Move(_ context.Context, id int64, cell uint16) error {
	if k.fail != nil {
		return k.fail
	}
	k.moved[id] = cell
	return nil
}

const itemSetSize = 42

// carrying is a player in the world with a sword at the top of the inventory, two cells
// tall, and 50 potions beside it.
func carrying(t *testing.T) (*session, *keptItems, net.Conn) {
	t.Helper()
	conn, client := watcher(t)
	kept := &keptItems{moved: map[int64]uint16{}}
	s := &session{svc: &Service{items: kept}, conn: conn, phase: protocol.PhaseGame, inventory: item.Inventory{
		{ID: 1, VNum: 10, Count: 1, Cell: 0}, {ID: 2, VNum: 27001, Count: 50, Cell: 1}}}
	return s, kept, client
}

func dragged(from, to uint16, count uint8) []byte {
	body := binary.LittleEndian.AppendUint16([]byte{1}, from)
	body = binary.LittleEndian.AppendUint16(append(body, 1), to)
	return append(body, count)
}

// A drag to cells where all of the item fits moves it: it is kept there, and the client is
// told its old cell is empty and its new one holds it.
func TestAnItemDraggedWhereItFitsIsMovedAndKept(t *testing.T) {
	s, kept, client := carrying(t)

	if err := s.onItemMove(dragged(0, 17, 1)); err != nil {
		t.Fatal(err)
	}

	if got := read(t, client, 2*itemSetSize); string(got) != string(itemCleared(0))+string(
		itemSet(item.Item{ID: 1, VNum: 10, Count: 1, Cell: 17})) {
		t.Fatalf("told %x", got)
	}
	if kept.moved[1] != 17 || s.inventory[0].Cell != 17 {
		t.Fatalf("kept at %d, carried at %d", kept.moved[1], s.inventory[0].Cell)
	}
}

// What the game does not do yet is refused, and the client is told the item is where it
// was: a split stack, a drop onto another item, a cell the item would run off the page
// from, and another window.
func TestAMoveTheGameDoesNotDoIsRefusedAndTheItemShownWhereItWas(t *testing.T) {
	cases := []struct {
		name string
		body []byte
		back item.Item
	}{
		{"half the potions", dragged(1, 20, 25), item.Item{ID: 2, VNum: 27001, Count: 50, Cell: 1}},
		{"the potions onto the sword", dragged(1, 5, 50), item.Item{ID: 2, VNum: 27001, Count: 50, Cell: 1}},
		{"the sword off the page", dragged(0, 40, 1), item.Item{ID: 1, VNum: 10, Count: 1, Cell: 0}},
		{"the sword out of the inventory", []byte{1, 0, 0, 2, 0, 0, 1}, item.Item{ID: 1, VNum: 10, Count: 1, Cell: 0}},
	}
	for _, c := range cases {
		s, kept, client := carrying(t)

		if err := s.onItemMove(c.body); err != nil {
			t.Fatal(err)
		}

		if got := read(t, client, itemSetSize); string(got) != string(itemSet(c.back)) || len(kept.moved) != 0 {
			t.Errorf("%s: told %x, kept %v", c.name, got, kept.moved)
		}
	}
}

// A move that cannot be kept is not made: the client is told the item is where it was.
func TestAMoveThatCannotBeKeptIsNotMade(t *testing.T) {
	s, kept, client := carrying(t)
	kept.fail = errors.New("the database is gone")

	if err := s.onItemMove(dragged(0, 17, 1)); err != nil {
		t.Fatal(err)
	}

	if got := read(t, client, itemSetSize); string(got) != string(itemSet(s.inventory[0])) || s.inventory[0].Cell != 0 {
		t.Fatalf("told %x, carried at %d", got, s.inventory[0].Cell)
	}
}

// A drag from a cell with nothing in it changes nothing and is not answered.
func TestADragFromAnEmptyCellIsNotAnswered(t *testing.T) {
	s, kept, client := carrying(t)

	if err := s.onItemMove(dragged(30, 31, 1)); err != nil {
		t.Fatal(err)
	}

	expectNothing(t, client)
	if len(kept.moved) != 0 {
		t.Fatalf("kept %v", kept.moved)
	}
}
