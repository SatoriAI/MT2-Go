package server

import (
	"context"
	"encoding/binary"
	"testing"
	"time"

	"github.com/SatoriAI/MT2-Go/internal/protocol"
)

// expectPointChange reads a POINT_CHANGE for vid and checks its point and value.
func (c *client) expectPointChange(vid uint32, point protocol.Point, value int32) {
	c.t.Helper()
	r := protocol.NewReader(c.read(17)[4:])
	if v, p, _, got := r.U32(), protocol.Point(r.U8()), r.I32(), r.I32(); v != vid || p != point || got != value {
		c.t.Fatalf("got point %d of %d at %d, want point %d of %d at %d", p, v, got, point, vid, value)
	}
}

// characterValue reads one of Hendra's stored values.
func (h *harness) characterValue(column string) int {
	h.t.Helper()
	var v int
	if err := h.accounts.Pool.QueryRow(context.Background(),
		`SELECT `+column+` FROM characters WHERE name = 'Hendra'`).Scan(&v); err != nil {
		h.t.Fatal(err)
	}
	return v
}

// Hendra, a level-1 sura, beats a wild dog to death and gets its 15 experience, which is
// kept when she changes character. The dog is the world's first character, so she is vid 2.
func TestKillingADogEarnsItsExperienceAndKeepsIt(t *testing.T) {
	h := startWith(t, aDogAtTheStart)
	hendra, _ := h.enterWorld("admin", "Hendra", 3, 6)
	hendra.read(35) // the dog

	for taken := int32(0); taken < 126; time.Sleep(110 * time.Millisecond) {
		hendra.hitTheDog()
		taken += hendra.expectDamageOnTheDog()
	}
	hendra.expect(protocol.EncodeDead(dogVID))
	hendra.expectPointChange(2, protocol.PointExp, 15)
	hendra.say("/phase_select")
	hendra.expect(protocol.EncodePhase(protocol.PhaseSelect)) // sent once the character is saved

	if got := h.characterValue("exp"); got != 15 {
		t.Fatalf("stored %d exp, want 15", got)
	}
}

// The character window's + button spends a stat point: the stat goes up, the points
// down, and both are kept.
func TestTheStatButtonSpendsAPointAndKeepsIt(t *testing.T) {
	h := start(t)
	hendra, key := h.hendraInWorld()
	hendra.say("/phase_select")
	hendra.expect(protocol.EncodePhase(protocol.PhaseSelect))
	h.exec(`UPDATE characters SET stat_points = 2 WHERE name = 'Hendra'`)
	hendra = h.reenterHendra(key)
	hendra.read(noBlocksSize)

	hendra.say("/stat st")

	hendra.expectPointChange(2, protocol.PointST, 6) // a sura starts with strength 5
	hendra.expectPointChange(2, protocol.PointStatPoints, 1)
	hendra.say("/phase_select")
	hendra.expect(protocol.EncodePhase(protocol.PhaseSelect))
	h.reenterHendra(key)
	if st, points := h.characterValue("strength"), h.characterValue("stat_points"); st != 6 || points != 1 {
		t.Fatalf("stored strength %d and %d points, want 6 and 1", st, points)
	}
}

// Without points the button does nothing.
func TestTheStatButtonWithoutPointsDoesNothing(t *testing.T) {
	h := start(t)
	game := h.inWorld()

	game.say("/stat st")

	game.expectOpen()
}

// Entering again, the character window gets the experience and unspent stat points kept,
// so the + buttons show.
func TestEnteringShowsTheExperienceAndStatPointsKept(t *testing.T) {
	h := start(t)
	hendra, key := h.hendraInWorld()
	hendra.say("/phase_select")
	hendra.expect(protocol.EncodePhase(protocol.PhaseSelect))
	h.exec(`UPDATE characters SET exp = 120, stat_points = 4 WHERE name = 'Hendra'`)

	game := h.enterSelection("admin", key)
	game.read(2 + 329 + 2) // kingdom, list, select phase
	game.selectCharacter(0)
	game.expect(protocol.EncodePhase(protocol.PhaseLoading))
	game.read(46) // the main character
	points := game.read(1021)[1:]

	if exp, statPoints := binary.LittleEndian.Uint32(points[4*protocol.PointExp:]),
		binary.LittleEndian.Uint32(points[4*protocol.PointStatPoints:]); exp != 120 || statPoints != 4 {
		t.Fatalf("shown %d exp and %d stat points, want 120 and 4", exp, statPoints)
	}
}
