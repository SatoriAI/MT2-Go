package game

import (
	"encoding/binary"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/SatoriAI/MT2-Go/internal/item"
	"github.com/SatoriAI/MT2-Go/internal/mob"
	"github.com/SatoriAI/MT2-Go/internal/network"
	"github.com/SatoriAI/MT2-Go/internal/protocol"
	"github.com/SatoriAI/MT2-Go/internal/world"
)

const groundAddSize, ownerSize, groundDelSize = 21, 30, 5

// hendrasPotion lays a red potion for Hendra (character 7) at the monsters' home, as the
// world loop would, so it is freed at noon and 20 s and cleared at noon and 60 s.
func hendrasPotion(s *Service, at *present) {
	potion := &groundItem{vid: 5000, x: homeX, y: homeY, vnum: 27001, count: 1, owner: owner{7, "Hendra"}, dropped: noon}
	s.presence.drop(at, []*groundItem{potion})
	s.freeing = append(s.freeing, timed{5000, noon.Add(ownedFor)})
	s.vanishing = append(s.vanishing, timed{5000, noon.Add(lieFor)})
}

// expectShown reads Hendra's potion coming into view, with her name over it.
func expectShown(t *testing.T, c net.Conn) {
	t.Helper()
	want := string(protocol.EncodeGroundAdd(5000, homeX, homeY, 27001)) + string(protocol.EncodeItemOwner(5000, "Hendra"))
	if got := read(t, c, len(want)); string(got) != want {
		t.Fatalf("got  %x\nwant %x", got, want)
	}
}

func expectGone(t *testing.T, c net.Conn) {
	t.Helper()
	if got := read(t, c, groundDelSize); string(got) != string(protocol.EncodeGroundDel(5000)) {
		t.Fatalf("got %x, want the potion gone", got)
	}
}

// A drop is kept for whoever took the most of the monster's hit points, so a last blow
// steals nothing; the killer has it on a tie, and so does the killer when the one who took
// more has left the world or lies dead.
func TestTheTopDamageDealerOwnsADrop(t *testing.T) {
	s := worldWith(t)
	_, hendra := aPlayer(t, s, 1000, 7, "Hendra", 20000)
	_, kira := aPlayer(t, s, 1001, 8, "Kira", 40000)
	cases := []struct {
		name    string
		hendras int
		setup   func()
		want    uint32
	}{
		{"Hendra took more", 50, func() {}, 7},
		{"a tie", 30, func() {}, 8},
		{"Hendra lies dead", 50, func() { hendra.char.HP = 0 }, 8},
		{"Hendra left", 50, func() { hendra.char.HP = 1; s.presence.leave(1000) }, 8},
	}
	for _, c := range cases {
		c.setup()
		f := &fight{damageBy: map[*present]int{hendra: c.hendras, kira: 30}}
		if got := s.presence.topDamager(f, kira); got.char.ID != c.want {
			t.Errorf("%s: kept for %d, want %d", c.name, got.char.ID, c.want)
		}
	}
}

// A monster that dies rolls each of its drops: one certain to fall lies near its body for
// its owner, shown with their name to a player beside it and to nobody far off; one that
// falls once in 10,000 does not, with this seed.
func TestADeathLeavesWhatItsDropsRoll(t *testing.T) {
	s := worldWith(t, world.Spawn{VNum: 101, X: homeX, Y: homeY, Count: 1})
	m := s.monsters[0]
	rolled := *m.proto
	rolled.Drops = []mob.Drop{{VNum: 27001, Count: 1, Chance: 10000}, {VNum: 27004, Count: 1, Chance: 1}}
	rolled.Yang = mob.Yang{}
	m.proto = &rolled
	beside, _ := aPlayer(t, s, 1000, 7, "Hendra", 100)
	read(t, beside, addSize) // the dog
	far, _ := aPlayer(t, s, 1001, 8, "Kira", 20000)

	s.dropFrom(m, death{owner: owner{7, "Hendra"}}, noon)

	if len(s.presence.ground) != 1 {
		t.Fatalf("%d items fell, want the potion alone", len(s.presence.ground))
	}
	for vid, it := range s.presence.ground {
		if it.vnum != 27001 || !near(it.x, it.y, m.x, m.y, 2*dropSpread) || s.freeing[0].at != noon.Add(ownedFor) ||
			s.vanishing[0].at != noon.Add(lieFor) {
			t.Fatalf("fell %+v, freed %v, cleared %v", it, s.freeing, s.vanishing)
		}
		want := string(protocol.EncodeGroundAdd(vid, it.x, it.y, 27001)) + string(protocol.EncodeItemOwner(vid, "Hendra"))
		if got := read(t, beside, groundAddSize+ownerSize); string(got) != want {
			t.Fatalf("got  %x\nwant %x", got, want)
		}
	}
	expectNothing(t, far)
}

// A player walking into view of an item on the ground is shown it, with its owner, and
// walking out of view takes it away.
func TestAnItemOnTheGroundComesAndGoesWithTheView(t *testing.T) {
	s := worldWith(t)
	client, p := aPlayer(t, s, 1000, 8, "Kira", 20000)
	hendrasPotion(s, p)

	s.presence.move(1000, protocol.Move{X: homeX + 100, Y: homeY})
	expectShown(t, client)
	s.presence.move(1000, protocol.Move{X: homeX + 200, Y: homeY})
	expectNothing(t, client)
	s.presence.move(1000, protocol.Move{X: homeX + 20000, Y: homeY})
	expectGone(t, client)
}

// A player entering is shown the items in view, and leaving takes them away again, so the
// client keeps none and is not shown them twice on coming back.
func TestEnteringShowsTheGroundAndLeavingTakesItAway(t *testing.T) {
	s := worldWith(t)
	_, far := aPlayer(t, s, 1001, 9, "Ola", 30000)
	hendrasPotion(s, far)
	client, _ := aPlayer(t, s, 1000, 8, "Kira", 100)
	expectShown(t, client)

	s.presence.leave(1000)

	expectGone(t, client)
}

// After 20 s anyone may take the item, which everyone near is told by its name cleared;
// after 60 s it is gone. One picked up before then is simply not there any more.
func TestADropIsFreedThenClearedAway(t *testing.T) {
	s := worldWith(t)
	client, p := aPlayer(t, s, 1000, 8, "Kira", 100)
	hendrasPotion(s, p)
	expectShown(t, client)

	s.keepGround(noon.Add(ownedFor - 1))
	expectNothing(t, client)
	s.keepGround(noon.Add(ownedFor))
	if got := read(t, client, ownerSize); string(got) != string(protocol.EncodeItemOwner(5000, "")) {
		t.Fatalf("got %x, want the owner cleared", got)
	}
	s.keepGround(noon.Add(lieFor))
	expectGone(t, client)
	hendrasPotion(s, p) // laid again, then picked up before its time
	expectShown(t, client)
	s.presence.lift(s.presence.ground[5000])
	s.keepGround(noon.Add(lieFor))
	expectNothing(t, client)
	if len(s.presence.ground) != 0 || len(s.freeing)+len(s.vanishing) != 0 {
		t.Fatalf("left %d on the ground, %d timers", len(s.presence.ground), len(s.freeing)+len(s.vanishing))
	}
}

// Who may pick the item up: its owner, beside it; nobody else before it is freed, and
// anyone after; nobody too far, nor dead, nor twice.
func TestOnlyTheOwnerPicksADropUpUntilItIsFreed(t *testing.T) {
	everywhere := func(*groundItem) bool { return true }
	s := worldWith(t)
	_, hendra := aPlayer(t, s, 1000, 7, "Hendra", 30000)
	aPlayer(t, s, 1001, 8, "Kira", 30100)
	hendrasPotion(s, hendra)
	s.presence.move(1001, protocol.Move{X: homeX + 250, Y: homeY})

	if _, got := s.presence.pickUp(1001, 5000, everywhere); got != pickRefused {
		t.Fatalf("Kira picked Hendra's potion up before it was freed: %v", got)
	}
	s.presence.move(1000, protocol.Move{X: homeX + 400, Y: homeY})
	if _, got := s.presence.pickUp(1000, 5000, everywhere); got != pickRefused {
		t.Fatalf("Hendra picked it up from 400 away: %v", got)
	}
	s.presence.move(1000, protocol.Move{X: homeX + 300, Y: homeY})
	hendra.char.HP = 0
	if _, got := s.presence.pickUp(1000, 5000, everywhere); got != pickRefused {
		t.Fatalf("Hendra picked it up dead: %v", got)
	}
	hendra.char.HP = 1
	if it, got := s.presence.pickUp(1000, 5000, everywhere); got != picked || it.vnum != 27001 {
		t.Fatalf("Hendra, beside it, got %v", got)
	}
	if _, got := s.presence.pickUp(1000, 5000, everywhere); got != pickRefused {
		t.Fatalf("picked up twice: %v", got)
	}

	hendrasPotion(s, hendra)
	s.keepGround(noon.Add(ownedFor))
	if _, got := s.presence.pickUp(1001, 5000, everywhere); got != picked {
		t.Fatalf("Kira could not pick the freed potion up: %v", got)
	}
}

// An item with no room for it stays on the ground.
func TestAnItemWithNoRoomStaysOnTheGround(t *testing.T) {
	s := worldWith(t)
	_, hendra := aPlayer(t, s, 1000, 7, "Hendra", 100)
	hendrasPotion(s, hendra)

	if _, got := s.presence.pickUp(1000, 5000, func(*groundItem) bool { return false }); got != pickNoRoom {
		t.Fatalf("got %v", got)
	}
	if s.presence.ground[5000] == nil {
		t.Fatal("the potion left the ground")
	}
}

// pickingUp is Hendra beside her potion on the ground, carrying a stack of potions.
func pickingUp(t *testing.T, stack uint8) (*session, *keptItems, net.Conn) {
	t.Helper()
	s := worldWith(t)
	kept := &keptItems{moved: map[int64]uint16{}, worn: map[int64]item.Wear{}, count: map[int64]uint8{}}
	s.items = kept
	client, hendra := aPlayer(t, s, 1000, 7, "Hendra", 100)
	hendrasPotion(s, hendra)
	expectShown(t, client)
	c := playingAs(hendra)
	s.now = func() time.Time { return noon.Add(time.Second) }
	sess := &session{svc: s, conn: hendra.out.(*network.Conn), phase: protocol.PhaseGame, playing: c,
		inventory: item.Inventory{{ID: 1, VNum: 27001, Count: stack, Cell: 0}}}
	return sess, kept, client
}

// thePotion is a pick-up of Hendra's potion, as the client sends it.
var thePotion = binary.LittleEndian.AppendUint32(nil, 5000)

// A potion picked up joins her stack, which is kept at its new count and shown.
func TestAPotionPickedUpJoinsHerStack(t *testing.T) {
	s, kept, client := pickingUp(t, 5)

	if err := s.onItemPickup(thePotion); err != nil {
		t.Fatal(err)
	}

	expectGone(t, client)
	if got := read(t, client, itemSetSize); string(got) != string(itemSet(item.Item{ID: 1, VNum: 27001, Count: 6})) ||
		kept.count[1] != 6 || s.inventory[0].Count != 6 {
		t.Fatalf("told %x, kept %v, carries %+v", got, kept.count, s.inventory)
	}
}

// With her stack full, the potion is a new item in the first free cell.
func TestAPotionWithItsStackFullTakesANewCell(t *testing.T) {
	s, kept, client := pickingUp(t, item.MaxStack)

	if err := s.onItemPickup(thePotion); err != nil {
		t.Fatal(err)
	}

	expectGone(t, client)
	want := item.Item{ID: 101, VNum: 27001, Count: 1, Cell: 1}
	if got := read(t, client, itemSetSize); string(got) != string(itemSet(want)) || len(kept.added) != 1 ||
		len(s.inventory) != 2 || s.inventory[1] != want {
		t.Fatalf("told %x, kept %+v, carries %+v", got, kept.added, s.inventory)
	}
}

// With no room at all she is told so, and the potion stays on the ground.
func TestAPotionWithNoRoomIsLeftAndSheIsTold(t *testing.T) {
	s, _, client := pickingUp(t, item.MaxStack)
	for cell := range uint16(item.Cells) {
		s.inventory = append(s.inventory, item.Item{ID: int64(cell) + 10, VNum: 10, Count: 1, Cell: cell})
	}

	if err := s.onItemPickup(thePotion); err != nil {
		t.Fatal(err)
	}

	line := textToClient("Nie masz miejsca w ekwipunku.")
	if got := read(t, client, chatHeaderSize+len(line)+1); string(got) != string(protocol.EncodeChat(protocol.ChatInfo, 0, 0, line)) ||
		s.svc.presence.ground[5000] == nil {
		t.Fatalf("told %x", got)
	}
}

// A potion that cannot be kept goes back on the ground, still hers.
func TestAPotionThatCannotBeKeptGoesBack(t *testing.T) {
	s, kept, client := pickingUp(t, 5)
	kept.fail = errors.New("the database is gone")

	if err := s.onItemPickup(thePotion); err != nil {
		t.Fatal(err)
	}

	expectGone(t, client)
	expectShown(t, client)
	if s.svc.presence.ground[5000] == nil || s.inventory[0].Count != 5 {
		t.Fatalf("on the ground: %v; carries %+v", s.svc.presence.ground[5000] != nil, s.inventory)
	}
}

// A potion whose write gives up only after its minute is over is not put back: its time to
// be cleared away has passed, and nothing would clear it. One whose owner's time ran out
// meanwhile goes back free.
func TestAPotionPutBackAfterItsTimeIsGoneOrFree(t *testing.T) {
	s, kept, client := pickingUp(t, 5)
	kept.fail = errors.New("the database hung")
	s.svc.now = func() time.Time { return noon.Add(lieFor) }

	if err := s.onItemPickup(thePotion); err != nil {
		t.Fatal(err)
	}

	expectGone(t, client)
	expectNothing(t, client)
	if s.svc.presence.ground[5000] != nil {
		t.Fatal("put back after its minute")
	}

	s, kept, client = pickingUp(t, 5)
	kept.fail = errors.New("the database hung")
	s.svc.now = func() time.Time { return noon.Add(ownedFor) }
	if err := s.onItemPickup(thePotion); err != nil {
		t.Fatal(err)
	}
	expectGone(t, client)
	want := string(protocol.EncodeGroundAdd(5000, homeX, homeY, 27001))
	if got := read(t, client, len(want)); string(got) != want || s.svc.presence.ground[5000].owner != (owner{}) {
		t.Fatalf("got %x, put back owned by %+v", got, s.svc.presence.ground[5000].owner)
	}
}

// An item cleared away is gone for good: a player arriving beside where it lay is shown
// nothing of it.
func TestAnItemClearedAwayIsShownToNobodyArriving(t *testing.T) {
	s := worldWith(t)
	_, hendra := aPlayer(t, s, 1000, 7, "Hendra", 100)
	hendrasPotion(s, hendra)
	s.presence.vanish(5000)

	kira, _ := aPlayer(t, s, 1001, 8, "Kira", 200)

	read(t, kira, addSize+infoSize) // Hendra
	expectNothing(t, kira)
}
