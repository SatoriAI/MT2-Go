package server

import (
	"context"
	"encoding/binary"
	"strings"
	"testing"

	"github.com/SatoriAI/MT2-Go/internal/protocol"
)

// whisper sends a whisper as the client frames it: header, total size, the recipient's
// name in its 25-byte field, the text and its NUL, then the sequence byte.
func (c *client) whisper(to, text string) {
	c.t.Helper()
	size := 1 + 2 + 25 + len(text) + 1
	name := make([]byte, 25)
	copy(name, to)
	packet := binary.LittleEndian.AppendUint16([]byte{byte(protocol.ClientWhisper)}, uint16(size)) //nolint:gosec // test lines are short
	c.send(packet, name, []byte(text), []byte{0, sequenceByte})
}

// whispers reads the stored whispers in the order they were sent, as sender, recipient
// and text.
func (h *harness) whispers() [][3]any {
	h.t.Helper()
	rows, err := h.accounts.Pool.Query(context.Background(),
		`SELECT sender, recipient, text FROM whispers ORDER BY sent_at, id`)
	if err != nil {
		h.t.Fatal(err)
	}
	defer rows.Close()
	var got [][3]any
	for rows.Next() {
		var sender, recipient int32
		var text string
		if err := rows.Scan(&sender, &recipient, &text); err != nil {
			h.t.Fatal(err)
		}
		got = append(got, [3]any{sender, recipient, text})
	}
	if err := rows.Err(); err != nil {
		h.t.Fatal(err)
	}
	return got
}

// A whisper reaches the player named wherever they are, here on another map, and nobody
// else. The sender's client shows its own line, so nothing comes back.
func TestAWhisperReachesOnlyItsRecipientWhereverTheyAre(t *testing.T) {
	h := start(t)
	hendra, kira := h.hendraAndKira()
	sora, _ := h.enterWorld("carl", "Sora", 1, 0)

	hendra.whisper("Sora", "hi")

	sora.expect(protocol.EncodeWhisper(protocol.WhisperLine, "Hendra", "hi"))
	hendra.expectOpen()
	kira.expectOpen()
}

func TestAWhisperFindsItsRecipientWhateverTheCaseOfTheName(t *testing.T) {
	h := start(t)
	hendra, kira := h.hendraAndKira()

	hendra.whisper("kIRA", "hi")

	kira.expect(protocol.EncodeWhisper(protocol.WhisperLine, "Hendra", "hi"))
}

func TestAWhisperToNobodyInTheGameTellsTheSenderWithTheNameTheyTyped(t *testing.T) {
	h := start(t)
	hendra, _ := h.enterWorld("admin", "Hendra", 3, 6)

	hendra.whisper("Nobody", "hi")

	hendra.expect(protocol.EncodeWhisper(protocol.WhisperNotOnline, "Nobody", ""))
	if got := h.whispers(); len(got) != 0 {
		t.Fatalf("stored %v, want nothing", got)
	}
}

// A player on the character screen is not in the game.
func TestAWhisperToSomeoneChoosingACharacterFindsNobody(t *testing.T) {
	h := start(t)
	hendra, kira := h.hendraAndKira()
	kira.changeCharacter()
	hendra.expect(protocol.EncodeCharacterDelete(2))

	hendra.whisper("Kira", "hi")

	hendra.expect(protocol.EncodeWhisper(protocol.WhisperNotOnline, "Kira", ""))
}

// The client writes in cp1250; the line is passed on in it and stored as UTF-8, between
// the two characters' ids.
func TestAWhisperIsKeptBetweenTheTwoCharactersInUTF8(t *testing.T) {
	h := start(t)
	hendra, kira := h.hendraAndKira()
	const yellowGoose = "\xbf\xf3\xb3ta g\xea\x9c" // "żółta gęś"

	hendra.whisper("Kira", yellowGoose)
	kira.expect(protocol.EncodeWhisper(protocol.WhisperLine, "Hendra", yellowGoose))
	kira.whisper("Hendra", "hi")
	hendra.expect(protocol.EncodeWhisper(protocol.WhisperLine, "Kira", "hi"))
	hendra.expectOpen() // each session has finished with its whisper, saving included
	kira.expectOpen()

	got, want := h.whispers(), [][3]any{{int32(1), int32(2), "żółta gęś"}, {int32(2), int32(1), "hi"}}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("stored %q, want %q", got, want)
	}
}

// The client shows the sender's line itself, so a whisper to oneself is left at that:
// nothing comes back, whatever the case of the name, and nothing is kept.
func TestAWhisperToOneselfIsIgnored(t *testing.T) {
	h := start(t)
	hendra, _ := h.enterWorld("admin", "Hendra", 3, 6)

	hendra.whisper("hENDRA", "hi")

	hendra.expectOpen()
	if got := h.whispers(); len(got) != 0 {
		t.Fatalf("stored %v, want nothing", got)
	}
}

func TestAnEmptyWhisperIsIgnored(t *testing.T) {
	h := start(t)
	hendra, kira := h.hendraAndKira()

	hendra.whisper("Kira", "")

	kira.expectOpen()
	hendra.expectOpen()
	if got := h.whispers(); len(got) != 0 {
		t.Fatalf("stored %v, want nothing", got)
	}
}

// A line that reached its recipient but could not be kept is still theirs; the sender is
// told, and both stay in the game. Kira's row is gone, so the save breaks its reference.
func TestAWhisperThatCannotBeKeptIsDeliveredAndTheSenderTold(t *testing.T) {
	h := start(t)
	hendra, kira := h.hendraAndKira()
	h.exec(`DELETE FROM characters WHERE name = 'Kira'`)

	hendra.whisper("Kira", "hi")

	kira.expect(protocol.EncodeWhisper(protocol.WhisperLine, "Hendra", "hi"))
	hendra.expect(protocol.EncodeWhisper(protocol.WhisperError, "Kira", "Not saved."))
	hendra.expectOpen()
}

// The client refuses to send 255 bytes or more: the longest it sends is passed on, and
// anything longer is not the client's.
func TestAWhisperLongerThanTheClientSendsEndsTheConnection(t *testing.T) {
	h := start(t)
	hendra, kira := h.hendraAndKira()
	longest := strings.Repeat("x", 254)
	hendra.whisper("Kira", longest)
	kira.expect(protocol.EncodeWhisper(protocol.WhisperLine, "Hendra", longest))

	hendra.whisper("Kira", longest+"x")

	hendra.expectClosed()
}
