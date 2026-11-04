package server

import (
	"slices"
	"testing"

	"github.com/SatoriAI/MT2-Go/internal/protocol"
	"github.com/SatoriAI/MT2-Go/internal/world"
)

// Near the Jinno start: the weapon dealer and a wild dog in view, a wolf well out of it.
// None is spread, so each stands where written.
var nearTheStart = []world.Spawn{
	{VNum: 9001, X: startX + 1000, Y: startY, Count: 1, Facing: 90},
	{VNum: 101, X: startX - 1000, Y: startY, Count: 1},
	{VNum: 102, X: startX + 20000, Y: startY, Count: 1},
}

// packetSizes are the packets about characters in view, by header.
var packetSizes = map[byte]int{0x01: 35, 0x02: 5, 0x03: 24, 0x88: 54}

// nextThree reads the next three packets about characters in view, whole, in the order
// they came: what each step of these tests changes.
func (c *client) nextThree() [][]byte {
	c.t.Helper()
	var got [][]byte
	for range 3 {
		header := c.read(1)
		size, ok := packetSizes[header[0]]
		if !ok {
			c.t.Fatalf("unexpected packet %02x", header[0])
		}
		got = append(got, append(header, c.read(size-1)...))
	}
	return got
}

// mobsShown sorts what a player was shown of mobs into each add, by race, and the infos.
func mobsShown(packets [][]byte) (adds map[uint16]protocol.CharacterAdd, infos []string) {
	adds = map[uint16]protocol.CharacterAdd{}
	for _, p := range packets {
		r := protocol.NewReader(p[1:])
		switch p[0] {
		case 0x01:
			a := protocol.CharacterAdd{VID: r.U32()}
			r.Skip(4) // rotation
			a.X, a.Y = r.I32(), r.I32()
			r.Skip(4) // height
			a.Type, a.Race, a.MoveSpeed, a.AttackSpeed = r.U8(), r.U16(), r.U8(), r.U8()
			adds[a.Race] = a
		case 0x88:
			r.Skip(4)
			infos = append(infos, r.String(25))
		}
	}
	return adds, infos
}

// A player entering sees the mobs in view: an NPC with its name, a monster by its add
// alone, which the client names from its own data. One out of view is not shown.
func TestAPlayerEnteringSeesTheMobsInView(t *testing.T) {
	h := startWith(t, nearTheStart)
	hendra, _ := h.enterWorld("admin", "Hendra", 3, 6)

	adds, infos := mobsShown(hendra.nextThree()) // the dealer's add and info, the dog's add

	dealer, dog := adds[9001], adds[101]
	if dealer.Type != protocol.EntityNPC || dealer.X != startX+1000 || dealer.Y != startY || dealer.MoveSpeed != 100 {
		t.Errorf("the dealer: %+v", dealer)
	}
	if dog.Type != protocol.EntityMonster || dog.X != startX-1000 || dog.MoveSpeed != 100 || dog.AttackSpeed != 100 {
		t.Errorf("the dog: %+v", dog)
	}
	if want := []string{"Handlarz Broni\xb9"}; !slices.Equal(infos, want) { // "Handlarz Bronią" in cp1250
		t.Errorf("infos %q, want %q", infos, want)
	}
	hendra.expectOpen() // and nothing of the wolf
}

// Walking off, the mobs left behind vanish and the ones reached appear.
func TestWalkingToOtherMobsSwapsWhichAreShown(t *testing.T) {
	h := startWith(t, nearTheStart)
	hendra, _ := h.enterWorld("admin", "Hendra", 3, 6)
	hendra.nextThree()

	hendra.send(moveTo(startX+19000, startY))

	got := hendra.nextThree()
	adds, _ := mobsShown(got)
	if wolf, ok := adds[102]; !ok || wolf.Type != protocol.EntityMonster || len(adds) != 1 {
		t.Errorf("shown %+v, want the wolf alone", adds)
	}
	deletes := 0
	for _, p := range got {
		if p[0] == 0x02 {
			deletes++
		}
	}
	if deletes != 2 {
		t.Errorf("%d deletes, want the dealer's and the dog's", deletes)
	}
	hendra.expectOpen()
}

// An NPC's name is not a player's: a whisper to it finds nobody.
func TestAWhisperToAnNPCFindsNobody(t *testing.T) {
	h := startWith(t, nearTheStart)
	hendra, _ := h.enterWorld("admin", "Hendra", 3, 6)
	hendra.nextThree()

	hendra.whisper("Handlarz Broni\xb9", "hi")

	hendra.expect(protocol.EncodeWhisper(protocol.WhisperNotOnline, "Handlarz Broni\xb9", ""))
}
