package game

import (
	"context"
	"slices"
	"strings"

	"github.com/SatoriAI/MT2-Go/internal/item"
)

const (
	enchantNothing = "Ten przedmiot nie ma bonusów, które można zmienić."
	enchantDone    = "Bonusy przedmiotu zostały zmienione."
)

// offerEnchant asks before Zaczarowanie Przedmiotu, dropped onto gear in the bag, rolls its
// bonuses afresh: the client sends the drop without asking, so the player is shown the
// bonuses they would lose. Gear with no ordinary bonus, worn gear, or anything else is
// told why not, the scroll kept.
func (s *session) offerEnchant(scroll, it item.Item, held bool) error {
	kind, _ := item.Lookup(it.VNum)
	if !held || it.Worn != "" || kind.Wear == "" || it.Bonuses.Ordinary() == 0 {
		return s.conn.Send(infoLine(enchantNothing))
	}
	var now []string
	for _, b := range it.Bonuses {
		now = append(now, bonusText(b.Kind, int(b.Value)))
	}
	s.talking = &talk{enchant: scroll.ID, enchanted: it.ID}
	return s.conn.Send(script("Czy chcesz zmienić bonusy przedmiotu " + kind.Name + "?[ENTER]" + strings.Join(now, "[ENTER]") +
		"[ENTER]Zwykłe bonusy zostaną zastąpione nowymi, a para zostanie.[ENTER][QUESTION 1;Tak|2;Nie]"))
}

// enchant carries out a yes to offerEnchant, if the player still carries the scroll and
// the gear, unworn, in the bag, and is not trading: the gear's ordinary bonuses rolled
// afresh and one scroll spent, kept at once, then shown. What a trade the other side kept
// since moved is taken first, as the answer is no packet that takes it; a trade still open
// refuses it, or the gear offered could change under the other side's eyes. A write that
// fails changes nothing.
func (s *session) enchant(ctx context.Context, t *talk, answer uint8) error {
	trading, err := s.catchUp(ctx)
	if err != nil {
		return err
	}
	if trading && answer == 0 {
		return s.conn.Send(infoLine("Nie możesz tego zrobić podczas handlu."))
	}
	scroll, held := s.inventory.ByID(t.enchant)
	it, kept := s.inventory.ByID(t.enchanted)
	if answer != 0 || !held || !kept || it.Worn != "" {
		return nil
	}
	kind, _ := item.Lookup(it.VNum)
	it.Bonuses = item.Reroll(kind, it.Bonuses, s.rand)
	scroll.Count--
	ctx, cancel := context.WithTimeout(ctx, saveTimeout)
	defer cancel()
	if err := s.svc.items.Enchant(ctx, s.playing.ID, it, scroll); err != nil {
		s.conn.Log().Error("enchanting not kept", "item", it.ID, "err", err)
		return nil
	}
	s.inventory.Replace(it)
	told := [][]byte{itemSet(it)}
	if scroll.Count == 0 {
		s.inventory = slices.DeleteFunc(s.inventory, func(held item.Item) bool { return held.ID == scroll.ID })
		told = append(told, itemCleared(scroll))
	} else {
		s.inventory.Replace(scroll)
		told = append(told, itemSet(scroll))
	}
	s.conn.Log().Info("enchanted", "item", it.ID, "vnum", it.VNum, "bonuses", it.Bonuses)
	told = append(told, infoLine(enchantDone))
	return s.conn.Send(append(told, s.followTaskbar()...)...)
}
