package game

import (
	"context"
	"fmt"

	"github.com/SatoriAI/MT2-Go/internal/account"
	"github.com/SatoriAI/MT2-Go/internal/item"
	"github.com/SatoriAI/MT2-Go/internal/protocol"
	"github.com/SatoriAI/MT2-Go/internal/world"
)

// villages are the client's names for each kingdom's first village (locale_game.txt's
// MAP_A1, MAP_B1, MAP_C1), where the ring takes a player home.
var villages = map[account.Empire]string{
	account.EmpireShinsoo: "Strefa Yongan",
	account.EmpireChunjo:  "Strefa Joan",
	account.EmpireJinno:   "Strefa Pyungmoo",
}

const (
	ringRefused = "Nie możesz teraz użyć tego przedmiotu."
	ringTooLow  = "Masz za niski poziom, aby użyć tego przedmiotu."
	// valleyName is the client's MAP_A2; valleyLevel, the debaters', the level it opens at.
	valleyName  = "Dolina Seungryong"
	valleyLevel = 20
)

var valleyLow = fmt.Sprintf("%s jest dostępna od %d poziomu.", valleyName, valleyLevel)

// offerTeleport asks where the ring should take the player: their kingdom's village, the
// valley, with a word on how hard it is, or nowhere. The ring is kept whatever they
// answer: its 50 charges wait for items that keep a count of their own
// (docs/mechanics/teleporting.md).
func (s *session) offerTeleport(ring item.Item) error {
	if why := s.whyNotTeleport(); why != "" {
		return s.conn.Send(infoLine(why))
	}
	s.talking = &talk{ring: ring.ID}
	return s.conn.Send(script(fmt.Sprintf("Dokąd chcesz się przenieść?[ENTER]%s: zalecany poziom %d+, potwory są tam silniejsze."+
		"[ENTER][QUESTION 1;%s|2;%s|3;Anuluj]", valleyName, valleyLevel, villages[s.empire], valleyName)))
}

// teleport carries out the answer to offerTeleport: the first takes the player home and
// the second to the valley's arrival, from its level, if they still carry the ring and
// still may; the last cancels. What a trade the other side kept since moved is taken
// first, as the answer is no packet that takes it: it may have taken the ring.
func (s *session) teleport(ctx context.Context, t *talk, answer uint8) error {
	if _, err := s.catchUp(ctx); err != nil {
		return err
	}
	if _, held := s.inventory.ByID(t.ring); !held || answer > 1 {
		return nil
	}
	if why := s.whyNotTeleport(); why != "" {
		return s.conn.Send(infoLine(why))
	}
	if answer == 1 {
		valley, _ := world.MapNamed(world.Valley)
		if level, _ := s.svc.record.unbusy(s.playing.vid); level < valleyLevel {
			return s.conn.Send(infoLine(valleyLow))
		}
		return s.warp(valley.Arrival.X, valley.Arrival.Y)
	}
	x, y, _ := s.empire.StartPosition()
	return s.warp(x, y)
}

// whyNotTeleport is what the player is told instead of being taken anywhere, or "": not
// while dead, trading, or at a shop or the blacksmith, nor below the ring's level, nor
// with no kingdom to go home to.
func (s *session) whyNotTeleport() string {
	ring, _ := item.Lookup(item.TeleportRing)
	level, free := s.svc.record.unbusy(s.playing.vid)
	_, _, home := s.empire.StartPosition()
	switch {
	case !free || !home || s.shop != 0 || s.upgrading != nil:
		return ringRefused
	case level < ring.Level:
		return ringTooLow
	}
	return ""
}

// warp sends the player to (x, y), as the client goes anywhere: out of everyone's view and
// saved there, then told to connect again, which enters them anew where they were saved.
// Until the client lets go of this connection, what it still sends from the game is dropped.
func (s *session) warp(x, y int32) error {
	s.leaveBy(func(vid uint32) *present { return s.svc.record.leaveFor(vid, x, y) })
	s.leftWorld = true
	s.phase = protocol.PhaseSelect
	s.conn.Log().Info("warped", "x", x, "y", y)
	return s.conn.Send(protocol.EncodeWarp(x, y, s.svc.public))
}

// unbusy reports the player's level and whether they are alive and out of a trade.
func (w *record) unbusy(vid uint32) (uint8, bool) {
	l, unlock := w.lock()
	defer unlock()
	p := l.all[vid]
	return p.char.Level, !p.dead() && l.trades[vid] == nil
}

// leaveFor takes the player out of the world, telling those who saw them, and puts them at
// (x, y), where leaving saves them.
func (w *record) leaveFor(vid uint32, x, y int32) *present {
	l, unlock := w.lock()
	defer unlock()
	p := l.all[vid]
	l.remove(p)
	p.add.X, p.add.Y = x, y
	return p
}
