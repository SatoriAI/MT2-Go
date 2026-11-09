package server

import (
	"bytes"
	"encoding/binary"
	"net/netip"
	"testing"
	"time"

	"github.com/SatoriAI/MT2-Go/internal/protocol"
	"github.com/SatoriAI/MT2-Go/internal/world"
)

// A wild dog beside the Jinno start; it is the first in the world, so its vid is 1.
var aDogAtTheStart = []world.Spawn{{VNum: 101, X: startX + 100, Y: startY, Count: 1}}

const dogVID = 1

// target picks a character as the client does: header, vid, sequence byte.
func (c *client) target(vid uint32) {
	c.t.Helper()
	c.send(binary.LittleEndian.AppendUint32([]byte{byte(protocol.ClientTarget)}, vid), []byte{sequenceByte})
}

// hitTheDog lands a plain blow on the dog as the client does: header, skill 0, victim, two
// check bytes, and the sequence byte.
func (c *client) hitTheDog() {
	c.t.Helper()
	c.send(binary.LittleEndian.AppendUint32([]byte{byte(protocol.ClientAttack), 0}, dogVID), []byte{0x5A, 0xA5, sequenceByte})
}

// moveSize is a MOVE's; damageSize and pointChangeSize the two packets a bite costs.
const moveSize, damageSize, pointChangeSize = 24, 10, 17

// nextPastBites reads the next packet, of n bytes, passing over what the dog does
// meanwhile: its moves, and its bites with the damage and hit points each costs Hendra.
// The tests that use it are about her blows, not its.
func (c *client) nextPastBites(n int) []byte {
	c.t.Helper()
	for {
		header := c.read(1)
		if header[0] != byte(protocol.ServerMove) {
			return append(header, c.read(n-1)...)
		}
		if move := c.read(moveSize - 1); move[0] == protocol.MoveAttack {
			c.read(damageSize + pointChangeSize)
		}
	}
}

// expectOpenPastBites is expectOpen, passing over what the dog does meanwhile.
func (c *client) expectOpenPastBites() {
	c.t.Helper()
	port := netip.MustParseAddrPort(c.conn.RemoteAddr().String()).Port()
	c.send([]byte{byte(protocol.ClientStateChecker)})
	want := protocol.EncodeChannelStatus([]protocol.ChannelStatus{{Port: port, Status: protocol.ChannelNormal}})
	if got := c.nextPastBites(len(want)); !bytes.Equal(got, want) {
		c.t.Fatalf("got %x, want %x", got, want)
	}
}

// expectDamageOnTheDog reads the damage of a blow on the dog and returns it.
func (c *client) expectDamageOnTheDog() int32 {
	c.t.Helper()
	r := protocol.NewReader(c.nextPastBites(damageSize))
	if header, victim, flags, damage := r.U8(), r.U32(), r.U8(), r.I32(); header != byte(protocol.ServerDamage) ||
		victim != dogVID || flags != protocol.DamageNormal {
		c.t.Fatalf("got damage packet %02x on %d, flags %d", header, victim, flags)
	} else {
		return damage
	}
	return 0
}

// Hendra, a level-1 sura bare-handed, picks the dog and hits it: an attack of 12, of which
// the dog's defence of 10 takes a share, 12 × 100 / 110, give or take a tenth; its bar
// drops by as much.
func TestHittingAPickedMonsterShowsTheDamageAndItsBar(t *testing.T) {
	h := startWith(t, aDogAtTheStart)
	hendra, _ := h.enterWorld("admin", "Hendra", 3, 6)
	hendra.read(35) // the dog

	hendra.target(dogVID)
	hendra.expect(protocol.EncodeTarget(dogVID, 100))
	hendra.hitTheDog()

	damage := hendra.expectDamageOnTheDog()
	if damage < 10 || damage > 12 {
		t.Fatalf("a bare-handed sura did %d to a wild dog, want 10 to 12", damage)
	}
	hendra.expect(protocol.EncodeTarget(dogVID, uint8((126*100-int(damage)*100+63)/126))) //nolint:gosec // 0 to 100
}

// A second blow on the same victim sooner than the attack speed allows is dropped; one
// after the gap lands.
func TestABlowTooSoonAfterTheLastIsDropped(t *testing.T) {
	h := startWith(t, aDogAtTheStart)
	hendra, _ := h.enterWorld("admin", "Hendra", 3, 6)
	hendra.read(35)
	hendra.hitTheDog()
	hendra.expectDamageOnTheDog()

	hendra.hitTheDog()
	hendra.expectOpenPastBites()
	time.Sleep(150 * time.Millisecond) // a level-1 character's attack speed allows a blow each 100 ms
	hendra.hitTheDog()

	hendra.expectDamageOnTheDog()
}

// Blows and picks the client sent before it took in Change character are dropped.
func TestBlowsInFlightAfterChangeCharacterAreIgnored(t *testing.T) {
	h := startWith(t, aDogAtTheStart)
	hendra, _ := h.enterWorld("admin", "Hendra", 3, 6)
	hendra.read(35)
	hendra.changeCharacter()

	hendra.hitTheDog()
	hendra.target(dogVID)

	hendra.expectOpen()
}

// The dog Hendra hits bites back: everyone who sees it sees its attack, and she is told the
// damage, 27 to 31 for a wild dog on a level-1 sura, and her hit points left.
func TestADogThatIsHitBitesBack(t *testing.T) {
	h := startWith(t, aDogAtTheStart)
	hendra, _ := h.enterWorld("admin", "Hendra", 3, 6)
	hendra.read(35)
	hendra.hitTheDog()
	hendra.read(damageSize)

	move := protocol.NewReader(hendra.read(moveSize))
	if header, kind, _, _, vid := move.U8(), move.U8(), move.U8(), move.U8(), move.U32(); header != byte(protocol.ServerMove) ||
		kind != protocol.MoveAttack || vid != dogVID {
		t.Fatalf("got %02x of type %d for %d, want the dog's attack", header, kind, vid)
	}
	d := protocol.NewReader(hendra.read(damageSize))
	header, victim, _, damage := d.U8(), d.U32(), d.U8(), d.I32()
	if header != byte(protocol.ServerDamage) || victim != 2 || damage < 27 || damage > 31 {
		t.Fatalf("got %02x: %d on %d, want 27 to 31 on Hendra", header, damage, victim)
	}
	p := protocol.NewReader(hendra.read(pointChangeSize)[4:])
	if vid, point, amount, value := p.U32(), protocol.Point(p.U8()), p.I32(), p.I32(); vid != 2 ||
		point != protocol.PointHP || amount != -damage || value <= 0 {
		t.Fatalf("got point %d of %d %+d to %d, want her hit points down %d", point, vid, amount, value, damage)
	}
}

// The hit points a bite leaves are kept when Hendra changes character.
func TestTheHitPointsABiteLeavesAreKept(t *testing.T) {
	h := startWith(t, aDogAtTheStart)
	hendra, _ := h.enterWorld("admin", "Hendra", 3, 6)
	hendra.read(35)
	hendra.hitTheDog()
	hendra.read(damageSize)
	hendra.read(moveSize)
	hendra.read(damageSize)
	p := protocol.NewReader(hendra.read(pointChangeSize)[4:])
	_, _, _, left := p.U32(), p.U8(), p.I32(), p.I32()

	hendra.changeCharacter()

	if got := h.characterValue("hp"); got != int(left) || left >= 950 {
		t.Fatalf("stored %d hit points, want the %d the bite left", got, left)
	}
}

// Hendra, at her last hit point, is killed by the dog and leaves dead: she is kept at
// Jinno's start with all her 950 hit points and 850 mana, which the database takes.
func TestAPlayerWhoLeavesDeadIsKeptWholeAtTheStart(t *testing.T) {
	h := startWith(t, aDogAtTheStart)
	hendra, _ := h.enterWorld("admin", "Hendra", 3, 6)
	hendra.read(35)
	hendra.changeCharacter()
	h.exec(`UPDATE characters SET hp = 1, sp = 100 WHERE name = 'Hendra'`)
	hendra.selectCharacter(0)
	hendra.expect(protocol.EncodePhase(protocol.PhaseLoading))
	hendra.read(46 + 1021 + 1531) // the main character, points and skills
	hendra.enterGame()
	hendra.read(2 + 5 + 2 + appearanceSize + noBlocksSize + 35) // phase, time, channel, herself, blocks, the dog
	hendra.hitTheDog()
	hendra.read(damageSize + moveSize + damageSize + pointChangeSize) // her blow, the bite, her last hit point
	hendra.expect(protocol.EncodeDead(3))                             // a new vid for her second entry

	hendra.changeCharacter()

	if hp, sp, x, y := h.characterValue("hp"), h.characterValue("sp"), h.characterValue("x"), h.characterValue("y"); hp != 950 ||
		sp != 850 || x != 963400 || y != 278200 {
		t.Fatalf("kept with %d and %d at (%d, %d), want 950 and 850 at Jinno's start", hp, sp, x, y)
	}
}

// Cien, a ninja with Łuk+0 and a hundred arrows worn, shoots the dog 1500 away as the
// client does, FLY_TARGETING then SHOOT: the arrow lands, her arrows count down to 99,
// kept, and the connection stays open.
func TestANinjaShootsTheDogAndUsesAnArrow(t *testing.T) {
	h := startWith(t, []world.Spawn{{VNum: 101, X: startX + 1500, Y: startY, Count: 1}})
	cien, _ := h.enterWorld("admin", "Hendra", 3, 6)
	cien.read(35) // the dog
	cien.changeCharacter()
	h.exec(`UPDATE characters SET class = 'ninja' WHERE name = 'Hendra'`)
	h.exec(`INSERT INTO items (character_id, vnum, count, worn) SELECT id, 2000, 1, 'weapon' FROM characters WHERE name = 'Hendra'`)
	h.exec(`INSERT INTO items (character_id, vnum, count, worn) SELECT id, 8000, 100, 'arrow' FROM characters WHERE name = 'Hendra'`)
	cien.startAgain()
	cien.read(35 + 2*42) // the dog, the bow and the arrows

	fly := binary.LittleEndian.AppendUint32([]byte{byte(protocol.ClientFlyTarget)}, dogVID)
	fly = binary.LittleEndian.AppendUint32(binary.LittleEndian.AppendUint32(fly, startX+1500), startY)
	cien.send(fly, []byte{sequenceByte})
	cien.send([]byte{byte(protocol.ClientShoot), 0}, []byte{sequenceByte})

	if damage := cien.expectDamageOnTheDog(); damage < 1 {
		t.Fatalf("the arrow did %d", damage)
	}
	arrows := cien.nextPastBites(42)
	if arrows[0] != byte(protocol.ServerItemSet) || arrows[2] != 99 || arrows[8] != 99 {
		t.Fatalf("got %x, want the arrows at 99, 99 left", arrows)
	}
	var left int
	if err := h.accounts.Pool.QueryRow(t.Context(), `SELECT count FROM items WHERE vnum = 8000`).Scan(&left); err != nil || left != 99 {
		t.Fatalf("kept %d, %v", left, err)
	}
	cien.expectOpenPastBites() // framed whole, sequence bytes and all
}
