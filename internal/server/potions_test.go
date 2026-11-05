package server

import (
	"encoding/binary"
	"testing"

	"github.com/SatoriAI/MT2-Go/internal/protocol"
)

// Hendra, hurt to 1 hit point, drinks one of two red potions with a double-click: one is
// left, she has 301 hit points and sees the potion's effect. After Change character she
// enters with both kept.
func TestAPotionDrunkHealsAndIsKeptUsedUp(t *testing.T) {
	h := start(t)
	hendra, _ := h.hendraInWorld()
	hendra.changeCharacter()
	h.exec(`UPDATE characters SET hp = 1 WHERE name = 'Hendra'`)
	h.give(27001, 2)
	hendra.startAgain()
	hendra.expect(inInventory(0, 27001, 2))

	hendra.send([]byte{byte(protocol.ClientItemUse), 1, 0, 0, sequenceByte})

	hendra.expect(inInventory(0, 27001, 1))
	hp := hendra.read(17)
	if hp[0] != byte(protocol.ServerPointChange) || binary.LittleEndian.Uint32(hp[13:]) != 301 {
		t.Fatalf("got %x, want her at 301 hit points", hp)
	}
	hendra.expect(protocol.EncodeSpecialEffect(protocol.EffectRedPotion, binary.LittleEndian.Uint32(hp[4:])))
	hendra.changeCharacter()
	hendra.selectCharacter(0)
	hendra.expect(protocol.EncodePhase(protocol.PhaseLoading))
	points := hendra.read(46 + 1021)[46+1:]
	if kept := binary.LittleEndian.Uint32(points[4*protocol.PointHP:]); kept != 301 {
		t.Fatalf("entered with %d hit points", kept)
	}
	hendra.read(1531) // skills
	hendra.enterGame()
	hendra.read(2 + 5 + 2 + appearanceSize + noBlocksSize)
	hendra.expect(inInventory(0, 27001, 1))
}
