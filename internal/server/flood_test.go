package server

import (
	"fmt"
	"testing"

	"github.com/SatoriAI/MT2-Go/internal/protocol"
)

// floodLines is the game's limit: lines a player may say, chat and whispers together, in
// five seconds.
const floodLines = 10

// sayUpToTheLimit has hendra say as many numbered lines as the limit allows, and checks
// kira and hendra each get them.
func sayUpToTheLimit(hendra, kira *client) {
	for i := range floodLines {
		hendra.say(fmt.Sprint(i))
		line := protocol.EncodeChat(protocol.ChatNormal, 1, 3, fmt.Sprintf("Hendra : %d", i))
		hendra.expect(line)
		kira.expect(line)
	}
}

// A line over the limit is dropped without a word, and the speaker stays in the game.
func TestALineOverTheFloodLimitReachesNobody(t *testing.T) {
	h := start(t)
	hendra, kira := h.hendraAndKira()
	sayUpToTheLimit(hendra, kira)

	hendra.say("one too many")

	kira.expectOpen()
	hendra.expectOpen()
}

func TestWhispersCountAgainstTheSameLimitAsChat(t *testing.T) {
	h := start(t)
	hendra, kira := h.hendraAndKira()
	sayUpToTheLimit(hendra, kira)

	hendra.whisper("Kira", "hi")

	kira.expectOpen()
	hendra.expectOpen() // not even "not online": a flood gets no replies
	if got := h.whispers(); len(got) != 0 {
		t.Fatalf("stored %v, want nothing", got)
	}
}

// Commands act on the player alone, so they are not counted: leaving always works.
func TestACommandStillWorksOnceTheLimitIsReached(t *testing.T) {
	h := start(t)
	hendra, kira := h.hendraAndKira()
	sayUpToTheLimit(hendra, kira)
	hendra.say("one too many")

	hendra.say("/phase_select")

	hendra.expect(protocol.EncodePhase(protocol.PhaseSelect))
}
