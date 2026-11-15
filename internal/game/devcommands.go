package game

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/SatoriAI/MT2-Go/internal/item"
)

// onDevCommand carries out a command that hands a tester what they need, answered only
// with the development switch on (docs/mechanics/chat.md), and says whether name was one.
func (s *session) onDevCommand(name, arg string) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), saveTimeout)
	defer cancel()
	switch name {
	case "getitem":
		return true, s.getItem(ctx, arg)
	case "getyang":
		return true, s.getYang(ctx, arg)
	}
	return false, nil
}

// getItemUsage is how /getitem is written.
const getItemUsage = "Użycie: /getitem VNUM [ilość] [bonusy]"

// getItem puts count, 1 if not given and at most a stack, of the kind vnum into the bag,
// where a pick-up would put it; with "bonusy" last, gear carrying bonuses rolled as a
// stone's are, so what bonuses do can be tried without a stone.
func (s *session) getItem(ctx context.Context, arg string) error {
	fields := strings.Fields(arg)
	bonused := len(fields) > 1 && fields[len(fields)-1] == "bonusy"
	if bonused {
		fields = fields[:len(fields)-1]
	}
	if len(fields) < 1 || len(fields) > 2 {
		return s.conn.Send(infoLine(getItemUsage))
	}
	vnum, err := strconv.ParseUint(fields[0], 10, 32)
	count := uint64(1)
	if err == nil && len(fields) == 2 {
		count, err = strconv.ParseUint(fields[1], 10, 32)
	}
	if err != nil || count == 0 {
		return s.conn.Send(infoLine(getItemUsage))
	}
	kind, known := item.Lookup(uint32(vnum))
	if !known {
		return s.conn.Send(infoLine(fmt.Sprintf("Serwer nie zna przedmiotu %d.", vnum)))
	}
	var bonuses item.Bonuses
	if bonused {
		if bonuses = item.Roll(kind, true, s.rand); len(bonuses) == 0 {
			return s.conn.Send(infoLine("Ten przedmiot nie może mieć bonusów."))
		}
	}
	given := uint8(min(count, uint64(kind.Stack))) //nolint:gosec // at most a stack, a uint8
	into, room := s.inventory.RoomFor(kind, given)
	if !room {
		return s.conn.Send(infoLine("Nie masz miejsca w ekwipunku."))
	}
	into.Bonuses = bonuses
	if into, err = s.receive(ctx, into); err != nil {
		s.conn.Log().Error("item given by command not kept", "vnum", vnum, "err", err)
		return nil
	}
	s.conn.Log().Info("item given by command", "character", s.playing.Name, "vnum", vnum, "count", given, "bonuses", into.Bonuses)
	return s.conn.Send(itemSet(into))
}

// getYang gives the player amount yang, up to what a character may hold.
func (s *session) getYang(ctx context.Context, arg string) error {
	amount, err := strconv.ParseUint(strings.TrimSpace(arg), 10, 32)
	if err != nil || amount == 0 {
		return s.conn.Send(infoLine("Użycie: /getyang ilość"))
	}
	if err := s.catchUpYang(ctx); err != nil {
		return err
	}
	if uint64(s.gold)+amount > maxGold {
		return s.conn.Send(infoLine(fmt.Sprintf("Tyle się nie zmieści: postać może mieć najwyżej %d Yang.", maxGold)))
	}
	gold, err := s.svc.items.AddGold(ctx, s.playing.ID, uint32(amount))
	if err != nil {
		s.conn.Log().Error("yang given by command not kept", "amount", amount, "err", err)
		return nil
	}
	s.gold = gold
	s.conn.Log().Info("yang given by command", "character", s.playing.Name, "amount", amount)
	return s.conn.Send(s.yangChanged(int64(amount)))
}
