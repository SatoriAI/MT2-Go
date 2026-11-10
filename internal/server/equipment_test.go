package server

import (
	"encoding/binary"
	"testing"

	"github.com/SatoriAI/MT2-Go/internal/protocol"
)

// Hendra puts on a sword given to her with a double-click: its cells empty, it shows in the
// weapon's place, her model holds it, and she strikes at its 22 more attack speed. After Change character she enters still holding
// it, which the world is told of in her CHARACTER_INFO, at its attack speed in her POINTS
// and her own ADD.
func TestAWornSwordIsKeptAndShownOnEntering(t *testing.T) {
	h := start(t)
	hendra, _ := h.hendraInWorld()
	hendra.changeCharacter()
	h.give(10, 1)
	hendra.startAgain()
	hendra.expect(inInventory(0, 10, 1))

	hendra.send([]byte{byte(protocol.ClientItemUse), 1, 0, 0, sequenceByte})

	hendra.expect(inInventory(0, 0, 0))
	hendra.expect(inInventory(94, 10, 1))
	if update := hendra.read(35); update[0] != byte(protocol.ServerCharacterUpdate) ||
		binary.LittleEndian.Uint16(update[7:]) != 10 {
		t.Fatalf("got %x, want her holding the sword", update)
	}
	if told := hendra.read(17); told[0] != byte(protocol.ServerPointChange) || protocol.Point(told[8]) != protocol.PointAttackSpeed ||
		binary.LittleEndian.Uint32(told[13:]) != 150+22 { // the sword's 22 on her 150
		t.Fatalf("got %x, want her attack speed at 172", told)
	}
	hendra.changeCharacter()
	hendra.selectCharacter(0)
	hendra.expect(protocol.EncodePhase(protocol.PhaseLoading))
	points := hendra.read(46 + 1021 + 1531)[46:] // the main character, points and skills
	if speed := binary.LittleEndian.Uint32(points[1+4*protocol.PointAttackSpeed:]); speed != 150+22 {
		t.Fatalf("her points show attack speed %d", speed)
	}
	hendra.enterGame()
	look := hendra.read(2 + 5 + 2 + appearanceSize)[2+5+2:]               // phase, time, channel, then herself
	if weapon := binary.LittleEndian.Uint16(look[35+32:]); weapon != 10 { // INFO: header, vid, name, body, weapon
		t.Fatalf("enters holding %d", weapon)
	}
	if speed := look[25]; speed != 150+22 { // ADD: header, vid, angle, x, y, z, type, race, move speed, attack speed
		t.Fatalf("enters striking at %d", speed)
	}
	hendra.read(noBlocksSize)
	hendra.expect(inInventory(94, 10, 1))
}

// Armour dragged onto the body's place is worn on the character screen too.
func TestTheArmourWornShowsOnTheCharacterScreen(t *testing.T) {
	h := start(t)
	hendra, _ := h.hendraInWorld()
	hendra.changeCharacter()
	h.give(11600, 1)
	hendra.startAgain()
	hendra.read(42) // the armour, at 0

	hendra.send([]byte{byte(protocol.ClientItemMove), 1, 0, 0, 1, 90, 0, 1, sequenceByte})
	hendra.read(2*42 + 35 + 17) // its cells emptied, it worn, her model and her DEF

	list := hendra.changeCharacter()
	if body := binary.LittleEndian.Uint16(list[40:]); body != 11600 { // the header, id, name, race, level, play time, stats
		t.Fatalf("the character screen shows body %d", body)
	}
}

// Kira, beside her, sees Hendra put the sword on.
func TestOthersSeeWhatIsPutOn(t *testing.T) {
	h := start(t)
	hendra, kira := h.hendraAndKira()
	hendra.changeCharacter()
	kira.read(5) // Hendra leaves her view
	h.give(10, 1)
	hendra.startAgain()
	hendra.read(appearanceSize + 42) // Kira, then the sword
	kira.read(appearanceSize)        // Hendra again

	hendra.send([]byte{byte(protocol.ClientItemUse), 1, 0, 0, sequenceByte})

	if update := kira.read(35); update[0] != byte(protocol.ServerCharacterUpdate) ||
		binary.LittleEndian.Uint16(update[7:]) != 10 {
		t.Fatalf("Kira got %x, want Hendra holding the sword", update)
	}
}
