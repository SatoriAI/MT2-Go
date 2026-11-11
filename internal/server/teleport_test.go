package server

import (
	"encoding/binary"
	"net/netip"
	"testing"

	"github.com/SatoriAI/MT2-Go/internal/account"
	"github.com/SatoriAI/MT2-Go/internal/item"
	"github.com/SatoriAI/MT2-Go/internal/protocol"
)

// Hendra, of level 30 and a long way from her village, uses the teleport ring and picks
// the village: she is told to connect again, and a new connection with the same key, the
// old one still open as the client leaves it a moment, loads her at Jinno's start.
func TestTheRingTakesThePlayerToTheStartOverANewConnection(t *testing.T) {
	h := start(t)
	hendra, key := h.hendraInWorldAt(5000, 0)
	hendra.changeCharacter()
	h.exec(`UPDATE characters SET level = 30 WHERE name = 'Hendra'`)
	h.give(item.TeleportRing, 1)
	hendra.startAgain()
	hendra.expect(inInventory(0, item.TeleportRing, 1))

	hendra.send([]byte{byte(protocol.ClientItemUse), 1, 0, 0, sequenceByte})
	head := hendra.read(3)
	hendra.read(int(binary.LittleEndian.Uint16(head[1:])) - 3) // the question
	hendra.send([]byte{byte(protocol.ClientScriptAnswer), 0, sequenceByte})

	x, y, _ := account.EmpireJinno.StartPosition()
	hendra.expect(protocol.EncodeWarp(x, y, netip.MustParseAddrPort(h.game)))
	if atX, atY := h.startHendra(key); atX != x || atY != y {
		t.Fatalf("loaded at (%d, %d), want the start (%d, %d)", atX, atY, x, y)
	}
}
