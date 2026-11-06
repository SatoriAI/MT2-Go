package skill

import (
	"slices"
	"testing"
	"time"

	"github.com/SatoriAI/MT2-Go/internal/account"
)

// Every skill is of a class and one of that class's own skill groups.
func TestEverySkillIsOfAGroupOfItsClass(t *testing.T) {
	for s, p := range protos {
		if groups := p.Class.SkillGroups(); !slices.Contains(groups[:], p.Group) {
			t.Errorf("%s: %+v", s, p)
		}
	}
}

// A character earns a point at level 5 and one more each level, less the levels learnt;
// below 5 it has none, and it never has fewer than none.
func TestSkillPointsAreEarnedFromLevelFiveLessThoseSpent(t *testing.T) {
	cases := []struct {
		level  uint8
		learnt map[account.Skill]uint8
		want   uint32
	}{
		{4, nil, 0},
		{5, nil, 1},
		{10, nil, 6},
		{10, map[account.Skill]uint8{DragonSwirl: 2, EnchantedBlade: 1}, 3},
		{6, map[account.Skill]uint8{DragonSwirl: 5}, 0},
	}
	for _, c := range cases {
		if got := Points(c.level, c.learnt); got != c.want {
			t.Errorf("level %d with %v: %d points, want %d", c.level, c.learnt, got, c.want)
		}
	}
}

// A skill's power is the client's table: 0.05 at the first level, 0.40 at 19, 0.50 at 20.
func TestASkillsPowerIsTheClientsTable(t *testing.T) {
	for level, want := range map[uint8]uint32{0: 0, 1: 5, 2: 6, 3: 8, 10: 22, 19: 40, 20: 50} {
		if got := Power(level); got != want {
			t.Errorf("level %d: %d, want %d", level, got, want)
		}
	}
}

// What a skill costs, how long it cools down and how long it runs are the client's
// formulas at its power, rounded down, so the client's own checks agree: its skilltable
// for all but the Blade's run, three minutes as the veteran argued.
func TestASkillCostsAndWaitsWhatTheClientWorksOut(t *testing.T) {
	cases := []struct {
		skill          account.Skill
		mana, cooldown func(k float64) float64
		lasts          func(k float64) float64
	}{
		{FingerStrike, func(k float64) float64 { return 30 + 140*k }, func(float64) float64 { return 10 }, func(float64) float64 { return 0 }},
		{DragonSwirl, func(k float64) float64 { return 50 + 150*k }, func(float64) float64 { return 15 }, func(float64) float64 { return 0 }},
		{EnchantedBlade, func(k float64) float64 { return 20 + 240*k }, func(float64) float64 { return 0 }, func(float64) float64 { return 180 }},
		{EnchantedArmour, func(k float64) float64 { return 70 + 170*k }, func(k float64) float64 { return 33 + 140*k }, func(k float64) float64 { return 30 + 120*k }},
	}
	for _, c := range cases {
		p, _ := Lookup(c.skill)
		for level := uint8(1); level <= MaxLevel; level++ {
			k := float64(Power(level)) / 100
			if got, want := p.Mana.At(Power(level)), uint32(c.mana(k)); got != want {
				t.Errorf("%s at %d: mana %d, want %d", c.skill, level, got, want)
			}
			if got, want := p.CooldownAt(level), time.Duration(c.cooldown(k))*time.Second; got != want {
				t.Errorf("%s at %d: cooldown %v, want %v", c.skill, level, got, want)
			}
			if got, want := p.LastsAt(level), time.Duration(c.lasts(k))*time.Second; got != want {
				t.Errorf("%s at %d: lasts %v, want %v", c.skill, level, got, want)
			}
		}
	}
}

// Enchanted Armour never runs as long as it cools down, so it cannot be cast over itself.
func TestEnchantedArmourEndsBeforeItCanBeCastAgain(t *testing.T) {
	p, _ := Lookup(EnchantedArmour)
	for level := uint8(1); level <= MaxLevel; level++ {
		if p.LastsAt(level) >= p.CooldownAt(level) {
			t.Errorf("at %d: lasts %v, cools %v", level, p.LastsAt(level), p.CooldownAt(level))
		}
	}
}

// The blade adds 7 + (5·INT + 13)·k to the attack, as the client's tooltip shows it, and
// the armour (INT + 30)·k to the defence, as its skilltable, rounded down: a sura of INT 5
// adds 8 to the attack at 1 (8.9) and 26 at 20 (26.0), 1 to the defence at 1 (1.75) and
// 17 at 20 (17.5).
func TestBuffsAddWhatTheClientShows(t *testing.T) {
	blade, _ := Lookup(EnchantedBlade)
	armour, _ := Lookup(EnchantedArmour)
	swirl, _ := Lookup(DragonSwirl)
	for level, want := range map[uint8][2]uint16{1: {8, 1}, 20: {26, 17}} {
		if got := [2]uint16{blade.Bonus(5, level), armour.Bonus(5, level)}; got != want {
			t.Errorf("at %d: %v, want %v", level, got, want)
		}
	}
	if blade.Raises != Attack || armour.Raises != Defence || swirl.Bonus(5, 20) != 0 {
		t.Fatalf("blade %+v, armour %+v", blade, armour)
	}
}
