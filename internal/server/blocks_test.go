package server

import (
	"context"
	"testing"

	"github.com/SatoriAI/MT2-Go/internal/protocol"
)

// blockMode is the reply the client's options window reads, for a player of vid in Jinno.
func blockMode(vid uint32, text string) []byte {
	return protocol.EncodeChat(protocol.ChatCommand, vid, 3, text)
}

// reenterHendra presses Start on Hendra on a new connection with the key and returns it,
// read up to what follows her own character.
func (h *harness) reenterHendra(key uint32) *client {
	h.t.Helper()
	game := h.enterSelection("admin", key)
	game.read(2 + 329 + 2) // kingdom, list, select phase
	game.selectCharacter(0)
	game.expect(protocol.EncodePhase(protocol.PhaseLoading))
	game.read(46 + 1021 + 1531) // the main character, points and skills
	game.enterGame()
	game.read(2 + 5 + 2 + appearanceSize) // phase, time, channel, the player and their info
	return game
}

// The choice outlives the login: told again on the next entry, and still refusing
// whispers.
func TestBlockingIsToldBackAndKeptForTheNextEntry(t *testing.T) {
	h := start(t)
	game, key := h.hendraInWorld()

	game.say("/setblockmode 8")

	game.expect(blockMode(1, "setblockmode 8"))
	game.say("/phase_select")
	game.expect(protocol.EncodePhase(protocol.PhaseSelect))
	h.reenterHendra(key).expect(blockMode(2, "setblockmode 8"))
	kira, _ := h.enterWorld("bob", "Kira", 3, 0)
	kira.read(appearanceSize) // Hendra, standing there
	kira.whisper("Hendra", "hi")
	kira.expect(protocol.EncodeWhisper(protocol.WhisperTargetBlocked, "Hendra", ""))
}

// The client sends its whole mask; bits it does not name, and a mask that is not a
// number, change nothing they should not.
func TestUnknownBlockBitsAreDroppedAndANonNumberIsIgnored(t *testing.T) {
	h := start(t)
	game := h.inWorld()

	game.say("/setblockmode 72") // whisper, and a bit the client does not name
	game.expect(blockMode(1, "setblockmode 8"))
	game.say("/setblockmode x")

	game.expectOpen()
	var blocks []string
	if err := h.accounts.Pool.QueryRow(context.Background(),
		`SELECT blocks FROM characters WHERE name = 'Hendra'`).Scan(&blocks); err != nil ||
		len(blocks) != 1 || blocks[0] != "whisper" {
		t.Fatalf("stored %v, err %v; want whisper only", blocks, err)
	}
}

func TestAWhisperToSomeoneBlockingWhispersIsRefused(t *testing.T) {
	h := start(t)
	hendra, kira := h.hendraAndKira()
	kira.say("/setblockmode 8")
	kira.expect(blockMode(2, "setblockmode 8"))

	hendra.whisper("Kira", "hi")

	hendra.expect(protocol.EncodeWhisper(protocol.WhisperTargetBlocked, "Kira", ""))
	kira.expectOpen()
	if got := h.whispers(); len(got) != 0 {
		t.Fatalf("stored %v, want nothing", got)
	}
}

// A player who blocks whispers may not send them either, as the client says.
func TestAPlayerBlockingWhispersCannotSendThem(t *testing.T) {
	h := start(t)
	hendra, kira := h.hendraAndKira()
	hendra.say("/setblockmode 8")
	hendra.expect(blockMode(1, "setblockmode 8"))

	hendra.whisper("Kira", "hi")

	hendra.expect(protocol.EncodeWhisper(protocol.WhisperSenderBlocked, "Kira", ""))
	kira.expectOpen()
}

// Other blocks wait for their mechanics; they do not stop whispers.
func TestOtherBlocksLetWhispersThrough(t *testing.T) {
	h := start(t)
	hendra, kira := h.hendraAndKira()
	kira.say("/setblockmode 55") // everything but whispers
	kira.expect(blockMode(2, "setblockmode 55"))

	hendra.whisper("Kira", "hi")

	kira.expect(protocol.EncodeWhisper(protocol.WhisperLine, "Hendra", "hi"))
}

// Choosing what to block writes to the database, so unlike leaving it counts against the
// flood limit; a change dropped gets no reply, and nothing is kept.
func TestABlockChangeOverTheFloodLimitIsDropped(t *testing.T) {
	h := start(t)
	hendra, kira := h.hendraAndKira()
	sayUpToTheLimit(hendra, kira)

	hendra.say("/setblockmode 8")

	hendra.expectOpen()
	var blocks []string
	if err := h.accounts.Pool.QueryRow(context.Background(),
		`SELECT blocks FROM characters WHERE name = 'Hendra'`).Scan(&blocks); err != nil || len(blocks) != 0 {
		t.Fatalf("stored %v, err %v; want nothing", blocks, err)
	}
}
