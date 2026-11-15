package game

import (
	"context"
	"fmt"

	"github.com/SatoriAI/MT2-Go/internal/world"
)

// What the gates and the teleporter say, and the teleporter's price, the veteran's: a
// service, not a taxi, a level-20 player's few kills (docs/mechanics/maps.md).
const (
	gateClosed      = "Ta droga jest jeszcze zamknięta."
	gateBusy        = "Nie możesz teraz przejść przez bramę."
	teleporterPrice = 1500
	teleporterBroke = "Nie masz dość Yang na podróż."
	teleporterBusy  = "Nie mogę cię teraz zabrać."
)

var teleporterOffer = fmt.Sprintf("Mogę cię zabrać do krainy orków.[ENTER]%s: orki od 26 do 49 poziomu.[ENTER]"+
	"[QUESTION 1;Jedź (%d Yang)|2;Anuluj]", valleyName, teleporterPrice)

// passGate takes the player through the gate they have walked into, once until they walk
// out of it: into the valley from a first village's gate to it, from its level; home from
// the valley's southern gate, a step inside their kingdom's gate to it; nowhere through a
// closed one, which says so. Dead, trading, or at a shop or the blacksmith, they are told.
func (s *session) passGate() error {
	p, in := world.PortalAt(s.playing.X, s.playing.Y)
	if !in {
		s.gate = world.Spot{}
		return nil
	}
	if p.At == s.gate {
		return nil
	}
	s.gate = p.At
	level, free := s.svc.record.unbusy(s.playing.vid)
	switch {
	case p.Way == world.Closed:
		return s.conn.Send(infoLine(gateClosed))
	case !free || s.shop != 0 || s.upgrading != nil:
		return s.conn.Send(infoLine(gateBusy))
	case p.Way == world.ToValley && level < valleyLevel:
		return s.conn.Send(infoLine(valleyLow))
	case p.Way == world.ToValley:
		valley, _ := world.MapNamed(world.Valley)
		return s.warp(valley.Arrival.X, valley.Arrival.Y)
	}
	x, y, _ := s.empire.StartPosition()
	home, _ := world.HomeGate(x, y)
	return s.warp(home.X, home.Y)
}

// offerTeleporter shows the teleporter's offer: the valley, what lives there, and the price.
func (s *session) offerTeleporter(vid uint32) error {
	s.talking = &talk{teleporter: vid}
	return s.conn.Send(script(teleporterOffer))
}

// takeTeleporter carries out a yes to the teleporter, still in reach: from the valley's
// level, while free, and for its yang, taken and kept, the player is taken to the valley's
// arrival, where they enter with the yang left; otherwise they are told why. What a trade
// the other side kept since moved is taken first.
func (s *session) takeTeleporter(ctx context.Context, t *talk, answer uint8) error {
	if _, err := s.catchUp(ctx); err != nil {
		return err
	}
	if _, near := s.svc.record.npcInReach(s.playing.vid, t.teleporter); answer != 0 || !near {
		return nil
	}
	level, free := s.svc.record.unbusy(s.playing.vid)
	switch {
	case !free || s.shop != 0 || s.upgrading != nil:
		return s.tell(teleporterBusy)
	case level < valleyLevel:
		return s.tell(valleyLow)
	case s.gold < teleporterPrice:
		return s.tell(teleporterBroke)
	}
	ctx, cancel := context.WithTimeout(ctx, saveTimeout)
	defer cancel()
	gold, err := s.svc.items.SpendGold(ctx, s.playing.ID, teleporterPrice)
	if err != nil {
		s.conn.Log().Error("teleporter's price not kept", "err", err)
		return nil
	}
	s.gold = gold
	valley, _ := world.MapNamed(world.Valley)
	return s.warp(valley.Arrival.X, valley.Arrival.Y)
}
