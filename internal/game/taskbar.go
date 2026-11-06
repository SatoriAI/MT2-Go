package game

import (
	"context"

	"github.com/SatoriAI/MT2-Go/internal/item"
	"github.com/SatoriAI/MT2-Go/internal/protocol"
)

// Taskbar is where the game port keeps what each taskbar slot holds: an item, by its id,
// which the slot follows as it moves and loses when it goes.
type Taskbar interface {
	Slots(ctx context.Context, characterID uint32) (map[uint8]int64, error)
	Set(ctx context.Context, characterID uint32, slot uint8, itemID int64) error
	Clear(ctx context.Context, characterID uint32, slot uint8) error
	Swap(ctx context.Context, characterID uint32, a, b uint8) error
}

// taskbarSlots is how many slots the client keeps: four pages of eight it shows, and four
// more.
const taskbarSlots = 36

// shownSlot is what a taskbar slot holds and the cell the client was last told it is at.
type shownSlot struct {
	item int64
	cell uint16
}

// loadTaskbar reads what the slots hold, each at its item's cell now. Called once the
// inventory is read.
func (s *session) loadTaskbar(ctx context.Context, characterID uint32) error {
	slots, err := s.svc.taskbar.Slots(ctx, characterID)
	if err != nil {
		return err
	}
	s.taskbar = map[uint8]shownSlot{}
	for slot, id := range slots {
		if it, ok := s.inventory.ByID(id); ok {
			s.taskbar[slot] = shownSlot{id, it.Cell}
		}
	}
	return nil
}

// taskbarShown fills the taskbar as the player enters, a slot to a packet.
func (s *session) taskbarShown() [][]byte {
	packets := make([][]byte, 0, len(s.taskbar))
	for slot, held := range s.taskbar {
		packets = append(packets, slotFilled(slot, held.cell))
	}
	return packets
}

// onQuickslotAdd fills a slot with the item at a bag cell, if its kind is drunk, which is
// all the client offers the taskbar from the bag. A skill or an emotion is not kept yet.
func (s *session) onQuickslotAdd(body []byte) error {
	q, err := protocol.DecodeQuickslotAdd(body)
	if err != nil {
		return err
	}
	it, _ := s.inventory.At(uint16(q.Position)) // an empty cell is the zero item, which restores nothing
	kind, _ := item.Lookup(it.VNum)
	if q.Type != taskbarItem || q.Slot >= taskbarSlots || (kind.HP == 0 && kind.SP == 0) {
		s.conn.Log().Debug("taskbar slot not filled", "slot", q.Slot, "type", q.Type, "position", q.Position)
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), saveTimeout)
	defer cancel()
	if err := s.svc.taskbar.Set(ctx, s.playing.ID, q.Slot, it.ID); err != nil {
		s.conn.Log().Error("taskbar slot not kept", "slot", q.Slot, "err", err)
		return nil
	}
	s.taskbar[q.Slot] = shownSlot{it.ID, it.Cell}
	return s.conn.Send(slotFilled(q.Slot, it.Cell))
}

// onQuickslotDel empties a slot.
func (s *session) onQuickslotDel(body []byte) error {
	slot, err := protocol.DecodeQuickslotDel(body)
	if err != nil {
		return err
	}
	if slot >= taskbarSlots {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), saveTimeout)
	defer cancel()
	if err := s.svc.taskbar.Clear(ctx, s.playing.ID, slot); err != nil {
		s.conn.Log().Error("taskbar slot not cleared", "slot", slot, "err", err)
		return nil
	}
	delete(s.taskbar, slot)
	return s.conn.Send(protocol.EncodeQuickslotDel(slot))
}

// onQuickslotSwap swaps two slots, either of which may be empty.
func (s *session) onQuickslotSwap(body []byte) error {
	a, b, err := protocol.DecodeQuickslotSwap(body)
	if err != nil {
		return err
	}
	if a >= taskbarSlots || b >= taskbarSlots || a == b {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), saveTimeout)
	defer cancel()
	if err := s.svc.taskbar.Swap(ctx, s.playing.ID, a, b); err != nil {
		s.conn.Log().Error("taskbar slots not swapped", "slots", []uint8{a, b}, "err", err)
		return nil
	}
	heldA, okA := s.taskbar[a]
	heldB, okB := s.taskbar[b]
	delete(s.taskbar, a)
	delete(s.taskbar, b)
	if okA {
		s.taskbar[b] = heldA
	}
	if okB {
		s.taskbar[a] = heldB
	}
	return s.conn.Send(protocol.EncodeQuickslotSwap(a, b))
}

// followTaskbar keeps the client's taskbar true after the inventory changed: a slot whose
// item is gone is emptied, the database having let it go with the item, and one whose
// item moved is shown at its new cell.
func (s *session) followTaskbar() [][]byte {
	var packets [][]byte
	for slot, held := range s.taskbar {
		it, ok := s.inventory.ByID(held.item)
		switch {
		case !ok:
			delete(s.taskbar, slot)
			packets = append(packets, protocol.EncodeQuickslotDel(slot))
		case it.Cell != held.cell:
			s.taskbar[slot] = shownSlot{held.item, it.Cell}
			packets = append(packets, slotFilled(slot, it.Cell))
		}
	}
	return packets
}
