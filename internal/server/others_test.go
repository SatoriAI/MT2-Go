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

// Jinno's first village spans x from 921600; its start is 963400, 278200. These are
// positions on that map relative to the start, in and out of view of it.
const (
	startX, startY = 963400, 278200
	edgeOfView     = startX + 8000 // still in view
	outOfView      = startX + 9000
)

// movedBy is a move as moveTo sends it, as the server passes it on for vid.
func movedBy(vid uint32, x, y int32) []byte {
	return protocol.EncodeMove(vid, protocol.Move{Type: 1, Rotation: 64, X: x, Y: y, Time: 12345})
}

// expectAppearanceAt reads a character's add and info and checks whose they are and where.
func (c *client) expectAppearanceAt(vid uint32, x, y int32) {
	c.t.Helper()
	add := protocol.NewReader(c.read(appearanceSize)[1:])
	if v, _, ax, ay := add.U32(), add.U32(), add.I32(), add.I32(); v != vid || ax != x || ay != y {
		c.t.Fatalf("shown vid %d at (%d, %d), want %d at (%d, %d)", v, ax, ay, vid, x, y)
	}
}

func TestWalkingOutOfViewHidesBothFromEachOther(t *testing.T) {
	h := start(t)
	hendra, kira := h.hendraAndKira()

	hendra.send(moveTo(outOfView, startY))

	kira.expect(protocol.EncodeCharacterDelete(1))
	hendra.expect(protocol.EncodeCharacterDelete(2))
	kira.expectOpen() // and no move from someone out of view
}

// Coming back into view, each is shown the other again, and the onlooker then the move
// that brought them.
func TestWalkingBackIntoViewShowsBothBeforeTheMove(t *testing.T) {
	h := start(t)
	hendra, kira := h.hendraAndKira()
	hendra.send(moveTo(outOfView, startY))
	kira.expect(protocol.EncodeCharacterDelete(1))
	hendra.expect(protocol.EncodeCharacterDelete(2))

	hendra.send(moveTo(startX+100, startY))

	kira.expectAppearanceAt(1, startX+100, startY)
	kira.expect(movedBy(1, startX+100, startY))
	hendra.expectAppearanceAt(2, startX, startY)
}

func TestTheEdgeOfViewIsStillInView(t *testing.T) {
	h := start(t)
	hendra, kira := h.hendraAndKira()

	kira.send(moveTo(edgeOfView, startY))

	hendra.expect(movedBy(2, edgeOfView, startY))
}

// The view is a circle: 6000 along each axis is about 8485 away, out of view, though
// neither distance alone reaches the edge.
func TestTheViewIsMeasuredInAStraightLine(t *testing.T) {
	h := start(t)
	hendra, kira := h.hendraAndKira()

	hendra.send(moveTo(startX+6000, startY+6000))

	kira.expect(protocol.EncodeCharacterDelete(1))
	hendra.expect(protocol.EncodeCharacterDelete(2))
}

// A player entering out of view of another hears nothing of them until they meet.
func TestPlayersOutOfViewMeetByWalking(t *testing.T) {
	h := start(t)
	hendra, _ := h.enterWorld("admin", "Hendra", 3, 6)
	hendra.send(moveTo(outOfView, startY))
	hendra.expectOpen()
	kira, _ := h.enterWorld("bob", "Kira", 3, 0)
	kira.expectOpen()
	hendra.expectOpen()

	kira.send(moveTo(outOfView-1000, startY+500))

	kira.expectAppearanceAt(1, outOfView, startY)
	hendra.expectAppearanceAt(2, outOfView-1000, startY+500)
	hendra.expect(movedBy(2, outOfView-1000, startY+500))
}

func TestALineOutOfViewIsNotHeard(t *testing.T) {
	h := start(t)
	hendra, kira := h.hendraAndKira()
	hendra.send(moveTo(outOfView, startY))
	kira.expect(protocol.EncodeCharacterDelete(1))
	hendra.expect(protocol.EncodeCharacterDelete(2))

	hendra.say("hi")

	hendra.expect(protocol.EncodeChat(protocol.ChatNormal, 1, 3, "Hendra : hi"))
	kira.expectOpen()
}

// Leaving tells those who saw the player, and nobody else on the map.
func TestLeavingTellsOnlyThoseWhoSawThePlayer(t *testing.T) {
	h := start(t)
	hendra, _ := h.enterWorld("admin", "Hendra", 3, 6)
	sora, _ := h.enterWorld("carl", "Sora", 3, 0)
	sora.expectAppearanceAt(1, startX, startY)
	hendra.expectAppearanceAt(2, startX, startY)
	sora.send(moveTo(outOfView, startY))
	sora.expect(protocol.EncodeCharacterDelete(1))
	hendra.expect(protocol.EncodeCharacterDelete(2))
	kira, _ := h.enterWorld("bob", "Kira", 3, 0)
	kira.expectAppearanceAt(1, startX, startY)

	hendra.say("/logout")

	kira.expect(protocol.EncodeCharacterDelete(1))
	sora.expectOpen()
}
