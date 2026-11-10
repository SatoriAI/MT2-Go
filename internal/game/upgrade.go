package game

import (
	"context"
	"slices"

	"github.com/SatoriAI/MT2-Go/internal/item"
	"github.com/SatoriAI/MT2-Go/internal/protocol"
	"github.com/SatoriAI/MT2-Go/internal/upgrade"
)

// upgrading is the item a player gave a blacksmith, whose refine dialog is open.
type upgrading struct {
	smith uint32 // the blacksmith's vid
	item  int64
}

// What a blacksmith says of an upgrade: the client's own lines for the outcome and for an
// item that is not upgraded, and this server's for what the player lacks.
const (
	upgradeNotPossible = "Ten Przedmiot nie może zostać ulepszony."
	upgradeSucceeded   = "Przedmiot został ulepszony."
	upgradeFailed      = "Ulepszanie nie powiodło się."
	upgradeNoYang      = "Nie masz dość Yang na ulepszenie."
	upgradeNoMaterial  = "Brakuje ci materiałów do ulepszenia."
)

// onGiveItem answers an item dragged onto an NPC near the player: a blacksmith given gear
// from the bag below its last grade opens the refine dialog with what upgrading it takes,
// and says so of anything else. Other NPCs take nothing yet. Any give closes a dialog
// open before it, as the client has moved on from it.
func (s *session) onGiveItem(body []byte) error {
	g, err := protocol.DecodeGiveItem(body)
	if err != nil {
		return err
	}
	s.upgrading = nil
	smith, near := s.svc.record.npcInReach(s.playing.vid, g.To)
	if !near || !smith.Upgrades {
		s.conn.Log().Debug("give not taken", "to", g.To)
		return nil
	}
	it, held := s.itemAt(g.Pos)
	kind, _ := item.Lookup(it.VNum)
	step, upgraded := upgrade.Of(kind)
	if !held || it.Worn != "" || !upgraded {
		return s.tell(upgradeNotPossible)
	}
	s.upgrading = &upgrading{smith: g.To, item: it.ID}
	info := protocol.RefineInfo{Type: protocol.RefineNoQuestion, Cell: uint8(it.Cell), From: it.VNum, Into: step.Into, //nolint:gosec // a bag cell, under 90
		Cost: step.Yang, Chance: step.Chance}
	if step.Material != 0 {
		info.Type = protocol.RefineMayDowngrade
		info.Materials = []protocol.Material{{VNum: step.Material, Count: uint32(step.Count)}}
	}
	return s.conn.Send(protocol.EncodeRefineInfo(info))
}

// onRefine carries out the upgrade the refine dialog asks for, of the item given the
// blacksmith, still in its cell and the blacksmith still in reach, if the player has its
// yang and materials: they are spent, and the item upgraded, or on a failure kept or a
// grade lower, all kept at once. Closing the dialog sends 255, which is no bag cell, so it
// does nothing, as does an answer to none open.
func (s *session) onRefine(ctx context.Context, body []byte) error {
	q, err := protocol.DecodeRefine(body)
	if err != nil {
		return err
	}
	u := s.upgrading
	s.upgrading = nil
	it, held := s.inventory.At(uint16(q.Cell))
	if u == nil || !held || it.ID != u.item {
		return nil
	}
	if _, near := s.svc.record.npcInReach(s.playing.vid, u.smith); !near {
		return nil
	}
	kind, _ := item.Lookup(it.VNum)
	step, _ := upgrade.Of(kind)
	spent, enough := materialsFor(s.inventory, step)
	switch {
	case s.gold < step.Yang:
		return s.conn.Send(infoLine(upgradeNoYang))
	case !enough:
		return s.conn.Send(infoLine(upgradeNoMaterial))
	}
	upgraded, said := it, upgradeSucceeded
	upgraded.VNum = step.Into
	if s.rand.IntN(100) >= int(step.Chance) {
		upgraded.VNum, said = step.Failed(), upgradeFailed
	}
	ctx, cancel := context.WithTimeout(ctx, saveTimeout)
	defer cancel()
	gold, err := s.svc.items.Upgrade(ctx, s.playing.ID, step.Yang, upgraded, spent)
	if err != nil {
		s.conn.Log().Error("upgrade not kept", "item", it.ID, "err", err)
		return nil
	}
	s.gold = gold
	s.inventory.Replace(upgraded)
	told := [][]byte{itemSet(upgraded)}
	for _, m := range spent {
		if m.Count == 0 {
			s.inventory = slices.DeleteFunc(s.inventory, func(held item.Item) bool { return held.ID == m.ID })
			told = append(told, itemCleared(m))
		} else {
			s.inventory.Replace(m)
			told = append(told, itemSet(m))
		}
	}
	told = append(told, s.yangChanged(-int64(step.Yang)), infoLine(said))
	return s.conn.Send(append(told, s.followTaskbar()...)...)
}

// materialsFor is the stacks of material in the bag the upgrade spends, each at what is
// left of it, taken from the first stacks found, and whether the bag holds enough.
func materialsFor(inv item.Inventory, step upgrade.Step) ([]item.Item, bool) {
	need := step.Count
	var spent []item.Item
	for _, it := range inv {
		if it.VNum != step.Material || need == 0 {
			continue
		}
		taken := min(need, it.Count)
		it.Count -= taken
		need -= taken
		spent = append(spent, it)
	}
	return spent, need == 0
}
