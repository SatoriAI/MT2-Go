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
		kind  account.Bonus
		value int
	}{
		{account.BonusAverage, t.Average}, {account.BonusSkill, t.Skill}, {account.BonusCritical, t.Critical},
		{account.BonusPierce, t.Pierce}, {account.BonusAnimals, t.Animals}, {account.BonusOrcs, t.Orcs},
		{account.BonusAttackSpeed, t.AttackSpeed}, {account.BonusMaxHP, t.MaxHP}, {account.BonusResist, t.Resist},
	} {
		if b.value != 0 {
			lines = append(lines, infoLine(bonusText(b.kind, b.value)))
		}
	}
	if len(lines) == 0 {
		return [][]byte{infoLine("Nie nosisz nic z bonusami.")}
	}
	return append([][]byte{infoLine("Bonusy tego, co nosisz:")}, lines...)
}

// bonusFormats are each bonus in the client's tooltip words.
var bonusFormats = map[account.Bonus]string{
	account.BonusAverage: "Średnie Obrażenia: %+d%%", account.BonusSkill: "Obrażenie Umiejętności: %+d%%",
	account.BonusCritical: "Szansa na cios krytyczny: %d%%", account.BonusPierce: "Szansa na przeszywające Uderzenie: %d%%",
	account.BonusAnimals: "Silny przeciwko Zwierzętom: +%d%%", account.BonusOrcs: "Silny przeciwko Orkom: +%d%%",
	account.BonusAttackSpeed: "Szybkość ataku: +%d%%", account.BonusMaxHP: "Maks. PŻ: +%d", account.BonusResist: "Odporność na Obrażenia: %d%%",
}

func bonusText(kind account.Bonus, value int) string { return fmt.Sprintf(bonusFormats[kind], value) }

// totalsOf is the bonuses of what the player wears.
func (w *record) totalsOf(vid uint32) account.Totals {
	l, unlock := w.lock()
	defer unlock()
	return l.all[vid].char.Totals
}
