package game

import (
	"testing"

	"github.com/SatoriAI/MT2-Go/internal/account"
)

func TestJobPutsTheFemaleClassesFourAfterTheMale(t *testing.T) {
	cases := []struct {
		class account.Class
		sex   account.Sex
		want  uint8
	}{
		{account.Warrior, account.Male, 0},
		{account.Shaman, account.Male, 3},
		{account.Warrior, account.Female, 4},
		{account.Sura, account.Female, 6},
	}
	for _, c := range cases {
		if got := job(c.class, c.sex); got != c.want {
			t.Errorf("%s %s: got %d, want %d", c.sex, c.class, got, c.want)
		}
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
