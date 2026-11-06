package server

import (
	"testing"

	"github.com/SatoriAI/MT2-Go/internal/protocol"
)

// Hendra drags her red potions to the taskbar's first slot, key 1. After Change character,
// hurt, she enters with them still there, told after her inventory, and a use of their
// cell, which is what the client sends for the key, drinks one.
func TestPotionsOnTheTaskbarAreKeptOverChangeCharacter(t *testing.T) {
	h := start(t)
	hendra, _ := h.hendraInWorld()
	hendra.changeCharacter()
	h.give(27001, 2)
	hendra.startAgain()
	hendra.expect(inInventory(0, 27001, 2))

	hendra.send([]byte{byte(protocol.ClientQuickslotAdd), 0, 1, 0, sequenceByte})
	key1 := protocol.EncodeQuickslotAdd(protocol.QuickslotAdd{Slot: 0, Type: 1, Position: 0})
	hendra.expect(key1)

	hendra.changeCharacter()
	h.exec(`UPDATE characters SET hp = 1 WHERE name = 'Hendra'`)
	hendra.startAgain()
	hendra.expect(inInventory(0, 27001, 2))
	hendra.expect(key1)

	hendra.send([]byte{byte(protocol.ClientItemUse), 1, 0, 0, sequenceByte})
	hendra.expect(inInventory(0, 27001, 1))
}
