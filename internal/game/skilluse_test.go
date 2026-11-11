package game

import (
	"encoding/binary"
	"math/rand/v2"
	"net"
	"testing"
	"time"

	"github.com/SatoriAI/MT2-Go/internal/account"
	"github.com/SatoriAI/MT2-Go/internal/combat"
	"github.com/SatoriAI/MT2-Go/internal/mob"
	"github.com/SatoriAI/MT2-Go/internal/network"
	"github.com/SatoriAI/MT2-Go/internal/protocol"
	"github.com/SatoriAI/MT2-Go/internal/skill"
	"github.com/SatoriAI/MT2-Go/internal/world"
)

// swirling is Hendra, a level-10 Weaponry sura with 500 mana, a sword in hand and Dragon
// Swirl and Enchanted Blade at 1, 100 east of two wild dogs that leave nothing, with a third 1500
// further off, and Kira beside her; their clients have read what entering shows.
func swirling(t *testing.T) (*session, net.Conn, net.Conn) {
	t.Helper()
	s := worldWith(t, world.Spawn{VNum: 101, X: homeX, Y: homeY, Count: 2}, world.Spawn{VNum: 101, X: homeX + 1600, Y: homeY, Count: 1})
	for _, m := range s.monsters {
		bare := *m.proto
		bare.Drops, bare.Yang = nil, mob.Yang{}
		m.proto = &bare
	}
	s.now = func() time.Time { return noon }
	client, hendra := aPlayer(t, s, 1000, 7, "Hendra", 100)
	c := hendra.char
	c.Level, c.SP, c.HP, c.SkillGroup, c.Weapon = 10, 500, 500, "weaponry", 10
	c.Intelligence = 5
	c.Skills = map[account.Skill]uint8{skill.DragonSwirl: 1, skill.EnchantedBlade: 1}
	kira, _ := aPlayer(t, s, 1001, 8, "Kira", 200)
	read(t, client, 3*addSize+addSize+infoSize) // the dogs, then Kira
	read(t, kira, 3*addSize+addSize+infoSize)
	sess := &session{svc: s, conn: hendra.out.(*network.Conn), phase: protocol.PhaseGame, rand: rand.New(rand.NewPCG(5, 6)), //nolint:gosec // a fixed seed
		playing: playingAs(hendra)}
	return sess, client, kira
}

func used(sk account.Skill) []byte {
	return binary.LittleEndian.AppendUint32(binary.LittleEndian.AppendUint32(nil, uint32(skillVNums[sk])), 0)
}

func manaSpent(cost, left int32) []byte {
	return protocol.EncodePointChange(1000, protocol.PointSP, -cost, left)
}

// Dragon Swirl costs its mana, then strikes both dogs within 500 of her and not the one
// further off: each takes damage counted as hers, and turns on her.
func TestDragonSwirlStrikesEveryMonsterInItsReach(t *testing.T) {
	s, client, _ := swirling(t)

	if err := s.onUseSkill(used(skill.DragonSwirl)); err != nil {
		t.Fatal(err)
	}

	if got := read(t, client, pointChangeSize); string(got) != string(manaSpent(57, 443)) { // 50 + 150 × 0.05
		t.Fatalf("told %x", got)
	}
	read(t, client, 2*damageSize)
	expectNothing(t, client)
	pl := s.svc.record.state.all[1000]
	struck := 0
	for _, m := range s.svc.monsters {
		if m.p.fight.damageBy[pl] > 0 {
			struck++
			if m.x != homeX {
				t.Fatalf("struck the dog at %d", m.x)
			}
		}
	}
	if struck != 2 || len(s.svc.record.state.news.provoked) != 2 {
		t.Fatalf("struck %d, provoked %d", struck, len(s.svc.record.state.news.provoked))
	}
}

// A swirl's blow is a normal blow times the skill's: twice and a bit at its first level.
func TestDragonSwirlStrikesHarderThanABlow(t *testing.T) {
	s, _, _ := swirling(t)
	pl := s.svc.record.state.all[1000]
	dog := s.svc.monsters[0].p.fight
	mean := combat.Mean(attackerOf(pl.char), dog.defender)

	if err := s.onUseSkill(used(skill.DragonSwirl)); err != nil {
		t.Fatal(err)
	}

	if got := float64(dog.damageBy[pl]); got < 1.8*mean || got > 2.6*mean {
		t.Fatalf("struck %v against a blow's %v", got, mean)
	}
}

// A dog left with a blow's worth of hit points dies to the swirl: its death is the world
// loop's to take, and Hendra is given its experience.
func TestDragonSwirlKillsWhatItStrikesToDeath(t *testing.T) {
	s, client, _ := swirling(t)
	for _, m := range s.svc.monsters {
		m.p.fight.hp = 1
	}

	if err := s.onUseSkill(used(skill.DragonSwirl)); err != nil {
		t.Fatal(err)
	}

	read(t, client, pointChangeSize)
	if len(s.svc.record.state.news.fallen) != 2 || s.svc.record.state.all[1000].char.Exp == 0 {
		t.Fatalf("%d fell, exp %d", len(s.svc.record.state.news.fallen), s.svc.record.state.all[1000].char.Exp)
	}
}

// Dragon Swirl waits out its 15 seconds, and its next use strikes only the living: the
// dogs it killed are not struck again.
func TestDragonSwirlWaitsForItsCooldownAndSparesTheDead(t *testing.T) {
	s, _, _ := swirling(t)
	for _, m := range s.svc.monsters {
		m.p.fight.hp = 1
	}
	if err := s.onUseSkill(used(skill.DragonSwirl)); err != nil {
		t.Fatal(err)
	}
	sp := s.svc.record.state.all[1000].char.SP

	s.svc.now = func() time.Time { return noon.Add(15*time.Second - time.Millisecond) }
	if err := s.onUseSkill(used(skill.DragonSwirl)); err != nil {
		t.Fatal(err)
	}
	if s.svc.record.state.all[1000].char.SP != sp {
		t.Fatal("used again before its cooldown")
	}
	s.svc.now = func() time.Time { return noon.Add(15 * time.Second) }
	if err := s.onUseSkill(used(skill.DragonSwirl)); err != nil {
		t.Fatal(err)
	}

	if s.svc.record.state.all[1000].char.SP == sp || len(s.svc.record.state.news.fallen) != 2 {
		t.Fatalf("SP %d, %d fell", s.svc.record.state.all[1000].char.SP, len(s.svc.record.state.news.fallen))
	}
}

// A skill is not used before its cooldown is over, without the mana, unlearnt, by the
// dead, without the sword it needs, or by a number the game has no skill for; nothing is
// spent or struck.
func TestASkillThatCannotBeUsedIsNot(t *testing.T) {
	cases := map[string]func(s *session) []byte{
		"cooling down": func(s *session) []byte {
			s.svc.cooldowns.used(s.playing.ID, skill.DragonSwirl, noon.Add(time.Second))
			return used(skill.DragonSwirl)
		},
		"too little mana": func(s *session) []byte { s.svc.record.state.all[1000].char.SP = 56; return used(skill.DragonSwirl) },
		"unlearnt": func(s *session) []byte {
			delete(s.svc.record.state.all[1000].char.Skills, skill.DragonSwirl)
			return used(skill.DragonSwirl)
		},
		"dead": func(s *session) []byte { s.svc.record.state.all[1000].char.HP = 0; return used(skill.DragonSwirl) },
		"the blade with no sword": func(s *session) []byte {
			s.svc.record.state.all[1000].char.Weapon = 1000 // a dagger
			return used(skill.EnchantedBlade)
		},
		"no such skill": func(*session) []byte {
			return binary.LittleEndian.AppendUint32(binary.LittleEndian.AppendUint32(nil, 64), 0)
		},
	}
	for name, setup := range cases {
		s, client, _ := swirling(t)
		body := setup(s)
		sp := s.svc.record.state.all[1000].char.SP

		if err := s.onUseSkill(body); err != nil {
			t.Fatal(err)
		}

		expectNothing(t, client)
		if s.svc.record.state.all[1000].char.SP != sp || len(s.svc.record.state.news.provoked) != 0 {
			t.Errorf("%s: used", name)
		}
	}
}

const (
	affectAddSize    = 22
	affectRemoveSize = 6
)

// expectBladeGone reads Enchanted Blade's time taken off its icon.
func expectBladeGone(t *testing.T, c net.Conn) {
	t.Helper()
	if got := read(t, c, affectRemoveSize); string(got) != string(buffGone(skill.EnchantedBlade)) {
		t.Fatalf("got %x, want the blade's time gone", got)
	}
}

// expectGlow reads Hendra's CHARACTER_UPDATE, with the blade's glow or without.
func expectGlow(t *testing.T, c net.Conn, on bool) {
	t.Helper()
	got := read(t, c, updateSize)
	affects := binary.LittleEndian.Uint32(got[16:])
	if got[0] != byte(protocol.ServerCharacterUpdate) || (affects&(1<<19) != 0) != on {
		t.Fatalf("got %x, want the glow %v", got, on)
	}
}

// Enchanted Blade costs its mana, puts its glow on her for her and Kira, shows her its
// three minutes on its icon, and makes her blows stronger while it runs.
func TestEnchantedBladeGlowsAndStrengthensHerBlows(t *testing.T) {
	s, client, kira := swirling(t)
	pl := s.svc.record.state.all[1000]
	plain := attackerOf(pl.char).Bonus

	if err := s.onUseSkill(used(skill.EnchantedBlade)); err != nil {
		t.Fatal(err)
	}

	if got := read(t, client, pointChangeSize); string(got) != string(manaSpent(32, 468)) { // 20 + 240 × 0.05
		t.Fatalf("told %x", got)
	}
	expectGlow(t, client, true)
	if got := read(t, client, affectAddSize); string(got) != string(protocol.EncodeAffectAdd(protocol.Affect{Type: 63, Duration: 180})) {
		t.Fatalf("its time told %x", got)
	}
	expectGlow(t, kira, true)
	if got := attackerOf(pl.char).Bonus; plain != 0 || got != bladeBonus(5, 1) || got == 0 {
		t.Fatalf("bonus %d, before %d", got, plain)
	}
}

// Using the blade again turns it off, for nothing, and its glow goes.
func TestEnchantedBladeUsedAgainEndsFree(t *testing.T) {
	s, client, _ := swirling(t)
	if err := s.onUseSkill(used(skill.EnchantedBlade)); err != nil {
		t.Fatal(err)
	}
	read(t, client, pointChangeSize+updateSize+affectAddSize)
	s.svc.record.state.all[1000].char.SP = 0 // turning it off needs none, as the client checks none

	if err := s.onUseSkill(used(skill.EnchantedBlade)); err != nil {
		t.Fatal(err)
	}

	expectGlow(t, client, false)
	expectBladeGone(t, client)
	if c := s.svc.record.state.all[1000].char; c.SP != 0 || len(c.Buffs) != 0 || attackerOf(c).Bonus != 0 {
		t.Fatalf("SP %d, buffs %v", c.SP, c.Buffs)
	}
}

// The world loop ends the blade three minutes after it began.
func TestEnchantedBladeEndsAfterThreeMinutes(t *testing.T) {
	s, client, _ := swirling(t)
	if err := s.onUseSkill(used(skill.EnchantedBlade)); err != nil {
		t.Fatal(err)
	}
	read(t, client, pointChangeSize+updateSize+affectAddSize)
	for _, m := range s.svc.monsters {
		m.nextStroll = noon.Add(time.Hour) // nothing in these ticks but the blade
	}
	c := s.svc.record.state.all[1000].char
	points, _ := c.Points()
	c.HP, c.SP = points.MaxHP, points.MaxSP

	s.svc.tick(noon)
	s.svc.tick(noon.Add(3*time.Minute - time.Millisecond))
	expectNothing(t, client)
	s.svc.tick(noon.Add(3 * time.Minute))

	expectGlow(t, client, false)
	expectBladeGone(t, client)
}

// A run turned off and started again a minute in is not ended when the first was due.
func TestALaterRunOfTheBladeOutlastsTheFirstsEnd(t *testing.T) {
	s, client, _ := swirling(t)
	if err := s.onUseSkill(used(skill.EnchantedBlade)); err != nil {
		t.Fatal(err)
	}
	s.svc.now = func() time.Time { return noon.Add(time.Minute) }
	for range 2 { // off, then on again
		if err := s.onUseSkill(used(skill.EnchantedBlade)); err != nil {
			t.Fatal(err)
		}
	}
	read(t, client, pointChangeSize+updateSize+affectAddSize+updateSize+affectRemoveSize+pointChangeSize+updateSize+affectAddSize)
	queueBuffEnds(s.svc) // both runs' ends

	s.svc.keepBuffs(noon.Add(3 * time.Minute))

	expectNothing(t, client)
	s.svc.keepBuffs(noon.Add(4 * time.Minute))
	expectGlow(t, client, false)
	expectBladeGone(t, client)
}

// Death ends the blade at once.
func TestEnchantedBladeEndsOnDeath(t *testing.T) {
	s, client, _ := swirling(t)
	if err := s.onUseSkill(used(skill.EnchantedBlade)); err != nil {
		t.Fatal(err)
	}
	read(t, client, pointChangeSize+updateSize+affectAddSize)
	pl := s.svc.record.state.all[1000]

	s.svc.record.held(func(l locked) {
		pl.char.HP = 0
		l.die(pl)
	})

	expectGlow(t, client, false)
	expectBladeGone(t, client)
	if len(pl.char.Buffs) != 0 {
		t.Fatalf("buffs %v", pl.char.Buffs)
	}
}

// One who comes into view while the blade runs sees its glow on her as she appears.
func TestANewcomerSeesTheBladesGlow(t *testing.T) {
	s, client, _ := swirling(t)
	if err := s.onUseSkill(used(skill.EnchantedBlade)); err != nil {
		t.Fatal(err)
	}
	read(t, client, pointChangeSize+updateSize+affectAddSize)

	newcomer, _ := aPlayer(t, s.svc, 1002, 9, "Lena", 300)

	shown := read(t, newcomer, 3*addSize+2*(addSize+infoSize)) // the dogs, Hendra and Kira
	for len(shown) > 0 {
		size := addSize
		if shown[0] == byte(protocol.ServerCharacterInfo) {
			size = infoSize
		}
		if shown[0] == byte(protocol.ServerCharacterAdd) && binary.LittleEndian.Uint32(shown[1:]) == 1000 {
			if affects := binary.LittleEndian.Uint32(shown[27:]); affects&(1<<19) == 0 {
				t.Fatalf("Hendra shown with effects %x", affects)
			}
			return
		}
		shown = shown[size:]
	}
	t.Fatal("Hendra was not shown")
}

// queueBuffEnds hands the world loop the buffs' ends from the news, as its tick does,
// without the rest of a tick.
func queueBuffEnds(s *Service) {
	for _, b := range s.record.takeNews().buffs {
		s.buffEnds = inOrder(s.buffEnds, b)
	}
}

// Dressing while the blade runs keeps its glow on her: the update a change of armour
// sends carries the effects too.
func TestDressingKeepsTheBladesGlow(t *testing.T) {
	s, client, _ := swirling(t)
	if err := s.onUseSkill(used(skill.EnchantedBlade)); err != nil {
		t.Fatal(err)
	}
	read(t, client, pointChangeSize+updateSize+affectAddSize)

	s.svc.record.dress(1000, outfit{weapon: 10})

	expectGlow(t, client, true)
}

// Taking the sword out of hand ends the blade: the update the change sends shows no glow.
func TestTakingTheSwordOffEndsTheBlade(t *testing.T) {
	s, client, _ := swirling(t)
	if err := s.onUseSkill(used(skill.EnchantedBlade)); err != nil {
		t.Fatal(err)
	}
	read(t, client, pointChangeSize+updateSize+affectAddSize)

	s.svc.record.dress(1000, outfit{})

	expectGlow(t, client, false)
	expectBladeGone(t, client)
	if c := s.svc.record.state.all[1000].char; len(c.Buffs) != 0 {
		t.Fatalf("buffs %v", c.Buffs)
	}
}

// A cooldown is the character's: logging in again, as a new connection, does not end it.
func TestACooldownOutlastsTheConnection(t *testing.T) {
	s, _, _ := swirling(t)
	if err := s.onUseSkill(used(skill.DragonSwirl)); err != nil {
		t.Fatal(err)
	}
	again := &session{svc: s.svc, conn: s.conn, phase: protocol.PhaseGame, rand: s.rand, playing: s.playing}
	sp := s.svc.record.state.all[1000].char.SP

	if err := again.onUseSkill(used(skill.DragonSwirl)); err != nil {
		t.Fatal(err)
	}

	if s.svc.record.state.all[1000].char.SP != sp {
		t.Fatal("swirled again on a new connection")
	}
}

func bladeBonus(intelligence, level uint8) uint16 {
	p, _ := skill.Lookup(skill.EnchantedBlade)
	return uint16(p.Bonus(skill.Attack, skill.Bearer{Intelligence: intelligence}, level)) //nolint:gosec // small
}

// usedOn is the client using a skill on the monster with vid.
func usedOn(sk account.Skill, vid uint32) []byte {
	return binary.LittleEndian.AppendUint32(binary.LittleEndian.AppendUint32(nil, uint32(skillVNums[sk])), vid)
}

// Finger Strike costs its mana and strikes the dog she picked, alone, for two and a half
// blows and a bit at its first level; the other dog beside it is spared.
func TestFingerStrikeStrikesTheOneSheTargets(t *testing.T) {
	s, client, _ := swirling(t)
	s.svc.record.state.all[1000].char.Skills[skill.FingerStrike] = 1
	pl := s.svc.record.state.all[1000]
	dog, other := s.svc.monsters[0].p, s.svc.monsters[1].p
	mean := combat.Mean(attackerOf(pl.char), dog.fight.defender)

	if err := s.onUseSkill(usedOn(skill.FingerStrike, dog.add.VID)); err != nil {
		t.Fatal(err)
	}

	if got := read(t, client, pointChangeSize); string(got) != string(manaSpent(37, 463)) { // 30 + 140 × 0.05
		t.Fatalf("told %x", got)
	}
	read(t, client, damageSize)
	expectNothing(t, client)
	if got := float64(dog.fight.damageBy[pl]); got < 2.1*mean || got > 3.1*mean || other.fight.damageBy[pl] != 0 {
		t.Fatalf("struck %v against a blow's %v, the other %d", got, mean, other.fight.damageBy[pl])
	}
}

// Finger Strike needs a living monster within a blow's reach: on none, the far dog, a dead
// one or Kira it is not used, and no mana is spent.
func TestFingerStrikeNeedsAMonsterInReach(t *testing.T) {
	cases := map[string]func(s *session) uint32{
		"none":     func(*session) uint32 { return 0 },
		"too far":  func(s *session) uint32 { return s.svc.monsters[2].p.add.VID },
		"dead":     func(s *session) uint32 { s.svc.monsters[0].p.fight.diedAt = noon; return s.svc.monsters[0].p.add.VID },
		"a player": func(*session) uint32 { return 1001 },
	}
	for name, target := range cases {
		s, client, _ := swirling(t)
		s.svc.record.state.all[1000].char.Skills[skill.FingerStrike] = 1

		if err := s.onUseSkill(usedOn(skill.FingerStrike, target(s))); err != nil {
			t.Fatal(err)
		}

		expectNothing(t, client)
		if s.svc.record.state.all[1000].char.SP != 500 || len(s.svc.record.state.news.provoked) != 0 {
			t.Errorf("%s: used", name)
		}
	}
}

// Enchanted Armour costs its mana, shows its effect to her and Kira, tells her the defence
// it adds and sets it against bites, and its cooldown outlasts it: used again while it
// runs, it is refused rather than turned off.
func TestEnchantedArmourRaisesHerDefenceUntilItEnds(t *testing.T) {
	s, client, kira := swirling(t)
	c := s.svc.record.state.all[1000].char
	c.Skills[skill.EnchantedArmour] = 1
	before, plain := defence(c), armourOf(c)

	if err := s.onUseSkill(used(skill.EnchantedArmour)); err != nil {
		t.Fatal(err)
	}

	if got := read(t, client, pointChangeSize); string(got) != string(manaSpent(78, 422)) { // 70 + 170 × 0.05
		t.Fatalf("told %x", got)
	}
	expectArmoured(t, client, true)
	if got := read(t, client, pointChangeSize); string(got) != string(defenceTold(before+1)) {
		t.Fatalf("told %x", got)
	}
	armour, _ := skill.Lookup(skill.EnchantedArmour)
	if got := read(t, client, affectAddSize); string(got) != string(buffShown(skill.EnchantedArmour, armour.LastsAt(c.Skills[skill.EnchantedArmour]))) {
		t.Fatalf("its time told %x", got)
	}
	expectArmoured(t, kira, true)
	if armourOf(c) != plain+1 { // (5 + 30) × 0.05
		t.Fatalf("armour %d, before %d", armourOf(c), plain)
	}

	s.svc.now = func() time.Time { return noon.Add(time.Second) }
	if err := s.onUseSkill(used(skill.EnchantedArmour)); err != nil {
		t.Fatal(err)
	}
	expectNothing(t, client)
	if _, on := c.Buffs[skill.EnchantedArmour]; !on || c.SP != 422 {
		t.Fatalf("buffs %v, SP %d", c.Buffs, c.SP)
	}
}

// The world loop ends the armour 36 seconds after it began, at its first level, and tells
// her her defence without it.
func TestEnchantedArmourEndsWhenItsTimeIsUp(t *testing.T) {
	s, client, _ := swirling(t)
	c := s.svc.record.state.all[1000].char
	c.Skills[skill.EnchantedArmour] = 1
	before := defence(c)
	if err := s.onUseSkill(used(skill.EnchantedArmour)); err != nil {
		t.Fatal(err)
	}
	read(t, client, pointChangeSize+updateSize+pointChangeSize+affectAddSize)
	for _, m := range s.svc.monsters {
		m.nextStroll = noon.Add(time.Hour)
	}
	points, _ := c.Points()
	c.HP, c.SP = points.MaxHP, points.MaxSP

	s.svc.tick(noon)
	s.svc.tick(noon.Add(36*time.Second - time.Millisecond)) // 30 + 120 × 0.05
	expectNothing(t, client)
	s.svc.tick(noon.Add(36 * time.Second))

	expectArmoured(t, client, false)
	if got := read(t, client, pointChangeSize); string(got) != string(defenceTold(before)) {
		t.Fatalf("told %x", got)
	}
}

// The blade and the armour run together: each shows its own effect and adds its own
// bonus.
func TestTheBladeAndTheArmourRunTogether(t *testing.T) {
	s, client, _ := swirling(t)
	c := s.svc.record.state.all[1000].char
	c.Skills[skill.EnchantedArmour] = 1
	plain := armourOf(c)

	for _, sk := range []account.Skill{skill.EnchantedBlade, skill.EnchantedArmour} {
		if err := s.onUseSkill(used(sk)); err != nil {
			t.Fatal(err)
		}
	}

	read(t, client, pointChangeSize+updateSize+pointChangeSize+affectAddSize)
	got := read(t, client, updateSize)
	if affects := binary.LittleEndian.Uint32(got[16:]); affects != 1<<19|1<<21 {
		t.Fatalf("affects %b", affects)
	}
	if attackerOf(c).Bonus != bladeBonus(5, 1) || armourOf(c) != plain+1 {
		t.Fatalf("attack %d, armour %d", attackerOf(c).Bonus, armourOf(c))
	}
}

// expectArmoured reads Hendra's CHARACTER_UPDATE, with the armour's effect or without.
func expectArmoured(t *testing.T, c net.Conn, on bool) {
	t.Helper()
	got := read(t, c, updateSize)
	affects := binary.LittleEndian.Uint32(got[16:])
	if got[0] != byte(protocol.ServerCharacterUpdate) || (affects&(1<<21) != 0) != on {
		t.Fatalf("got %x, want the armour %v", got, on)
	}
}

// A point of intelligence while the armour runs raises what it adds, and she is told her
// defence with her new maximum mana.
func TestIntelligenceRaisedUnderTheArmourTellsHerDefence(t *testing.T) {
	s, client, _ := swirling(t)
	c := s.svc.record.state.all[1000].char
	c.Skills[skill.EnchantedArmour], c.Intelligence, c.StatPoints = 20, 9, 1
	if err := s.onUseSkill(used(skill.EnchantedArmour)); err != nil {
		t.Fatal(err)
	}
	read(t, client, pointChangeSize+updateSize+pointChangeSize+affectAddSize)
	before := defence(c)

	s.svc.record.spendStatPoint(1000, "iq")

	read(t, client, 3*pointChangeSize) // the stat, the points left, the maximum mana
	if got := read(t, client, pointChangeSize); string(got) != string(defenceTold(before+1)) {
		t.Fatalf("told %x, defence before %d", got, before)
	}
}

func defenceTold(d uint32) []byte {
	return protocol.EncodePointChange(1000, protocol.PointDefence, 0, int32(d)) //nolint:gosec // small
}

// Finger Strike refused on nothing notes no cooldown, so the strike on the dog right after
// is used; then it waits out its 10 seconds.
func TestFingerStrikeCoolsDownOnlyOnceUsed(t *testing.T) {
	s, client, _ := swirling(t)
	s.svc.record.state.all[1000].char.Skills[skill.FingerStrike] = 1
	dog := s.svc.monsters[0].p
	strike := func(at time.Duration, target uint32) {
		s.svc.now = func() time.Time { return noon.Add(at) }
		if err := s.onUseSkill(usedOn(skill.FingerStrike, target)); err != nil {
			t.Fatal(err)
		}
	}

	strike(0, 0)
	strike(0, dog.add.VID)
	read(t, client, pointChangeSize+damageSize)
	dog.fight.hp = 1000
	strike(10*time.Second-time.Millisecond, dog.add.VID)
	expectNothing(t, client)
	strike(10*time.Second, dog.add.VID)

	if got := read(t, client, pointChangeSize); string(got) != string(manaSpent(37, 426)) {
		t.Fatalf("told %x", got)
	}
}
