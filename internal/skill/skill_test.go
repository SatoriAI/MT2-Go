package skill

import (
	"slices"
	"testing"
	"time"

	"github.com/SatoriAI/MT2-Go/internal/account"
	"github.com/SatoriAI/MT2-Go/internal/item"
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
		{ThreeWayCut, func(k float64) float64 { return 40 + 100*k }, func(float64) float64 { return 12 }, func(float64) float64 { return 0 }},
		{SwordSpin, func(k float64) float64 { return 50 + 130*k }, func(float64) float64 { return 15 }, func(float64) float64 { return 0 }},
		{Berserk, func(k float64) float64 { return 50 + 140*k }, func(k float64) float64 { return 63 + 90*k }, func(k float64) float64 { return 60 + 90*k }},
		{SwordAura, func(k float64) float64 { return 100 + 200*k }, func(k float64) float64 { return 33 + 50*k }, func(k float64) float64 { return 30 + 50*k }},
		{SpiritStrike, func(k float64) float64 { return 60 + 120*k }, func(float64) float64 { return 15 }, func(float64) float64 { return 0 }},
		{Bash, func(k float64) float64 { return 60 + 150*k }, func(float64) float64 { return 15 }, func(float64) float64 { return 0 }},
		{Stump, func(k float64) float64 { return 50 + 140*k }, func(float64) float64 { return 25 }, func(float64) float64 { return 0 }},
		{StrongBody, func(k float64) float64 { return 80 + 220*k }, func(k float64) float64 { return 63 + 90*k }, func(k float64) float64 { return 60 + 90*k }},
		{SwordStrike, func(k float64) float64 { return 40 + 120*k }, func(float64) float64 { return 20 }, func(float64) float64 { return 0 }},
		{Ambush, func(k float64) float64 { return 40 + 160*k }, func(float64) float64 { return 15 }, func(float64) float64 { return 0 }},
		{FastAttack, func(k float64) float64 { return 40 + 160*k }, func(float64) float64 { return 20 }, func(float64) float64 { return 0 }},
		{RollingDagger, func(k float64) float64 { return 50 + 140*k }, func(float64) float64 { return 25 }, func(float64) float64 { return 0 }},
		{DarkStrike, func(k float64) float64 { return 30 + 140*k }, func(float64) float64 { return 7 }, func(float64) float64 { return 0 }},
		{FlameStrike, func(k float64) float64 { return 60 + 140*k }, func(float64) float64 { return 12 }, func(float64) float64 { return 0 }},
		{DarkProtection, func(k float64) float64 { return 20 + 30*k }, func(k float64) float64 { return 63 + 120*k }, func(k float64) float64 { return 60 + 120*k }},
		{GhostStrike, func(k float64) float64 { return 40 + 120*k }, func(float64) float64 { return 12 }, func(float64) float64 { return 0 }},
		{DarkOrb, func(k float64) float64 { return 80 + 220*k }, func(float64) float64 { return 24 }, func(float64) float64 { return 0 }},
		{FlyingTalisman, func(k float64) float64 { return 30 + 160*k }, func(float64) float64 { return 7 }, func(float64) float64 { return 0 }},
		{ShootingDragon, func(k float64) float64 { return 50 + 160*k }, func(float64) float64 { return 8 }, func(float64) float64 { return 0 }},
		{DragonRoar, func(k float64) float64 { return 50 + 160*k }, func(float64) float64 { return 20 }, func(float64) float64 { return 0 }},
		{LightningThrow, func(k float64) float64 { return 30 + 150*k }, func(float64) float64 { return 7 }, func(float64) float64 { return 0 }},
		{SummonLightning, func(k float64) float64 { return 50 + 150*k }, func(float64) float64 { return 15 }, func(float64) float64 { return 0 }},
		{LightningClaw, func(k float64) float64 { return 40 + 180*k }, func(float64) float64 { return 10 }, func(float64) float64 { return 0 }},
		{Blessing, func(k float64) float64 { return 40 + 160*k }, func(float64) float64 { return 10 }, func(k float64) float64 { return 60 + 200*k }},
		{Reflection, func(k float64) float64 { return 40 + 160*k }, func(float64) float64 { return 10 }, func(k float64) float64 { return 60 + 200*k }},
		{DragonsHelp, func(k float64) float64 { return 40 + 160*k }, func(float64) float64 { return 10 }, func(k float64) float64 { return 60 + 100*k }},
		{Heal, func(k float64) float64 { return 40 + 200*k }, func(float64) float64 { return 10 }, func(float64) float64 { return 0 }},
		{Swiftness, func(k float64) float64 { return 60 + 120*k }, func(float64) float64 { return 10 }, func(k float64) float64 { return 60 + 100*k }},
		{AttackUp, func(k float64) float64 { return 60 + 120*k }, func(float64) float64 { return 10 }, func(k float64) float64 { return 60 + 100*k }},
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
	sura := Bearer{Intelligence: 5}
	for level, want := range map[uint8][2]int{1: {8, 1}, 20: {26, 17}} {
		if got := [2]int{blade.Bonus(Attack, sura, level), armour.Bonus(Defence, sura, level)}; got != want {
			t.Errorf("at %d: %v, want %v", level, got, want)
		}
	}
	if blade.Bonus(Defence, sura, 20) != 0 || armour.Bonus(Attack, sura, 20) != 0 || swirl.Bonus(Attack, sura, 20) != 0 {
		t.Fatalf("blade %+v, armour %+v", blade, armour)
	}
}

// Sword Aura adds (100 + STR)·k to the attack, its tooltip, and Berserk 50k to the attack
// speed and 20k to the move speed, its skilltable, rounded down: a warrior of STR 30 adds
// 6 at 1 (6.5) and 65 at 20; Berserk 2 and 1 at 1 (2.5, 1.0), 25 and 10 at 20.
func TestWarriorBuffsAddWhatTheClientShows(t *testing.T) {
	aura, _ := Lookup(SwordAura)
	berserk, _ := Lookup(Berserk)
	warrior := Bearer{Strength: 30}
	for level, want := range map[uint8][3]int{1: {6, 2, 1}, 20: {65, 25, 10}} {
		got := [3]int{aura.Bonus(Attack, warrior, level), berserk.Bonus(AttackSpeed, warrior, level), berserk.Bonus(MoveSpeed, warrior, level)}
		if got != want {
			t.Errorf("at %d: %v, want %v", level, got, want)
		}
	}
	if berserk.Bonus(Attack, warrior, 20) != 0 || aura.Bonus(AttackSpeed, warrior, 20) != 0 {
		t.Fatal("a buff raises what it should not")
	}
}

// Three-Way Cut, Sword Spin and Sword Aura need a sword or a two-handed weapon, as the
// client's WEAPON_LIMITATION has it; Enchanted Blade a one-handed sword; Berserk any.
func TestWarriorSkillsNeedASwordOrATwoHandedWeapon(t *testing.T) {
	sword, glaive, fan := item.Proto{Sword: true}, item.Proto{TwoHanded: true}, item.Proto{}
	for s, want := range map[account.Skill][3]bool{
		ThreeWayCut: {true, true, false}, SwordSpin: {true, true, false}, SwordAura: {true, true, false},
		EnchantedBlade: {true, false, false}, Berserk: {true, true, true},
	} {
		p, _ := Lookup(s)
		if got := [3]bool{p.Wieldable(sword), p.Wieldable(glaive), p.Wieldable(fan)}; got != want {
			t.Errorf("%s with a sword, glaive, fan: %v, want %v", s, got, want)
		}
	}
}

// Strong Body adds its tooltip's 7 + (40 + 0.2·STR + 0.4·CON)·k to the defence and takes
// 1 + 9k off the move speed, rounded towards zero: with STR 30 and CON 10, 9 and 1 at its
// first level (9.5, 1.45), 32 and 5 at 20 (32, 5.5). Berserk makes a bite (5 + 10k)%
// stronger: 5 at 1, 10 at 20.
func TestStrongBodyAndBerserksPriceAreTheirFormulas(t *testing.T) {
	body, _ := Lookup(StrongBody)
	berserk, _ := Lookup(Berserk)
	warrior := Bearer{Strength: 30, Vitality: 10}
	for level, want := range map[uint8][3]int{1: {9, -1, 5}, 20: {32, -5, 10}} {
		got := [3]int{body.Bonus(Defence, warrior, level), body.Bonus(MoveSpeed, warrior, level), berserk.Bonus(DamageTaken, warrior, level)}
		if got != want {
			t.Errorf("at %d: %v, want %v", level, got, want)
		}
	}
}

// A stun's chance is the client's (100 + 1000k/6)/10 percent, in thousandths: 108 at the
// first level, 183 at 20.
func TestAStunsChanceIsTheClients(t *testing.T) {
	for level, want := range map[uint8]int{1: 108, 10: 136, 20: 183} {
		if p, _ := Lookup(Stump); p.StunChance(level) != want {
			t.Errorf("at %d: %d, want %d", level, p.StunChance(level), want)
		}
	}
}

// A poison's chance is its skill's, in thousandths: Rolling Dagger's tooltip 1 + 4k
// percent, 12 at the first level and 30 at 20, and Poison Arrow's skilltable 80k percent,
// 40 at the first, 176 at 10 and 400 at 20.
func TestAPoisonsChanceIsItsSkills(t *testing.T) {
	for sk, wants := range map[account.Skill]map[uint8]uint32{
		RollingDagger: {1: 12, 10: 18, 20: 30},
		PoisonArrow:   {1: 40, 10: 176, 20: 400},
	} {
		p, _ := Lookup(sk)
		for level, want := range wants {
			if got := p.Poisons.At(Power(level)); got != want {
				t.Errorf("%s at %d: %d, want %d", sk, level, got, want)
			}
		}
	}
}

// The ninja's Dagger skills take a dagger or a sword, the client's DAGGER|DOUBLE_SWORD|
// SWORD, and not a glaive.
func TestDaggerSkillsTakeADaggerOrASword(t *testing.T) {
	dagger, sword, glaive := item.Proto{Dagger: true}, item.Proto{Sword: true}, item.Proto{TwoHanded: true}
	for _, s := range []account.Skill{Ambush, FastAttack, RollingDagger} {
		p, _ := Lookup(s)
		if !p.Wieldable(dagger) || !p.Wieldable(sword) || p.Wieldable(glaive) {
			t.Errorf("%s: %v %v %v", s, p.Wieldable(dagger), p.Wieldable(sword), p.Wieldable(glaive))
		}
	}
}

// The hit rate is the client's (min(90, (4·DEX + 2·level)/6) + 210)/300, in hundredths:
// 70 for nobody, 73 at DEX 10 and level 10, 100 once the inner term reaches 90.
func TestTheHitRateIsTheClients(t *testing.T) {
	for _, c := range []struct {
		dex, level uint8
		want       int32
	}{{0, 0, 70}, {10, 10, 73}, {120, 30, 100}, {255, 99, 100}} {
		if got := HitRate(c.dex, c.level); got != c.want {
			t.Errorf("DEX %d, level %d: %d, want %d", c.dex, c.level, got, c.want)
		}
	}
}

// A spell rolls its tooltip's range of a number: Dark Strike 50 to 100, Flame and Ghost
// Strike 180 to 200; Dark Orb has none.
func TestEachSpellRollsItsTooltipsRange(t *testing.T) {
	for s, want := range map[account.Skill][2]int32{DarkStrike: {50, 100}, FlameStrike: {180, 200}, GhostStrike: {180, 200}, DarkOrb: {0, 0},
		LightningThrow: {5, 15}, SummonLightning: {5, 16}, LightningClaw: {1, 800}} {
		if p, _ := Lookup(s); p.Dice != want {
			t.Errorf("%s rolls %v, want %v", s, p.Dice, want)
		}
	}
}

// Each spell is its tooltip's formula, with its rolled number: for a level-10 caster of
// STR 5, CON 10, DEX 10, INT 20, a magic attack of 15, a roll of 90 and a hit rate of 73,
// at power 0.05 and 0.5.
func TestEachSpellIsItsTooltipsFormula(t *testing.T) {
	c := Caster{Level: 10, Strength: 5, Vitality: 10, Dexterity: 10, Intelligence: 20, MTK: 15, Dice: 90, AR: 73}
	cases := map[account.Skill]func(k float64) float64{
		DarkStrike:      func(k float64) float64 { return 40 + 50 + 40 + (400+105+90)*0.73*k },
		FlameStrike:     func(k float64) float64 { return 50 + 40 + (400+120+20+20+90)*k },
		GhostStrike:     func(k float64) float64 { return 40 + 20 + 40 + (20+20+440+90+90)*0.73*k },
		DarkOrb:         func(k float64) float64 { return 120 + 60 + (50+50+800+180)*0.73*k },
		FlyingTalisman:  func(k float64) float64 { return 70 + 40 + (400+75+50)*0.73*k },
		ShootingDragon:  func(k float64) float64 { return 60 + 50 + (360+90+120)*0.73*k },
		DragonRoar:      func(k float64) float64 { return 70 + 30 + (440+195+100)*0.73*k },
		LightningThrow:  func(k float64) float64 { return 60 + 40 + (140+120+90)*0.73*k },
		SummonLightning: func(k float64) float64 { return 40 + 40 + (260+105+90)*0.73*k },
		LightningClaw:   func(k float64) float64 { return 50 + 50 + (120+90+90)*0.73*k },
	}
	for s, want := range cases {
		p, _ := Lookup(s)
		for _, power := range []int32{5, 50} {
			if got, w := p.Magic(c, power), int32(want(float64(power)/100)); got != w {
				t.Errorf("%s at %d: %d, want %d", s, power, got, w)
			}
		}
	}
}

// A slow's chance is the client's (333 + 300k)/10 percent, in thousandths, and it lasts
// the skilltable's 10 + 10k seconds.
func TestASlowsChanceAndTimeAreTheClients(t *testing.T) {
	for level, want := range map[uint8]int{1: 348, 20: 483} {
		if got := SlowChance(level); got != want {
			t.Errorf("chance at %d: %d, want %d", level, got, want)
		}
	}
	for level, want := range map[uint8]time.Duration{1: 10500 * time.Millisecond, 20: 15 * time.Second} {
		if got := SlowsFor(level); got != want {
			t.Errorf("time at %d: %v, want %v", level, got, want)
		}
	}
}

// Dark Protection adds (0.5·INT + 15)·k to the defence and turns 0.84·INT·k percent of
// a bite onto mana, rounded down: at INT 100, 3 and 4 at its first level, 32 and 42 at 20.
func TestDarkProtectionsShieldIsItsFormula(t *testing.T) {
	p, _ := Lookup(DarkProtection)
	sura := Bearer{Intelligence: 100}
	for level, want := range map[uint8][2]int{1: {3, 4}, 20: {32, 42}} {
		if got := [2]int{p.Bonus(Defence, sura, level), p.Bonus(ManaShield, sura, level)}; got != want {
			t.Errorf("at %d: %v, want %v", level, got, want)
		}
	}
}

// A burn sets a monster alight at the skilltable's 0.2·iq·k percent, in thousandths, and
// takes its lv + 5·iq·k in all: at INT 90, 9 and 90 in a thousand at levels 1 and 20; for
// a level-30 caster, 52 and 255.
func TestABurnsChanceAndWholeAreTheSkilltables(t *testing.T) {
	for level, want := range map[uint8][2]int{1: {9, 52}, 20: {90, 255}} {
		if got := [2]int{BurnChance(90, level), BurnDamage(30, 90, level)}; got != want {
			t.Errorf("at %d: %v, want %v", level, got, want)
		}
	}
}

// Lightning Throw and Summon Lightning roll their number from 5 times her INT to 15 and
// 16 times it, every number between, as the skilltable's number(iq*5, iq*15) does.
func TestTheLightningsRollMultiplesOfIntelligence(t *testing.T) {
	throw, _ := Lookup(LightningThrow)
	summon, _ := Lookup(SummonLightning)
	claw, _ := Lookup(LightningClaw)
	if throw.DiceOf(20) != [2]int32{100, 300} || summon.DiceOf(20) != [2]int32{100, 320} || claw.DiceOf(20) != [2]int32{1, 800} {
		t.Fatalf("%v %v %v", throw.DiceOf(20), summon.DiceOf(20), claw.DiceOf(20))
	}
}
