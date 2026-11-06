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
		playing: &character{vid: 1000, Character: *c}}
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
	pl := s.svc.presence.all[1000]
	struck := 0
	for _, m := range s.svc.monsters {
		if m.p.fight.damageBy[pl] > 0 {
			struck++
			if m.x != homeX {
				t.Fatalf("struck the dog at %d", m.x)
			}
		}
	}
	if struck != 2 || len(s.svc.presence.provoked) != 2 {
		t.Fatalf("struck %d, provoked %d", struck, len(s.svc.presence.provoked))
	}
}

// A swirl's blow is a normal blow times the skill's: twice and a bit at its first level.
func TestDragonSwirlStrikesHarderThanABlow(t *testing.T) {
	s, _, _ := swirling(t)
	pl := s.svc.presence.all[1000]
	dog := s.svc.monsters[0].p.fight
	mean := combat.Mean(attackerOf(pl.char, noon), dog.defender)

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
	if len(s.svc.presence.fallen) != 2 || s.svc.presence.all[1000].char.Exp == 0 {
		t.Fatalf("%d fell, exp %d", len(s.svc.presence.fallen), s.svc.presence.all[1000].char.Exp)
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
	sp := s.svc.presence.all[1000].char.SP

	s.svc.now = func() time.Time { return noon.Add(15*time.Second - time.Millisecond) }
	if err := s.onUseSkill(used(skill.DragonSwirl)); err != nil {
		t.Fatal(err)
	}
	if s.svc.presence.all[1000].char.SP != sp {
		t.Fatal("used again before its cooldown")
	}
	s.svc.now = func() time.Time { return noon.Add(15 * time.Second) }
	if err := s.onUseSkill(used(skill.DragonSwirl)); err != nil {
		t.Fatal(err)
	}

	if s.svc.presence.all[1000].char.SP == sp || len(s.svc.presence.fallen) != 2 {
		t.Fatalf("SP %d, %d fell", s.svc.presence.all[1000].char.SP, len(s.svc.presence.fallen))
	}
}

// A skill is not used before its cooldown is over, without the mana, unlearnt, by the
// dead, without the sword it needs, or by a number the game has no skill for; nothing is
// spent or struck.
func TestASkillThatCannotBeUsedIsNot(t *testing.T) {
	cases := map[string]func(s *session) []byte{
		"cooling down": func(s *session) []byte {
			s.svc.cooldowns.used(s.playing.ID, skill.DragonSwirl, noon.Add(-14*time.Second))
			return used(skill.DragonSwirl)
		},
		"too little mana": func(s *session) []byte { s.svc.presence.all[1000].char.SP = 56; return used(skill.DragonSwirl) },
		"unlearnt": func(s *session) []byte {
			delete(s.svc.presence.all[1000].char.Skills, skill.DragonSwirl)
			return used(skill.DragonSwirl)
		},
		"dead": func(s *session) []byte { s.svc.presence.all[1000].char.HP = 0; return used(skill.DragonSwirl) },
		"the blade with no sword": func(s *session) []byte {
			s.svc.presence.all[1000].char.Weapon = 1000 // a dagger
			return used(skill.EnchantedBlade)
		},
		"no such skill": func(*session) []byte {
			return binary.LittleEndian.AppendUint32(binary.LittleEndian.AppendUint32(nil, 61), 0)
		},
	}
	for name, setup := range cases {
		s, client, _ := swirling(t)
		body := setup(s)
		sp := s.svc.presence.all[1000].char.SP

		if err := s.onUseSkill(body); err != nil {
			t.Fatal(err)
		}

		expectNothing(t, client)
		if s.svc.presence.all[1000].char.SP != sp || len(s.svc.presence.provoked) != 0 {
			t.Errorf("%s: used", name)
		}
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

// Enchanted Blade costs its mana, puts its glow on her for her and Kira, and makes her
// blows stronger while it runs.
func TestEnchantedBladeGlowsAndStrengthensHerBlows(t *testing.T) {
	s, client, kira := swirling(t)
	pl := s.svc.presence.all[1000]
	plain := attackerOf(pl.char, noon).Bonus

	if err := s.onUseSkill(used(skill.EnchantedBlade)); err != nil {
		t.Fatal(err)
	}

	if got := read(t, client, pointChangeSize); string(got) != string(manaSpent(32, 468)) { // 20 + 240 × 0.05
		t.Fatalf("told %x", got)
	}
	expectGlow(t, client, true)
	expectGlow(t, kira, true)
	if got := attackerOf(pl.char, noon).Bonus; plain != 0 || got != skill.Bonus(5, 1) || got == 0 {
		t.Fatalf("bonus %d, before %d", got, plain)
	}
}

// Using the blade again turns it off, for nothing, and its glow goes.
func TestEnchantedBladeUsedAgainEndsFree(t *testing.T) {
	s, client, _ := swirling(t)
	if err := s.onUseSkill(used(skill.EnchantedBlade)); err != nil {
		t.Fatal(err)
	}
	read(t, client, pointChangeSize+updateSize)
	s.svc.presence.all[1000].char.SP = 0 // turning it off needs none, as the client checks none

	if err := s.onUseSkill(used(skill.EnchantedBlade)); err != nil {
		t.Fatal(err)
	}

	expectGlow(t, client, false)
	if c := s.svc.presence.all[1000].char; c.SP != 0 || len(c.Buffs) != 0 || attackerOf(c, noon).Bonus != 0 {
		t.Fatalf("SP %d, buffs %v", c.SP, c.Buffs)
	}
}

// The world loop ends the blade three minutes after it began.
func TestEnchantedBladeEndsAfterThreeMinutes(t *testing.T) {
	s, client, _ := swirling(t)
	if err := s.onUseSkill(used(skill.EnchantedBlade)); err != nil {
		t.Fatal(err)
	}
	read(t, client, pointChangeSize+updateSize)
	for _, m := range s.svc.monsters {
		m.nextStroll = noon.Add(time.Hour) // nothing in these ticks but the blade
	}
	c := s.svc.presence.all[1000].char
	points, _ := c.Points()
	c.HP, c.SP = points.MaxHP, points.MaxSP

	s.svc.tick(noon)
	s.svc.tick(noon.Add(3*time.Minute - time.Millisecond))
	expectNothing(t, client)
	if bonus := attackerOf(c, noon.Add(3*time.Minute)).Bonus; bonus != 0 {
		t.Fatalf("still adding %d at its end", bonus)
	}
	s.svc.tick(noon.Add(3 * time.Minute))

	expectGlow(t, client, false)
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
	read(t, client, pointChangeSize+updateSize+updateSize+pointChangeSize+updateSize)
	queueBuffEnds(s.svc) // both runs' ends

	s.svc.keepBuffs(noon.Add(3 * time.Minute))

	expectNothing(t, client)
	s.svc.keepBuffs(noon.Add(4 * time.Minute))
	expectGlow(t, client, false)
}

// Death ends the blade at once.
func TestEnchantedBladeEndsOnDeath(t *testing.T) {
	s, client, _ := swirling(t)
	if err := s.onUseSkill(used(skill.EnchantedBlade)); err != nil {
		t.Fatal(err)
	}
	read(t, client, pointChangeSize+updateSize)
	pl := s.svc.presence.all[1000]

	s.svc.presence.mu.Lock()
	pl.char.HP = 0
	s.svc.presence.die(pl)
	s.svc.presence.mu.Unlock()

	expectGlow(t, client, false)
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
	read(t, client, pointChangeSize+updateSize)

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
	for _, b := range s.presence.takeNews().buffs {
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
	read(t, client, pointChangeSize+updateSize)

	s.svc.presence.dress(1000, 10, 0)

	expectGlow(t, client, true)
}

// Taking the sword out of hand ends the blade: the update the change sends shows no glow.
func TestTakingTheSwordOffEndsTheBlade(t *testing.T) {
	s, client, _ := swirling(t)
	if err := s.onUseSkill(used(skill.EnchantedBlade)); err != nil {
		t.Fatal(err)
	}
	read(t, client, pointChangeSize+updateSize)

	s.svc.presence.dress(1000, 0, 0)

	expectGlow(t, client, false)
	if c := s.svc.presence.all[1000].char; len(c.Buffs) != 0 {
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
	sp := s.svc.presence.all[1000].char.SP

	if err := again.onUseSkill(used(skill.DragonSwirl)); err != nil {
		t.Fatal(err)
	}

	if s.svc.presence.all[1000].char.SP != sp {
		t.Fatal("swirled again on a new connection")
	}
}
