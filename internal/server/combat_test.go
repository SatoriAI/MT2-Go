package server

import (
	"encoding/binary"
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

// expectDamage reads the damage of a blow on vid and returns it.
func (c *client) expectDamage(vid uint32) int32 {
	c.t.Helper()
	r := protocol.NewReader(c.read(10))
	if header, victim, flags, damage := r.U8(), r.U32(), r.U8(), r.I32(); header != byte(protocol.ServerDamage) ||
		victim != vid || flags != protocol.DamageNormal {
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

	damage := hendra.expectDamage(dogVID)
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
	hendra.expectDamage(dogVID)

	hendra.hitTheDog()
	hendra.expectOpen()
	time.Sleep(150 * time.Millisecond) // a level-1 character's attack speed allows a blow each 100 ms
	hendra.hitTheDog()

	hendra.expectDamage(dogVID)
}

// Blows and picks the client sent before it took in Change character are dropped.
func TestBlowsInFlightAfterChangeCharacterAreIgnored(t *testing.T) {
	h := startWith(t, aDogAtTheStart)
	hendra, _ := h.enterWorld("admin", "Hendra", 3, 6)
	hendra.read(35)
	hendra.say("/phase_select")
	hendra.expect(protocol.EncodePhase(protocol.PhaseSelect))

	hendra.hitTheDog()
	hendra.target(dogVID)

	hendra.expectOpen()
}
