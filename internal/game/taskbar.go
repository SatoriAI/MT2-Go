package game

import (
	"context"

	"github.com/SatoriAI/MT2-Go/internal/account"
	"github.com/SatoriAI/MT2-Go/internal/item"
	"github.com/SatoriAI/MT2-Go/internal/protocol"
)

// Taskbar is where the game port keeps what each taskbar slot holds: an item, by its id,
// which the slot follows as it moves and loses when it goes, or a skill learnt, which it
// loses when the skill is forgotten.
type Taskbar interface {
	Slots(ctx context.Context, characterID uint32) (map[uint8]account.Shortcut, error)
	Set(ctx context.Context, characterID uint32, slot uint8, held account.Shortcut) error
	Clear(ctx context.Context, characterID uint32, slot uint8) error
	Swap(ctx context.Context, characterID uint32, a, b uint8) error
}

// taskbarSlots is how many slots the client keeps: four pages of eight it shows, and four
// more.
const taskbarSlots = 36

// shownSlot is what a taskbar slot holds and, for an item, the cell the client was last
// told it is at.
type shownSlot struct {
	held account.Shortcut
	cell uint16
}

// loadTaskbar reads what the slots hold: each item at its cell now, and each skill. Called
// once the inventory is read.
func (s *session) loadTaskbar(ctx context.Context, characterID uint32) error {
	slots, err := s.svc.taskbar.Slots(ctx, characterID)
	if err != nil {
		return err
	}
	s.taskbar = map[uint8]shownSlot{}
	for slot, held := range slots {
		if it, ok := s.inventory.ByID(held.Item); ok || held.Skill != "" {
			s.taskbar[slot] = shownSlot{held, it.Cell}
		}
	}
	return nil
}

// taskbarShown fills the taskbar as the player enters, a slot to a packet.
func (s *session) taskbarShown() [][]byte {
	packets := make([][]byte, 0, len(s.taskbar))
	for slot, shown := range s.taskbar {
		packets = append(packets, slotShown(slot, shown))
	}
	return packets
}

// onQuickslotAdd fills a slot with the item at a bag cell, if its kind is drunk, which is
// all the client offers the taskbar from the bag, or with a skill the player has learnt,
// by its place in the skill window. An emotion is not kept yet.
func (s *session) onQuickslotAdd(body []byte) error {
	q, err := protocol.DecodeQuickslotAdd(body)
	if err != nil {
		return err
	}
	shown, ok := s.shortcutFor(q)
	if !ok || q.Slot >= taskbarSlots {
		s.conn.Log().Debug("taskbar slot not filled", "slot", q.Slot, "type", q.Type, "position", q.Position)
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), saveTimeout)
	defer cancel()
	if err := s.svc.taskbar.Set(ctx, s.playing.ID, q.Slot, shown.held); err != nil {
		s.conn.Log().Error("taskbar slot not kept", "slot", q.Slot, "err", err)
		return nil
	}
	s.taskbar[q.Slot] = shown
	return s.conn.Send(slotShown(q.Slot, shown))
}

// shortcutFor is what the client asks a slot to hold: a potion in the bag, or a skill
// learnt.
func (s *session) shortcutFor(q protocol.QuickslotAdd) (shownSlot, bool) {
	switch q.Type {
	case taskbarItem:
		it, _ := s.inventory.At(uint16(q.Position)) // an empty cell is the zero item, which restores nothing
		kind, _ := item.Lookup(it.VNum)
		return shownSlot{account.Shortcut{Item: it.ID}, it.Cell}, kind.HP > 0 || kind.SP > 0
	case taskbarSkill:
		_, group := s.svc.presence.trainee(s.playing.vid)
		sk, known := skillByWindowSlot(group, q.Position)
		return shownSlot{held: account.Shortcut{Skill: sk}}, known && s.svc.presence.learnt(s.playing.vid, sk)
	}
	return shownSlot{}, false
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
// item moved is shown at its new cell. A skill's slot follows no item.
func (s *session) followTaskbar() [][]byte {
	var packets [][]byte
	for slot, shown := range s.taskbar {
		if shown.held.Skill != "" {
			continue
		}
		it, ok := s.inventory.ByID(shown.held.Item)
		switch {
		case !ok:
			delete(s.taskbar, slot)
			packets = append(packets, protocol.EncodeQuickslotDel(slot))
		case it.Cell != shown.cell:
			s.taskbar[slot] = shownSlot{shown.held, it.Cell}
			packets = append(packets, slotShown(slot, s.taskbar[slot]))
		}
	}
	return packets
}

// forgetSkillSlots empties the slots of the skills forgotten on a change of skill group,
// which the database let go with them.
func (s *session) forgetSkillSlots() [][]byte {
	var packets [][]byte
	for slot, shown := range s.taskbar {
		if shown.held.Skill != "" {
			delete(s.taskbar, slot)
			packets = append(packets, protocol.EncodeQuickslotDel(slot))
		}
	}
	return packets
}

// learnt is whether the player has learnt the skill.
func (w *presence) learnt(vid uint32, sk account.Skill) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.all[vid].char.Skills[sk] > 0
}
