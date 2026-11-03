package server

import (
	"encoding/binary"
	"testing"

	"github.com/SatoriAI/MT2-Go/internal/protocol"
)

// appearanceSize is a character's add and info, which show it to a client.
const appearanceSize = 35 + 54

// enterWorld creates an account with one character in the kingdom, enters the world with
// it on a new connection and returns that connection, with everything the server sent so
// far read, and the character's appearance as its own client was shown it.
func (h *harness) enterWorld(account, name string, kingdom byte, race uint16) (*client, []byte) {
	h.t.Helper()
	h.createAccount(account)
	game := h.enterSelection(account, h.authenticate(account))
	game.expect(protocol.EncodePhase(protocol.PhaseSelect))
	game.chooseEmpire(kingdom, sequenceByte)
	game.createCharacter(0, name, race, 1)
	game.read(65)
	game.selectCharacter(0)
	game.expect(protocol.EncodePhase(protocol.PhaseLoading))
	game.read(46 + 1021 + 1531) // the main character, points and skills
	game.enterGame()
	game.read(2 + 5 + 2) // phase, time, channel
	return game, game.read(appearanceSize)
}

// Hendra and Kira enter Jinno's first village, Hendra first; each sees the other.
func (h *harness) hendraAndKira() (hendra, kira *client) {
	h.t.Helper()
	hendra, hendraLook := h.enterWorld("admin", "Hendra", 3, 6)
	kira, kiraLook := h.enterWorld("bob", "Kira", 3, 0)
	kira.expect(hendraLook)
	hendra.expect(kiraLook)
	return hendra, kira
}

// A player entering sees who is already on the map, as they see themselves, and is shown
// to them the same way.
func TestPlayersOnTheSameMapSeeEachOther(t *testing.T) {
	h := start(t)

	h.hendraAndKira()
}

func TestAMoveIsShownToTheOthersButNotToTheMover(t *testing.T) {
	h := start(t)
	hendra, kira := h.hendraAndKira()

	hendra.send(moveTo(963500, 278300))

	kira.expect(protocol.EncodeMove(1, protocol.Move{Type: 1, Rotation: 64, X: 963500, Y: 278300, Time: 12345}))
	hendra.expectOpen()
}

// Someone arriving after a move sees the character where it moved to.
func TestAPlayerArrivingSeesTheOthersWhereTheyStandNow(t *testing.T) {
	h := start(t)
	hendra, _ := h.enterWorld("admin", "Hendra", 3, 6)
	hendra.send(moveTo(963500, 278300))
	hendra.expectOpen()

	kira, _ := h.enterWorld("bob", "Kira", 3, 0)

	position := protocol.NewReader(kira.read(appearanceSize)[9:]) // after header, vid and rotation
	if x, y := position.I32(), position.I32(); x != 963500 || y != 278300 {
		t.Fatalf("Hendra shown at (%d, %d), want where she moved", x, y)
	}
}

func TestALineIsHeardByTheOthers(t *testing.T) {
	h := start(t)
	hendra, kira := h.hendraAndKira()

	hendra.say("hi")

	line := protocol.EncodeChat(protocol.ChatNormal, 1, 3, "Hendra : hi")
	hendra.expect(line)
	kira.expect(line)
}

// However the player leaves, the others stop seeing them.
func TestAPlayerLeavingDisappearsForTheOthers(t *testing.T) {
	cases := map[string]func(*client){
		"logout":           func(c *client) { c.say("/logout") },
		"change character": func(c *client) { c.say("/phase_select") },
		"a dropped connection": func(c *client) {
			_ = c.conn.Close()
		},
	}
	for name, leave := range cases {
		t.Run(name, func(t *testing.T) {
			h := start(t)
			hendra, kira := h.hendraAndKira()

			leave(hendra)

			kira.expect(protocol.EncodeCharacterDelete(1))
		})
	}
}

// Sora, in Shinsoo's first village, is on another map: she and Hendra never hear of each
// other.
func TestAPlayerOnAnotherMapIsNotSeen(t *testing.T) {
	h := start(t)
	hendra, _ := h.enterWorld("admin", "Hendra", 3, 6)
	sora, _ := h.enterWorld("carl", "Sora", 1, 0)

	hendra.send(moveTo(963500, 278300))
	hendra.say("hi")
	sora.say("hi")

	hendra.expect(protocol.EncodeChat(protocol.ChatNormal, 1, 3, "Hendra : hi"))
	hendra.expectOpen()
	sora.expect(protocol.EncodeChat(protocol.ChatNormal, 2, 1, "Sora : hi"))
	sora.expectOpen()
}

// Clicking another character, or picking it as a target, sends packets nothing answers
// yet; they must not end the connection.
func TestClickingOrTargetingACharacterIsIgnored(t *testing.T) {
	h := start(t)
	hendra, _ := h.hendraAndKira()

	for _, header := range []protocol.ClientHeader{protocol.ClientOnClick, protocol.ClientTarget} {
		hendra.send(binary.LittleEndian.AppendUint32([]byte{byte(header)}, 2), []byte{sequenceByte})
	}

	hendra.expectOpen()
}
