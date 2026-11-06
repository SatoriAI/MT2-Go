package server

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/SatoriAI/MT2-Go/internal/protocol"
	"github.com/SatoriAI/MT2-Go/internal/world"
)

// aWeaponryTeacherAtTheStart is the sura's Weaponry teacher beside where Hendra enters,
// the first character spawned.
var aWeaponryTeacherAtTheStart = []world.Spawn{{VNum: 20304, X: startX + 100, Y: startY, Count: 1}}

const teacherVID = 1

// Hendra, a level-6 sura, takes up Weaponry with its teacher and raises Dragon Swirl twice
// with her two points; after Change character, an answer to a dialog still in flight
// ending nothing, she enters with it at 2 and no points left.
func TestASuraTakesUpWeaponryAndKeepsTheSkillsSheRaised(t *testing.T) {
	h := startWith(t, aWeaponryTeacherAtTheStart)
	hendra, _ := h.enterWorld("admin", "Hendra", 3, 6)
	hendra.read(35 + 54) // the teacher
	hendra.changeCharacter()
	h.exec(`UPDATE characters SET level = 6 WHERE name = 'Hendra'`)
	hendra.startAgain()
	hendra.read(35 + 54)

	hendra.send(binary.LittleEndian.AppendUint32([]byte{byte(protocol.ClientOnClick)}, teacherVID), []byte{sequenceByte})
	hendra.readScript()
	hendra.send([]byte{byte(protocol.ClientScriptAnswer), 0, sequenceByte})
	hendra.readScript()
	hendra.send([]byte{byte(protocol.ClientScriptAnswer), 0, sequenceByte})
	hendra.expect(protocol.EncodeChangeSkillGroup(1))
	hendra.read(1531)           // every skill at 0
	hendra.expectSkillPoints(2) // her two points
	hendra.readScript()         // the teacher's last word
	hendra.say("/skillup 62")
	hendra.read(1531)
	hendra.expectSkillPoints(1)
	hendra.say("/skillup 62")
	hendra.read(1531)
	hendra.expectSkillPoints(0)

	hendra.changeCharacter()
	hendra.send([]byte{byte(protocol.ClientScriptAnswer), 254, sequenceByte}) // a dialog closed as she left
	hendra.selectCharacter(0)
	hendra.expect(protocol.EncodePhase(protocol.PhaseLoading))
	if main := hendra.read(46); main[len(main)-1] != 1 { // the skill group, last
		t.Fatalf("entered in group %d", main[len(main)-1])
	}
	points := hendra.read(1021)[1:]
	skills := hendra.read(1531)[1:]
	if p := binary.LittleEndian.Uint32(points[4*protocol.PointSkillPoints:]); p != 0 || skills[62*6+1] != 2 {
		t.Fatalf("entered with %d points and Dragon Swirl at %d", p, skills[62*6+1])
	}
}

// readScript reads one NPC dialog, whatever it says.
func (c *client) readScript() {
	c.t.Helper()
	head := c.read(3)
	if head[0] != byte(protocol.ServerScript) {
		c.t.Fatalf("got %x, want a dialog", head)
	}
	c.read(int(binary.LittleEndian.Uint16(head[1:])) - 3)
}

// expectSkillPoints reads a change of the player's skill points to have.
func (c *client) expectSkillPoints(have uint32) {
	c.t.Helper()
	change := c.read(pointChangeSize)
	if change[0] != byte(protocol.ServerPointChange) || change[8] != byte(protocol.PointSkillPoints) ||
		binary.LittleEndian.Uint32(change[13:]) != have {
		c.t.Fatalf("got %x, want skill points at %d", change, have)
	}
}

// An offer left open as she changes character is forgotten: a yes to it as she comes back
// asks nothing, and the teacher offers afresh on the next click.
func TestAnOfferIsForgottenOnChangeCharacter(t *testing.T) {
	h := startWith(t, aWeaponryTeacherAtTheStart)
	hendra, _ := h.enterWorld("admin", "Hendra", 3, 6)
	hendra.read(35 + 54)
	hendra.changeCharacter()
	h.exec(`UPDATE characters SET level = 6 WHERE name = 'Hendra'`)
	hendra.startAgain()
	hendra.read(35 + 54)
	click := binary.LittleEndian.AppendUint32([]byte{byte(protocol.ClientOnClick)}, teacherVID)
	hendra.send(click, []byte{sequenceByte})
	hendra.readScript()

	hendra.changeCharacter()
	hendra.startAgain()
	hendra.read(35 + 54)
	hendra.send([]byte{byte(protocol.ClientScriptAnswer), 0, sequenceByte})
	hendra.send(click, []byte{sequenceByte})

	offer := hendra.read(3)
	text := hendra.read(int(binary.LittleEndian.Uint16(offer[1:])) - 3)[3:]
	if !bytes.HasPrefix(text, []byte("Mog")) {
		t.Fatalf("got %q, want the offer afresh", text)
	}
}
