package game

import (
	"encoding/binary"
	"net"
	"testing"
	"time"

	"github.com/SatoriAI/MT2-Go/internal/account"
	"github.com/SatoriAI/MT2-Go/internal/item"
	"github.com/SatoriAI/MT2-Go/internal/protocol"
)

const updateSize, infoSize, chatHeaderSize = 35, 54, 9

// dressing is Hendra, a level-1 sura with vitality 3, in the world with Kira beside her,
// carrying a sword at the top of her inventory (cells 0 and 5), the sura's armour beside
// it (1 and 6), a second sword beside that (2 and 7), and a shaman's fan at 3.
func dressing(t *testing.T) (*session, *keptItems, net.Conn, net.Conn) {
	t.Helper()
	svc := worldWith(t)
	kept := &keptItems{moved: map[int64]uint16{}, worn: map[int64]item.Wear{}, count: map[int64]uint8{}}
	svc.items = kept
	conn, client := watcher(t)
	hendra := &character{vid: 1000, Character: account.Character{Name: "Hendra", Class: account.Sura, Level: 1, HP: 1,
		Stats: account.Stats{Vitality: 3}}}
	svc.presence.enter(&present{out: conn, char: &hendra.Character, info: protocol.CharacterInfo{VID: 1000, Name: "Hendra"},
		add: protocol.CharacterAdd{VID: 1000, X: homeX, Y: homeY, Type: protocol.EntityPlayer, MoveSpeed: 150, AttackSpeed: 150}})
	viewerConn, viewer := watcher(t)
	svc.presence.enter(&present{out: viewerConn, char: &account.Character{Name: "Kira", HP: 1},
		info: protocol.CharacterInfo{VID: 1001, Name: "Kira"},
		add:  protocol.CharacterAdd{VID: 1001, X: homeX + 100, Y: homeY, Type: protocol.EntityPlayer}})
	read(t, client, addSize+infoSize) // each is shown the other
	read(t, viewer, addSize+infoSize)
	s := &session{svc: svc, conn: conn, phase: protocol.PhaseGame, playing: hendra, inventory: item.Inventory{
		{ID: 1, VNum: 10, Count: 1, Cell: 0}, {ID: 2, VNum: 11600, Count: 1, Cell: 1},
		{ID: 3, VNum: 10, Count: 1, Cell: 2}, {ID: 4, VNum: 7000, Count: 1, Cell: 3}}}
	return s, kept, client, viewer
}

func doubleClicked(cell uint16) []byte { return binary.LittleEndian.AppendUint16([]byte{1}, cell) }

// expectDressed reads a CHARACTER_UPDATE for Hendra in the armour and weapon given.
func expectDressed(t *testing.T, c net.Conn, body, weapon uint16) {
	t.Helper()
	want := protocol.EncodeCharacterUpdate(protocol.CharacterInfo{VID: 1000, Name: "Hendra", BodyPart: body,
		WeaponPart: weapon}, protocol.CharacterAdd{MoveSpeed: 150, AttackSpeed: 150}, 0)
	if got := read(t, c, updateSize); string(got) != string(want) {
		t.Fatalf("got  %x\nwant %x", got, want)
	}
}

// A double-click on the sword puts it on: its cells empty, it is shown worn, and she and
// Kira see it in her hand; her blows use it from then on.
func TestADoubleClickPutsAWeaponOn(t *testing.T) {
	s, kept, client, viewer := dressing(t)

	if err := s.onItemUse(doubleClicked(0)); err != nil {
		t.Fatal(err)
	}

	sword := item.Item{ID: 1, VNum: 10, Count: 1}
	if got := read(t, client, 2*itemSetSize); string(got) != string(itemCleared(sword))+
		string(itemSet(item.Item{ID: 1, VNum: 10, Count: 1, Worn: item.Weapon})) {
		t.Fatalf("told %x", got)
	}
	expectDressed(t, client, 0, 10)
	expectDressed(t, viewer, 0, 10)
	if kept.worn[1] != item.Weapon || s.playing.Weapon != 10 {
		t.Fatalf("kept %q, holding %d", kept.worn[1], s.playing.Weapon)
	}
}

// Dragging the armour onto the body's place puts it on, and her DEF rises by its 12, to
// her level, vitality and armour: 1 + 3 + 12.
func TestArmourDraggedOntoItsPlaceIsPutOn(t *testing.T) {
	s, kept, client, viewer := dressing(t)

	if err := s.onItemMove([]byte{1, 1, 0, 1, 90, 0, 1}); err != nil {
		t.Fatal(err)
	}

	read(t, client, 2*itemSetSize)
	expectDressed(t, client, 11600, 0)
	if point, _, value := pointChange(t, client, 1000); point != protocol.PointDefence || value != 16 {
		t.Fatalf("told point %d at %d, want DEF 16", point, value)
	}
	expectDressed(t, viewer, 11600, 0)
	if kept.worn[2] != item.Body || s.playing.Body != 11600 {
		t.Fatalf("kept %q, wearing %d", kept.worn[2], s.playing.Body)
	}
}

// Putting on the second sword while the first is worn swaps them: the first goes into the
// cells the second leaves.
func TestPuttingOnWhereAnotherIsWornSwapsThem(t *testing.T) {
	s, kept, client, _ := dressing(t)
	if err := s.onItemUse(doubleClicked(0)); err != nil {
		t.Fatal(err)
	}
	read(t, client, 2*itemSetSize+updateSize)

	if err := s.onItemUse(doubleClicked(2)); err != nil {
		t.Fatal(err)
	}

	first, second := item.Item{ID: 1, VNum: 10, Count: 1}, item.Item{ID: 3, VNum: 10, Count: 1, Cell: 2}
	firstWorn, secondWorn := first, second
	firstWorn.Worn, secondWorn.Worn = item.Weapon, item.Weapon
	first.Cell = 2
	want := string(itemCleared(second)) + string(itemCleared(firstWorn)) + string(itemSet(secondWorn)) +
		string(itemSet(first))
	if got := read(t, client, 4*itemSetSize); string(got) != want { // both places emptied, then both filled
		t.Fatalf("told %x\nwant %x", got, want)
	}
	if kept.worn[3] != item.Weapon || kept.worn[1] != "" || kept.moved[1] != 2 {
		t.Fatalf("kept the second as %q, the first as %q at %d", kept.worn[3], kept.worn[1], kept.moved[1])
	}
	if first, _ := s.inventory.ByID(1); first.Worn != "" || first.Cell != 2 {
		t.Fatalf("carries the first as %+v", first)
	}
}

// What is worn comes off by a double-click, into the first cells it fits, or by a drag,
// into the cells it is dropped on.
func TestAWornItemComesOffByDoubleClickOrDrag(t *testing.T) {
	cases := []struct {
		name string
		take func(*session) error
		cell uint16
	}{
		{"a double-click", func(s *session) error { return s.onItemUse(doubleClicked(94)) }, 0},
		{"a drag to cell 20", func(s *session) error { return s.onItemMove([]byte{1, 94, 0, 1, 20, 0, 1}) }, 20},
	}
	for _, c := range cases {
		s, kept, client, viewer := dressing(t)
		if err := s.onItemUse(doubleClicked(0)); err != nil {
			t.Fatal(err)
		}
		read(t, client, 2*itemSetSize+updateSize)
		read(t, viewer, updateSize)

		if err := c.take(s); err != nil {
			t.Fatal(err)
		}

		read(t, client, 2*itemSetSize)
		expectDressed(t, client, 0, 0)
		expectDressed(t, viewer, 0, 0)
		if kept.worn[1] != "" || kept.moved[1] != c.cell || s.playing.Weapon != 0 {
			t.Errorf("%s: kept %q at %d, holding %d", c.name, kept.worn[1], kept.moved[1], s.playing.Weapon)
		}
	}
}

// A sura may not wear a shaman's fan: she is told why in chat, and the fan stays where it
// was.
func TestAKindForAnotherClassIsNotPutOn(t *testing.T) {
	s, kept, client, _ := dressing(t)

	if err := s.onItemUse(doubleClicked(3)); err != nil {
		t.Fatal(err)
	}

	line := textToClient(cannotWear[item.ErrWrongClass])
	if got := read(t, client, chatHeaderSize+len(line)+1); string(got) != string(protocol.EncodeChat(protocol.ChatInfo, 0, 0, line)) {
		t.Fatalf("told %x", got)
	}
	if got := read(t, client, itemSetSize); string(got) != string(itemSet(item.Item{ID: 4, VNum: 7000, Count: 1, Cell: 3})) ||
		len(kept.worn) != 0 {
		t.Fatalf("told %x, kept %v", got, kept.worn)
	}
}

// A sword dragged onto the body's place is not put on, and is shown back where it was;
// nor is a worn sword taken off when there is no room for it.
func TestAnItemIsNotWornOutOfItsPlaceNorTakenOffWithoutRoom(t *testing.T) {
	s, kept, client, _ := dressing(t)
	if err := s.onItemMove([]byte{1, 0, 0, 1, 90, 0, 1}); err != nil {
		t.Fatal(err)
	}
	if got := read(t, client, itemSetSize); string(got) != string(itemSet(s.inventory[0])) || len(kept.worn) != 0 {
		t.Fatalf("told %x, kept %v", got, kept.worn)
	}

	sword := item.Item{ID: 1, VNum: 10, Count: 1, Worn: item.Weapon}
	s.inventory = item.Inventory{sword}
	for cell := range uint16(item.Cells) {
		s.inventory = append(s.inventory, item.Item{ID: int64(cell) + 10, VNum: 27001, Count: 1, Cell: cell})
	}
	if err := s.onItemUse(doubleClicked(94)); err != nil {
		t.Fatal(err)
	}
	if got := read(t, client, itemSetSize); string(got) != string(itemSet(sword)) || len(kept.worn) != 0 {
		t.Fatalf("told %x, kept %v", got, kept.worn)
	}
}

// A sura dressed in the sword hits a wild dog for 31 to 42, never the 11 or 12 of her bare
// hands, and in the armour its 12 takes bites down to 24 to 28.
func TestWhatIsPutOnCountsInTheFight(t *testing.T) {
	s, dog, client := aDog(t, homeX+100)
	c := &account.Character{Level: 1, Class: account.Sura, HP: 1000,
		Stats: account.Stats{Strength: 5, Vitality: 3, Dexterity: 3, Intelligence: 5}}
	s.presence.all[1000].char = c
	s.presence.dress(1000, outfit{weapon: 10, body: 11600})
	read(t, client, updateSize+pointChangeSize) // her new look, and DEF

	s.presence.attack(1000, dog.p.add.VID, 0, someRand, noon)

	d := protocol.NewReader(read(t, client, damageSize)[1:])
	if _, _, damage := d.U32(), d.U8(), d.I32(); damage < 31 || damage > 42 {
		t.Fatalf("a blow with the sword took %d, want 31 to 42", damage)
	}
	s.tick(noon.Add(time.Second))
	read(t, client, moveSize)
	b := protocol.NewReader(read(t, client, damageSize)[1:])
	if _, _, damage := b.U32(), b.U8(), b.I32(); damage < 24 || damage > 28 {
		t.Fatalf("a bite through the armour took %d, want 24 to 28", damage)
	}
}

// A sura's helmet double-clicked goes on her head: shown worn at the head's place, and her
// DEF rises by its 5, to 1 + 3 + 5. Kira sees no change of look, but is told she is
// dressed again.
func TestAHelmetIsPutOnAndAddsItsDefence(t *testing.T) {
	s, kept, client, viewer := dressing(t)
	s.inventory = append(s.inventory, item.Item{ID: 5, VNum: 12480, Count: 1, Cell: 4})

	if err := s.onItemUse(doubleClicked(4)); err != nil {
		t.Fatal(err)
	}

	if got := read(t, client, 2*itemSetSize); string(got[itemSetSize:]) != string(protocol.EncodeItemSet(protocol.ItemPos{Window: 1, Cell: 91}, 12480, 1)) {
		t.Fatalf("told %x", got)
	}
	expectDressed(t, client, 0, 0)
	if point, _, value := pointChange(t, client, 1000); point != protocol.PointDefence || value != 9 {
		t.Fatalf("told point %d at %d, want DEF 9", point, value)
	}
	expectDressed(t, viewer, 0, 0)
	if kept.worn[5] != item.Head || s.playing.Head != 12480 {
		t.Fatalf("kept %q, wearing %d", kept.worn[5], s.playing.Head)
	}
}

// Shoes +6 go on her feet: her DEF rises by their 1, and she moves 9 faster, at 159, told
// to her and shown to Kira; taken off, she is back at 150.
func TestShoesAreFasterByTheirGrade(t *testing.T) {
	s, _, client, viewer := dressing(t)
	s.inventory = append(s.inventory, item.Item{ID: 5, VNum: 15006, Count: 1, Cell: 4})
	faster := protocol.EncodeCharacterUpdate(protocol.CharacterInfo{VID: 1000, Name: "Hendra"},
		protocol.CharacterAdd{MoveSpeed: 159, AttackSpeed: 150}, 0)

	if err := s.onItemUse(doubleClicked(4)); err != nil {
		t.Fatal(err)
	}

	if got := read(t, client, 2*itemSetSize); string(got[itemSetSize:]) != string(protocol.EncodeItemSet(protocol.ItemPos{Window: 1, Cell: 92}, 15006, 1)) {
		t.Fatalf("told %x", got)
	}
	if got := read(t, client, updateSize); string(got) != string(faster) {
		t.Fatalf("got %x", got)
	}
	if point, _, value := pointChange(t, client, 1000); point != protocol.PointDefence || value != 5 {
		t.Fatalf("told point %d at %d, want DEF 5", point, value)
	}
	if point, _, value := pointChange(t, client, 1000); point != protocol.PointMoveSpeed || value != 159 {
		t.Fatalf("told point %d at %d, want speed 159", point, value)
	}
	if got := read(t, viewer, updateSize); string(got) != string(faster) {
		t.Fatalf("Kira got %x", got)
	}
	if shown := s.svc.presence.all[1000].add.MoveSpeed; shown != 159 { // what anyone arriving is shown
		t.Fatalf("shown at %d", shown)
	}

	if err := s.onItemUse(doubleClicked(92)); err != nil {
		t.Fatal(err)
	}

	read(t, client, 2*itemSetSize+updateSize+pointChangeSize)
	if point, _, value := pointChange(t, client, 1000); point != protocol.PointMoveSpeed || value != 150 {
		t.Fatalf("told point %d at %d, want speed 150", point, value)
	}
}

// Shoes, a helmet +3 and armour +9 all count against a bite: 1 + 17 + 66.
func TestEverythingWornCountsAgainstABite(t *testing.T) {
	c := &account.Character{Body: 11609, Head: 12483, Feet: 15000}
	if got := armourOf(c); got != 84 {
		t.Fatalf("armour %d, want 84", got)
	}
}

// A sura entering in shoes +9 moves at 165, in her points.
func TestShoesWornOnEnteringSetHerSpeed(t *testing.T) {
	p := &character{Character: account.Character{Class: account.Sura, Level: 1, Feet: 15009}}
	if got := pointsOf(p, 0).MoveSpeed; got != 165 || moveSpeed(&p.Character) != 165 {
		t.Fatalf("speed %d", got)
	}
}
