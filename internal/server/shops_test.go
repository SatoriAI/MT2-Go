package server

import (
	"context"
	"encoding/binary"
	"testing"

	"github.com/SatoriAI/MT2-Go/internal/protocol"
	"github.com/SatoriAI/MT2-Go/internal/world"
)

// aGeneralStoreAtTheStart is the general store beside where Hendra enters, the first
// character spawned.
var aGeneralStoreAtTheStart = []world.Spawn{{VNum: 9003, X: startX + 100, Y: startY, Count: 1}}

const storeVID = 1

// Hendra, with 100 yang, clicks the general store beside her, buys a red potion for 32 and
// sells it back for 6. Her client closes the shop as she changes character, which ends
// nothing, and she has 74 yang kept and no potion.
func TestAPotionBoughtAndSoldBackIsKept(t *testing.T) {
	h := startWith(t, aGeneralStoreAtTheStart)
	hendra, _ := h.enterWorld("admin", "Hendra", 3, 6)
	hendra.read(35 + 54) // the general store
	hendra.changeCharacter()
	h.exec(`UPDATE characters SET gold = 100 WHERE name = 'Hendra'`)
	hendra.startAgain()
	hendra.read(35 + 54)

	hendra.send(binary.LittleEndian.AppendUint32([]byte{byte(protocol.ClientOnClick)}, storeVID), []byte{sequenceByte})
	if window := hendra.read(1728); window[0] != byte(protocol.ServerShop) || binary.LittleEndian.Uint32(window[4:]) != storeVID {
		t.Fatalf("got %x, want the general store's window", window[:8])
	}
	hendra.send([]byte{byte(protocol.ClientShop), protocol.ShopBuy, 1, 0, sequenceByte})
	hendra.expect(inInventory(0, 27001, 1))
	hendra.expectYang(68)
	hendra.send([]byte{byte(protocol.ClientShop), protocol.ShopSell2, 0, 1, sequenceByte})
	hendra.expect(inInventory(0, 0, 0))
	hendra.expectYang(74)
	hendra.changeCharacter()
	hendra.send([]byte{byte(protocol.ClientShop), protocol.ShopEnd, sequenceByte}) // the window closing as she leaves
	hendra.selectCharacter(0)
	hendra.expect(protocol.EncodePhase(protocol.PhaseLoading))

	var gold, items int
	if err := h.accounts.Pool.QueryRow(context.Background(), `SELECT gold, (SELECT count(*) FROM items)
		FROM characters WHERE name = 'Hendra'`).Scan(&gold, &items); err != nil || gold != 74 || items != 0 {
		t.Fatalf("kept %d yang and %d items, %v", gold, items, err)
	}
}

// expectYang reads a change of the player's yang to have.
func (c *client) expectYang(have uint32) {
	c.t.Helper()
	change := c.read(pointChangeSize)
	if change[0] != byte(protocol.ServerPointChange) || change[8] != byte(protocol.PointGold) ||
		binary.LittleEndian.Uint32(change[13:]) != have {
		c.t.Fatalf("got %x, want yang at %d", change, have)
	}
}
