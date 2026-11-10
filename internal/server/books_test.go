package server

import (
	"context"
	"encoding/binary"
	"testing"

	"github.com/SatoriAI/MT2-Go/internal/protocol"
)

// Hendra, a Weaponry sura with Dragon Swirl at M1, reads its book: the book is gone, from
// her bag and the database, and the skill is kept at M1 or M2, as the roll fell, with the
// time she may read again.
func TestABookReadIsKept(t *testing.T) {
	h := start(t)
	hendra, _ := h.hendraInWorld()
	hendra.changeCharacter()
	h.exec(`UPDATE characters SET level = 40, skill_group = 'weaponry' WHERE name = 'Hendra'`)
	h.exec(`INSERT INTO skills (character_id, skill, level) SELECT id, 'dragon_swirl', 20 FROM characters WHERE name = 'Hendra'`)
	h.give(50462, 1)
	hendra.startAgain()
	hendra.expect(inInventory(0, 50462, 1))

	hendra.send([]byte{byte(protocol.ClientItemUse), 1, 0, 0, sequenceByte})

	hendra.expect(inInventory(0, 0, 0))
	header := hendra.read(1)
	if header[0] == byte(protocol.ServerSkillLevels) {
		hendra.read(1530)
		header = hendra.read(1)
	}
	if header[0] != byte(protocol.ServerChat) {
		t.Fatalf("got header %x, want the line", header)
	}
	hendra.read(int(binary.LittleEndian.Uint16(hendra.read(2))) - 3)
	var level, books int
	var waits bool
	if err := h.accounts.Pool.QueryRow(context.Background(), `SELECT s.level, s.next_read > now(),
		(SELECT count(*) FROM items i WHERE i.character_id = s.character_id)
		FROM skills s JOIN characters c ON c.id = s.character_id WHERE c.name = 'Hendra'`).Scan(&level, &waits, &books); err != nil ||
		level < 20 || level > 21 || !waits || books != 0 {
		t.Fatalf("kept at %d, waiting %v, %d books, %v", level, waits, books, err)
	}
}
