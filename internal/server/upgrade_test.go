package server

import (
	"context"
	"encoding/binary"
	"testing"

	"github.com/SatoriAI/MT2-Go/internal/protocol"
	"github.com/SatoriAI/MT2-Go/internal/world"
)

// Hendra, with 300 yang and a sword +0, drags it onto the blacksmith beside her, is shown
// the refine dialog for +1 at 200 yang, and says yes: the sword is +1 and she has 100
// yang, kept.
func TestASwordUpgradedAtTheBlacksmithIsKept(t *testing.T) {
	h := startWith(t, []world.Spawn{{VNum: 20016, X: startX + 100, Y: startY, Count: 1}})
	hendra, _ := h.enterWorld("admin", "Hendra", 3, 6)
	hendra.read(35 + 54) // the blacksmith
	hendra.changeCharacter()
	h.exec(`UPDATE characters SET gold = 300 WHERE name = 'Hendra'`)
	h.exec(`INSERT INTO items (character_id, vnum, count, cell) SELECT id, 10, 1, 0 FROM characters WHERE name = 'Hendra'`)
	hendra.startAgain()
	hendra.expect(inInventory(0, 10, 1))
	hendra.read(35 + 54)

	hendra.send(binary.LittleEndian.AppendUint32([]byte{byte(protocol.ClientGiveItem)}, 1), []byte{1, 0, 0, 1, sequenceByte})
	hendra.expect(protocol.EncodeRefineInfo(protocol.RefineInfo{Type: protocol.RefineNoQuestion, Cell: 0, From: 10, Into: 11, Cost: 200, Chance: 100}))
	hendra.send([]byte{byte(protocol.ClientRefine), 0, protocol.RefineNoQuestion, sequenceByte})
	hendra.expect(inInventory(0, 11, 1))
	hendra.expectYang(100)

	var gold, vnum int
	if err := h.accounts.Pool.QueryRow(context.Background(), `SELECT gold, (SELECT vnum FROM items)
		FROM characters WHERE name = 'Hendra'`).Scan(&gold, &vnum); err != nil || gold != 100 || vnum != 11 {
		t.Fatalf("kept %d yang and kind %d, %v", gold, vnum, err)
	}
}

// Hendra gives the blacksmith her sword, then changes character as the dialog is open; a
// yes still in flight, and one sent once she is back, upgrade nothing.
func TestADialogLeftOpenOverChangeCharacterUpgradesNothing(t *testing.T) {
	h := startWith(t, []world.Spawn{{VNum: 20016, X: startX + 100, Y: startY, Count: 1}})
	hendra, _ := h.enterWorld("admin", "Hendra", 3, 6)
	hendra.read(35 + 54)
	hendra.changeCharacter()
	h.exec(`UPDATE characters SET gold = 300 WHERE name = 'Hendra'`)
	h.exec(`INSERT INTO items (character_id, vnum, count, cell) SELECT id, 10, 1, 0 FROM characters WHERE name = 'Hendra'`)
	hendra.startAgain()
	hendra.expect(inInventory(0, 10, 1))
	hendra.read(35 + 54)
	hendra.send(binary.LittleEndian.AppendUint32([]byte{byte(protocol.ClientGiveItem)}, 1), []byte{1, 0, 0, 1, sequenceByte})
	hendra.read(60)

	hendra.changeCharacter()
	hendra.send([]byte{byte(protocol.ClientRefine), 0, protocol.RefineNoQuestion, sequenceByte})
	hendra.startAgain()
	hendra.expect(inInventory(0, 10, 1))
	hendra.read(35 + 54)
	hendra.send([]byte{byte(protocol.ClientRefine), 0, protocol.RefineNoQuestion, sequenceByte})
	hendra.changeCharacter() // answered after the yes, so the yes was handled

	var gold, vnum int
	if err := h.accounts.Pool.QueryRow(context.Background(), `SELECT gold, (SELECT vnum FROM items)
		FROM characters WHERE name = 'Hendra'`).Scan(&gold, &vnum); err != nil || gold != 300 || vnum != 10 {
		t.Fatalf("kept %d yang and kind %d, %v", gold, vnum, err)
	}
}
