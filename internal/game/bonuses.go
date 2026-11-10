package game

import (
	"fmt"

	"github.com/SatoriAI/MT2-Go/internal/account"
)

// bonusLines are the info lines /bonusy answers with: each total of what the player wears,
// in the client's tooltip words, or a line saying there are none.
func bonusLines(t account.Totals) [][]byte {
	var lines [][]byte
	for _, b := range []struct {
		value  int
		format string
	}{
		{t.Average, "Średnie Obrażenia: %+d%%"},
		{t.Skill, "Obrażenie Umiejętności: %+d%%"},
		{t.Critical, "Szansa na cios krytyczny: %d%%"},
		{t.Pierce, "Szansa na przeszywające Uderzenie: %d%%"},
		{t.Animals, "Silny przeciwko Zwierzętom: +%d%%"},
		{t.AttackSpeed, "Szybkość ataku: +%d%%"},
		{t.MaxHP, "Maks. PŻ: +%d"},
		{t.Resist, "Odporność na Obrażenia: %d%%"},
	} {
		if b.value != 0 {
			lines = append(lines, infoLine(fmt.Sprintf(b.format, b.value)))
		}
	}
	if len(lines) == 0 {
		return [][]byte{infoLine("Nie nosisz nic z bonusami.")}
	}
	return append([][]byte{infoLine("Bonusy tego, co nosisz:")}, lines...)
}

// totalsOf is the bonuses of what the player wears.
func (w *record) totalsOf(vid uint32) account.Totals {
	l, unlock := w.lock()
	defer unlock()
	return l.all[vid].char.Totals
}
