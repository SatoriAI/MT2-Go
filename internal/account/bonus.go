package account

// Bonus is a kind of bonus an item may carry (docs/mechanics/bonuses.md), stored by name.
type Bonus string

const (
	BonusMaxHP       Bonus = "max_hp"          // Maks. PŻ, in hit points
	BonusAttackSpeed Bonus = "attack_speed"    // Szybkość ataku
	BonusCritical    Bonus = "critical"        // Szansa na cios krytyczny, in percent
	BonusPierce      Bonus = "pierce"          // Szansa na przeszywające uderzenie, in percent
	BonusAnimals     Bonus = "against_animals" // Silny przeciwko Zwierzętom, in percent
	BonusAverage     Bonus = "average_damage"  // Średnie Obrażenia, in percent
	BonusSkill       Bonus = "skill_damage"    // Obrażenie Umiejętności, in percent
	BonusResist      Bonus = "average_resist"  // Odporność na Obrażenia, in percent
)

// Totals are the bonuses of everything a character wears, summed and capped: what their
// blows, skills, bites taken, hit points and speed count.
type Totals struct {
	MaxHP, AttackSpeed, Critical, Pierce, Animals, Average, Skill, Resist int
}

// caps are the most each total counts, the designer's, so a full set never trivialises
// the content up to level 36. Hit points have none.
var caps = Totals{AttackSpeed: 20, Critical: 20, Pierce: 20, Animals: 25, Average: 30, Skill: 30, Resist: 15}

// Add counts value of the bonus into the totals.
func (t *Totals) Add(b Bonus, value int) {
	switch b {
	case BonusMaxHP:
		t.MaxHP += value
	case BonusAttackSpeed:
		t.AttackSpeed += value
	case BonusCritical:
		t.Critical += value
	case BonusPierce:
		t.Pierce += value
	case BonusAnimals:
		t.Animals += value
	case BonusAverage:
		t.Average += value
	case BonusSkill:
		t.Skill += value
	case BonusResist:
		t.Resist += value
	}
}

// Capped is the totals with each cut to its cap, once everything worn is counted: a
// negative bonus, half of a weapon's pair, takes off before the cap, not after.
func (t Totals) Capped() Totals {
	return Totals{MaxHP: t.MaxHP, AttackSpeed: min(t.AttackSpeed, caps.AttackSpeed), Critical: min(t.Critical, caps.Critical),
		Pierce: min(t.Pierce, caps.Pierce), Animals: min(t.Animals, caps.Animals), Average: min(t.Average, caps.Average),
		Skill: min(t.Skill, caps.Skill), Resist: min(t.Resist, caps.Resist)}
}
