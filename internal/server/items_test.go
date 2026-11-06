package server

import (
	"bytes"
	"context"
	"log/slog"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/SatoriAI/MT2-Go/internal/config"
	"github.com/SatoriAI/MT2-Go/internal/item"
	"github.com/SatoriAI/MT2-Go/internal/protocol"
	"github.com/SatoriAI/MT2-Go/internal/store"
)

// give leaves an order for Hendra, as `mt2go item give` does.
func (h *harness) give(vnum uint32, count int) {
	h.t.Helper()
	o, err := item.GiveOrder(vnum, count)
	if err != nil {
		h.t.Fatal(err)
	}
	if _, err := (store.Items{Pool: h.accounts.Pool}).Order(context.Background(), "Hendra", o); err != nil {
		h.t.Fatal(err)
	}
}

// startAgain presses Start on Hendra after Change character and enters the world, reading
// what entering shows up to her inventory.
func (c *client) startAgain() {
	c.t.Helper()
	c.selectCharacter(0)
	c.expect(protocol.EncodePhase(protocol.PhaseLoading))
	c.read(46 + 1021 + 1531) // the main character, points and skills
	c.enterGame()
	c.read(2 + 5 + 2 + appearanceSize + noBlocksSize) // phase, time, channel, herself, blocks
}

func inInventory(cell uint16, vnum uint32, count uint8) []byte {
	return protocol.EncodeItemSet(protocol.ItemPos{Window: 1, Cell: cell}, vnum, count)
}

// A sword and 50 potions given on the command line are hers the next time she is selected,
// and entering shows them: the sword at the top, the potions beside it.
func TestItemsGivenAreShownOnEntering(t *testing.T) {
	h := start(t)
	hendra, _ := h.hendraInWorld()
	hendra.changeCharacter()
	h.give(10, 1)
	h.give(27001, 50)

	hendra.startAgain()

	hendra.expect(inInventory(0, 10, 1))
	hendra.expect(inInventory(1, 27001, 50))
}

// A drag moves the sword, and it is still there after Change character.
func TestADraggedItemStaysWhereItWasPut(t *testing.T) {
	h := start(t)
	hendra, _ := h.hendraInWorld()
	hendra.changeCharacter()
	h.give(10, 1)
	hendra.startAgain()
	hendra.read(42) // the sword

	hendra.send([]byte{byte(protocol.ClientItemMove), 1, 0, 0, 1, 17, 0, 1, sequenceByte})

	hendra.expect(inInventory(0, 0, 0))
	hendra.expect(inInventory(17, 10, 1))
	hendra.changeCharacter()
	hendra.startAgain()
	hendra.expect(inInventory(17, 10, 1))
}

// Whatever else the inventory window sends is taken without ending the connection: a
// double-click, a drop to the ground, a trade, a gift to an NPC.
func TestTheInventoryWindowsOtherPacketsDoNotDisconnect(t *testing.T) {
	h := start(t)
	hendra, _ := h.hendraInWorld()

	for header, size := range map[protocol.ClientHeader]int{
		protocol.ClientItemUse: 4, protocol.ClientItemDrop: 8, protocol.ClientItemDrop2: 9,
		protocol.ClientExchange: 10, protocol.ClientItemUseToItem: 7, protocol.ClientGiveItem: 9,
	} {
		hendra.send([]byte{byte(header)}, make([]byte, size-1), []byte{sequenceByte})
	}

	hendra.expectOpen()
}

// The server refuses to start while someone owns an item of a kind the game does not
// have, and names it, rather than lose it.
func TestTheServerRefusesToStartWithItemsOfKindsItDoesNotHave(t *testing.T) {
	if testing.Short() {
		t.Skip("needs PostgreSQL; run without -short")
	}
	ctx := context.Background()
	pool, err := store.Open(ctx, config.TestDatabaseURL())
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if _, err := pool.Exec(ctx, `TRUNCATE accounts, characters, events RESTART IDENTITY CASCADE;
		INSERT INTO accounts (name, password_hash) VALUES ('admin', 'x');
		INSERT INTO characters (account_id, slot, name, class, sex, strength, vitality, dexterity, intelligence, x, y)
		VALUES (1, 0, 'Hendra', 'sura', 'female', 5, 3, 3, 5, 0, 0);
		INSERT INTO items (character_id, vnum, count, cell) VALUES (1, 11, 1, 0)`); err != nil {
		t.Fatal(err)
	}

	serving, cancel := context.WithTimeout(ctx, 2*time.Second) // were it to start, it stops here
	defer cancel()
	err = Serve(serving, Ports{Auth: listen(t), Game: listen(t)}, pool, nil,
		config.Config{GamePublic: netip.MustParseAddrPort("127.0.0.1:1")}, slog.New(slog.DiscardHandler))

	if err == nil || !strings.Contains(err.Error(), "[11]") {
		t.Fatalf("started with %v", err)
	}
}

// A pick-up of nothing on the ground is ignored, and the connection stays: the packet is
// framed at the size the real client sends it.
func TestAPickUpOfNothingIsIgnored(t *testing.T) {
	h := start(t)
	hendra, _ := h.hendraInWorld()

	hendra.send([]byte{byte(protocol.ClientItemPickup), 0xa2, 0xbb, 0x0d, 0x00, sequenceByte})

	hendra.expectOpen()
}

// groundSizes are the packets an item on the ground can send, which a dog's death may or
// may not bring: what it drops is rolled.
var groundSizes = map[byte]int{byte(protocol.ServerGroundAdd): 21, byte(protocol.ServerItemOwner): 30,
	byte(protocol.ServerGroundDel): 5}

// expectPastDrops is expect, passing over whatever the dog dropped coming and going.
func (c *client) expectPastDrops(want []byte) {
	c.t.Helper()
	for {
		header := c.read(1)
		if n, ok := groundSizes[header[0]]; ok {
			c.read(n - 1)
			continue
		}
		if got := append(header, c.read(len(want)-1)...); !bytes.Equal(got, want) {
			c.t.Fatalf("got %x, want %x", got, want)
		}
		return
	}
}
