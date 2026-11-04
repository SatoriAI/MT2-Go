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
