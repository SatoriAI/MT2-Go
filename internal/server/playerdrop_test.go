package server

import (
	"context"
	"encoding/binary"
	"testing"

	"github.com/SatoriAI/MT2-Go/internal/protocol"
)

// Hendra drops one of her two red potions, and Kira beside her picks it up at once: each
// then has one, kept.
func TestAPotionDroppedIsPickedUpByAnother(t *testing.T) {
	h := start(t)
	hendra, kira := h.hendraAndKira()
	hendra.changeCharacter()
	kira.read(5) // Hendra leaves her view
	h.give(27001, 2)
	hendra.startAgain()
	hendra.read(appearanceSize + 42) // Kira, then the potions
	kira.read(appearanceSize)        // Hendra again

	hendra.send([]byte{byte(protocol.ClientItemDrop2), 1, 0, 0, 0, 0, 0, 0, 1, sequenceByte})
	hendra.expect(inInventory(0, 27001, 2-1))
	lying := kira.read(21)
	if lying[0] != byte(protocol.ServerGroundAdd) || binary.LittleEndian.Uint32(lying[17:]) != 27001 {
		t.Fatalf("Kira was shown %x, want the potion on the ground", lying)
	}
	vid := binary.LittleEndian.Uint32(lying[13:])
	hendra.read(21)

	kira.send(binary.LittleEndian.AppendUint32([]byte{byte(protocol.ClientItemPickup)}, vid), []byte{sequenceByte})
	kira.expect(protocol.EncodeGroundDel(vid))
	kira.expect(inInventory(0, 27001, 1))
	hendra.expect(protocol.EncodeGroundDel(vid))

	var hendras, kiras int
	if err := h.accounts.Pool.QueryRow(context.Background(), `SELECT
		(SELECT coalesce(sum(i.count), 0) FROM items i JOIN characters c ON c.id = i.character_id WHERE c.name = 'Hendra'),
		(SELECT coalesce(sum(i.count), 0) FROM items i JOIN characters c ON c.id = i.character_id WHERE c.name = 'Kira')`).
		Scan(&hendras, &kiras); err != nil || hendras != 1 || kiras != 1 {
		t.Fatalf("Hendra keeps %d, Kira %d, %v", hendras, kiras, err)
	}
}
