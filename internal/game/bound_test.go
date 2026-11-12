package game

import (
	"testing"

	"github.com/SatoriAI/MT2-Go/internal/item"
	"github.com/SatoriAI/MT2-Go/internal/protocol"
)

// The thief's gloves, bound to Hendra, are not dropped: she is told so, and nothing lies.
func TestABoundItemIsNotDropped(t *testing.T) {
	s, kept, client, _ := dropping(t)
	s.inventory = append(s.inventory, item.Item{ID: 3, VNum: item.ThiefGloves, Count: 1, Cell: 4})

	if err := s.onItemDrop(dropped(1, 4, 0, 1)); err != nil {
		t.Fatal(err)
	}

	if got := packets(t, drain(t, client)); len(got) != 1 || !holds(got, infoLine("Nie możesz upuścić tego przedmiotu.")) {
		t.Fatalf("told %x", got)
	}
	if len(s.svc.record.state.ground) != 0 || len(kept.gone) != 0 || len(s.inventory) != 3 {
		t.Fatal("dropped")
	}
}

// Nor offered in a trade: she is told in the client's words, and Kira sees nothing.
func TestABoundItemIsNotOfferedInATrade(t *testing.T) {
	_, hendra, kira := started(t)
	hendra.s.inventory = append(hendra.s.inventory, item.Item{ID: 4, VNum: item.TeleportRing, Count: 1, Cell: 1})

	hendra.do(t, exchanging(protocol.ExchangeAdd, 0, 0, 1))

	if got := hendra.told(t); len(got) != 1 || !holds(got, infoLine("Nie możesz handlować tym przedmiotem.")) {
		t.Fatalf("told %x", got)
	}
	if got := kira.told(t); len(got) != 0 {
		t.Fatalf("Kira told %x", got)
	}
	if offer := hendra.s.svc.record.state.trades[1000].sides[0].offer; len(offer) != 0 {
		t.Fatalf("offered %v", offer)
	}
}

// Nor sold to a shop: she is told in the client's words, and keeps her capes and yang.
func TestABoundItemIsNotSold(t *testing.T) {
	s, kept, client, store := shopping(t, 100, 50, item.Inventory{{ID: 9, VNum: item.BraveryCape, Count: 5, Cell: 0}})
	open(t, s, client, store)

	if err := s.onShop(sold(0, 1)); err != nil {
		t.Fatal(err)
	}

	if got := packets(t, drain(t, client)); len(got) != 1 || !holds(got, infoLine("Ten Przedmiot nie może zostać sprzedany.")) {
		t.Fatalf("told %x", got)
	}
	if kept.gold != 50 || s.inventory[0].Count != 5 {
		t.Fatalf("has %d yang, %d capes", kept.gold, s.inventory[0].Count)
	}
}
