package store

import (
	"context"
	"testing"

	"github.com/SatoriAI/MT2-Go/internal/item"
)

// hendraAtTheSmith is Hendra with gold yang, a sword +4 at cell 0, and two stacks of wolf
// claws, one at cell 5 and two at cell 6.
func hendraAtTheSmith(t *testing.T, gold uint32) (Items, uint32, [3]int64) {
	t.Helper()
	s, cid := hendrasItems(t)
	ctx := context.Background()
	if _, err := s.AddGold(ctx, cid, gold); err != nil {
		t.Fatal(err)
	}
	var ids [3]int64
	for i, it := range []item.Item{{VNum: 14, Count: 1, Cell: 0}, {VNum: 30028, Count: 1, Cell: 5}, {VNum: 30028, Count: 2, Cell: 6}} {
		id, err := s.Add(ctx, cid, it)
		if err != nil {
			t.Fatal(err)
		}
		ids[i] = id
	}
	return s, cid, ids
}

// An upgrade takes the yang, keeps the sword's new kind, and keeps the claws spent: the
// stack used up is forgotten, and the other kept at what is left.
func TestAnUpgradeIsKeptWhole(t *testing.T) {
	s, cid, ids := hendraAtTheSmith(t, 5000)

	gold, err := s.Upgrade(context.Background(), cid, 3000, item.Item{ID: ids[0], VNum: 15, Count: 1, Cell: 0},
		[]item.Item{{ID: ids[1], VNum: 30028, Count: 0, Cell: 5}, {ID: ids[2], VNum: 30028, Count: 1, Cell: 6}})

	if err != nil || gold != 2000 {
		t.Fatalf("left %d, %v", gold, err)
	}
	inv, kept := s.mustOwn(t)
	sword, _ := inv.ByID(ids[0])
	claws, _ := inv.ByID(ids[2])
	if _, gone := inv.ByID(ids[1]); kept != 2000 || len(inv) != 2 || sword.VNum != 15 || claws.Count != 1 || gone {
		t.Fatalf("owns %+v with %d yang", inv, kept)
	}
}

// An upgrade the character cannot pay for is refused, and nothing changes.
func TestAnUpgradeTheYangCannotCoverChangesNothing(t *testing.T) {
	s, cid, ids := hendraAtTheSmith(t, 2999)

	if _, err := s.Upgrade(context.Background(), cid, 3000, item.Item{ID: ids[0], VNum: 15, Count: 1, Cell: 0},
		[]item.Item{{ID: ids[1], VNum: 30028, Count: 0, Cell: 5}}); err == nil {
		t.Fatal("upgraded")
	}

	inv, kept := s.mustOwn(t)
	sword, _ := inv.ByID(ids[0])
	if kept != 2999 || len(inv) != 3 || sword.VNum != 14 {
		t.Fatalf("owns %+v with %d yang", inv, kept)
	}
}
