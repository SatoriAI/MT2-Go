package game

import (
	"testing"

	"github.com/SatoriAI/MT2-Go/internal/account"
)

// The numbers are the client's RACE_* constants in playersettingmodule.py.
func TestEveryRaceNumberIsTheClientsAndLeadsBack(t *testing.T) {
	cases := []struct {
		class account.Class
		sex   account.Sex
		want  uint8
	}{
		{account.Warrior, account.Male, 0},
		{account.Ninja, account.Female, 1},
		{account.Sura, account.Male, 2},
		{account.Shaman, account.Female, 3},
		{account.Warrior, account.Female, 4},
		{account.Ninja, account.Male, 5},
		{account.Sura, account.Female, 6},
		{account.Shaman, account.Male, 7},
	}
	for _, c := range cases {
		if got := raceNumber(c.class, c.sex); got != c.want {
			t.Errorf("%s %s: got %d, want %d", c.sex, c.class, got, c.want)
		}
		if class, sex, ok := raceFromNumber(uint16(c.want)); !ok || class != c.class || sex != c.sex {
			t.Errorf("%d: got %s %s", c.want, sex, class)
		}
	}
	if _, _, ok := raceFromNumber(8); ok {
		t.Error("8 is not a race")
	}
}

func TestASkillGroupIsNumberedByItsPlaceInTheClass(t *testing.T) {
	if got := skillGroupNumber(account.Ninja, "archery"); got != 2 {
		t.Errorf("archery: got %d", got)
	}
	if got := skillGroupNumber(account.Ninja, account.SkillGroup("")); got != 0 {
		t.Errorf("none chosen: got %d", got)
	}
}

func TestEveryKingdomNumberLeadsBackToItsKingdom(t *testing.T) {
	for empire, number := range empireNumbers {
		if got, ok := empireFromNumber(number); !ok || got != empire {
			t.Errorf("%d: got %q", number, got)
		}
	}
	if _, ok := empireFromNumber(0); ok {
		t.Error("0 is not a kingdom")
	}
}
