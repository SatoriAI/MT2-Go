package item

import (
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/SatoriAI/MT2-Go/internal/account"
)

// rolls are n rolls of the kind's bonuses, from a fixed seed.
func rolls(t *testing.T, vnum uint32, fromStone bool, n int) []Bonuses {
	t.Helper()
	p, ok := Lookup(vnum)
	if !ok {
		t.Fatalf("no kind %d", vnum)
	}
	r := rand.New(rand.NewPCG(7, 7)) //nolint:gosec // a fixed seed, so each run rolls the same
	out := make([]Bonuses, n)
	for i := range out {
		out[i] = Roll(p, fromStone, r)
	}
	return out
}

// A stone's helmet carries one bonus half the time, two 35 times in 100 and three 15,
// each a different kind its place allows, at a value of its grades.
func TestAStonesGearCarriesOneToThreeBonusesItsPlaceAllows(t *testing.T) {
	const n = 20000
	var counts [4]int
	for _, b := range rolls(t, 12220, true, n) {
		counts[len(b)]++
		seen := map[account.Bonus]bool{}
		for _, r := range b {
			values := bonusValues[r.Kind]
			if !slices.Contains(allowed[Head], r.Kind) || seen[r.Kind] || !slices.Contains(values[:], r.Value) {
				t.Fatalf("a helmet carries %+v", b)
			}
			seen[r.Kind] = true
		}
	}
	for i, want := range map[int]int{1: 50, 2: 35, 3: 15} {
		if got := counts[i] * 100 / n; got < want-2 || got > want+2 {
			t.Errorf("%d bonuses %d times in 100, want about %d", i, got, want)
		}
	}
	if counts[0] != 0 {
		t.Errorf("%d pieces from a stone with none", counts[0])
	}
}

// Armour allows only two kinds, hit points and resistance, so it carries two where the
// roll comes up three.
func TestArmourCarriesAtMostTwoBonuses(t *testing.T) {
	two := 0
	for _, b := range rolls(t, 11240, true, 5000) {
		if len(b) > 2 {
			t.Fatalf("armour carries %+v", b)
		}
		if len(b) == 2 {
			two++
		}
	}
	if got := two * 100 / 5000; got < 47 || got > 53 {
		t.Errorf("two bonuses %d times in 100, want about 50, the 35 and the 15", got)
	}
}

// Gear below level 25 never rolls a bonus's top grade; gear from 25 sometimes does.
func TestTheTopGradeComesOnlyFromLevelTwentyFive(t *testing.T) {
	top := func(vnum uint32) int {
		n := 0
		for _, b := range rolls(t, vnum, true, 5000) {
			for _, r := range b {
				if r.Value == bonusValues[r.Kind][4] {
					n++
				}
			}
		}
		return n
	}
	if got := top(11220); got != 0 { // armour of level 18
		t.Errorf("level 18 rolled the top grade %d times", got)
	}
	if got := top(11240); got == 0 { // armour of level 34
		t.Error("level 34 never rolled the top grade")
	}
}

// Nothing but a stone gives an ordinary bonus, and what is not worn carries none.
func TestOnlyAStonesDropRollsOrdinaryBonuses(t *testing.T) {
	for _, b := range rolls(t, 11240, false, 1000) {
		if len(b) != 0 {
			t.Fatalf("armour not from a stone carries %+v", b)
		}
	}
	for _, b := range rolls(t, 27001, true, 100) {
		if len(b) != 0 {
			t.Fatalf("a potion carries %+v", b)
		}
	}
}

// A level-30 or level-36 weapon from anything carries the pair 15 times in 100: average or
// skill damage at its strong value, and the other always taken off at the same grade's
// weak one. Other weapons never do.
func TestTheBestWeaponsSometimesCarryThePair(t *testing.T) {
	const n = 20000
	pairs := 0
	for _, b := range rolls(t, 290, false, n) {
		if len(b) == 0 {
			continue
		}
		pairs++
		if len(b) != 2 || b[0].Kind == b[1].Kind || b[0].Value <= 0 || b[1].Value >= 0 {
			t.Fatalf("the pair %+v", b)
		}
		strong := pairStrong[b[0].Kind]
		g := slices.Index(strong[:], b[0].Value)
		if g < 0 || b[1].Value != -pairWeak[g] || !slices.Contains([]account.Bonus{account.BonusAverage, account.BonusSkill}, b[1].Kind) {
			t.Fatalf("the pair %+v", b)
		}
	}
	if got := pairs * 100 / n; got < 13 || got > 17 {
		t.Errorf("the pair %d times in 100, want about 15", got)
	}
	for _, b := range rolls(t, 60, false, 2000) { // a sword of level 25
		if len(b) != 0 {
			t.Fatalf("a level-25 sword carries %+v", b)
		}
	}
}

// Three ordinary bonuses, or any at its top grade, are worth telling everyone of; a pair
// and one bonus at lower grades are not.
func TestWhatIsRemarkable(t *testing.T) {
	for b, want := range map[*Bonuses]bool{
		{{account.BonusCritical, 1}, {account.BonusPierce, 1}, {account.BonusAnimals, 2}}: true,
		{{account.BonusCritical, 8}}:                                                       true,
		{{account.BonusAverage, 30}, {account.BonusSkill, -15}}:                            true,
		{{account.BonusCritical, 1}, {account.BonusAverage, 10}, {account.BonusSkill, -5}}: false,
		{{account.BonusMaxHP, 280}}:                                                        false,
	} {
		if got := b.Remarkable(); got != want {
			t.Errorf("%+v: %v, want %v", *b, got, want)
		}
	}
}

// The totals of what is worn are summed, a pair's weak side taking off, and only then cut
// to their caps.
func TestTotalsAreSummedThenCapped(t *testing.T) {
	var sum account.Totals
	Bonuses{{account.BonusCritical, 8}, {account.BonusAverage, 30}, {account.BonusSkill, -15}}.Sum(&sum)
	Bonuses{{account.BonusCritical, 8}, {account.BonusMaxHP, 400}}.Sum(&sum)
	Bonuses{{account.BonusCritical, 8}, {account.BonusMaxHP, 400}}.Sum(&sum)
	got := sum.Capped()
	if got != (account.Totals{MaxHP: 800, Critical: 20, Average: 30, Skill: -15}) {
		t.Fatalf("totals %+v", got)
	}
}
